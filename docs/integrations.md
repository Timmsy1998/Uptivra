# Integrations

Uptivra exposes JSON over HTTP. Your application needs neither Uptivra's database nor its login system. Use your dashboard origin for `/api/v1/*`; the Next.js proxy keeps browser sessions on one origin. Server-side clients may call the Go backend directly.

The complete [OpenAPI 3.1 contract](openapi.json) describes every route, body and security scheme. Responses are uncached. Errors are JSON `{ "error": "message" }`; callers must check HTTP status. JSON bodies are limited to 32 KiB. Routes and existing field meanings remain compatible within `/api/v1`; clients should tolerate additional response fields and status values. Breaking changes require a new major API path.

## Public status and badges

`GET /api/v1/public/status` needs no credentials. It includes only public names, groups, measured statistics and incidents. Targets and request headers stay private. Uptime is a percentage of successful **recorded checks in the last 24 hours**, not a time-weighted SLA. Null metrics mean no samples. `updated_at` is the snapshot time; each service's `last_check` indicates observation freshness.

```sh
curl --fail https://status.example.org/api/v1/public/status
```

Embed the aggregate badge without JavaScript:

```html
<a href="https://status.example.org/status">
  <img src="https://status.example.org/api/v1/public/badge.svg"
       alt="Current service status" width="176" height="24">
</a>
```

The badge follows the worst public service state. Browser/image proxies may cache remote images despite the API's no-store header; link to `/status` for current information.

## Existing applications

- [Next.js server component](../examples/nextjs/service-health.tsx): copy into an App Router application and set server-side `UPTIVRA_ORIGIN=https://status.example.org`. Render `<ServiceHealth />` where needed. It fetches without caching and shows unknown health on failure. No API key enters the browser. A dynamic parent route is needed for request-time rendering.
- [Laravel Blade partial](../examples/laravel/service-health.blade.php): copy into `resources/views/partials/service-health.blade.php`; add `'uptivra' => ['origin' => env('UPTIVRA_ORIGIN')]` to `config/services.php`, then `@include('partials.service-health')`. It uses Laravel's HTTP client, a five-second timeout and escaped output. Configure a trusted HTTP(S) origin. This partial was reviewed, but has not been run in a Laravel installation here.
- [Go client](../examples/go/main.go): run `UPTIVRA_ORIGIN=http://localhost:3000 go run ./examples/go`. It uses only the standard library, a bounded response, context and a five-second timeout. Its HTTP parsing and failure handling have automated tests.

For high-traffic applications, fetch status in your own short-lived cache or background job. Keep the last successful update time visible and change health to unknown when fetching fails or that update expires. Never turn an unavailable status API into an assumed healthy result.

## Scoped API access

Create a key in **Settings & access**. Its value appears once. `read` permits monitor configuration, history, incidents and settings reads. `write` also permits monitor CRUD and settings updates. Both can read decrypted request headers: treat either key as sensitive. Keys cannot manage other keys, notification credentials, login sessions or passwords; those operations require the administrator session.

Use a server-side secret store. Never embed an admin API key in client-side JavaScript, URLs or public widgets. Send `Authorization: Bearer $UPTIVRA_API_KEY`. Rotate by creating a replacement, updating clients and revoking the old key. Keys do not expire automatically.

Register a monitor only for an endpoint you are authorized to check:

```sh
curl --fail-with-body "$UPTIVRA_ORIGIN/api/v1/monitors" \
  -H "Authorization: Bearer $UPTIVRA_API_KEY" \
  -H 'Content-Type: application/json' \
  --data '{
    "name": "Application API", "description": "Production readiness endpoint",
    "group": "Core services", "url": "https://api.example.org/health",
    "interval_seconds": 60, "timeout_seconds": 5, "expected_status": 200,
    "contains": "", "headers": {}, "failure_threshold": 2,
    "recovery_threshold": 2, "enabled": true, "public": true,
    "authorized": true, "maintenance_until": null
  }'
```

`PUT /api/v1/monitors/{id}` replaces configuration; supply all intended fields, including headers. Omitted booleans become false and omitted headers clear them. GET then modify then PUT is a convenient pattern; concurrent writers are last-write-wins. Updates invalidate in-flight checks and reset threshold counters, preserving any open incident until measured recovery. Deleting a monitor permanently removes its checks and incidents.

Monitor collections currently return at most 1,000 entries; checks and incidents return the newest 100. There is no cursor pagination yet. Avoid treating these endpoints as a complete archival export. All timestamps use RFC 3339 UTC. Input string limits are enforced in UTF-8 bytes; OpenAPI character limits describe ASCII inputs.

Cookie-authenticated mutations require an exact `Origin` equal to `APP_ORIGIN`. Bearer keys support server-to-server access without an Origin header. Cross-origin browser access is intentionally not enabled.

## Incident webhooks

Create a **webhook** channel with an authorized public HTTP(S) URL and an independent 32–256 byte signing secret. The URL and secret are encrypted in PostgreSQL and are never returned by the channel-list API. Channels receive incident openings and recoveries for their selected monitor group, including private monitor names; channel recipients are trusted operators. Existing channels and new channels without a group filter receive all groups.

Example payload (IDs are opaque):

```json
{
  "id": "event-id",
  "type": "incident.opened",
  "incident_id": "incident-id",
  "monitor_id": "monitor-id",
  "monitor_name": "Application API",
  "at": "2026-10-13T10:00:00Z"
}
```

New events also include an `opened_at` timestamp, `interval_seconds`, `failure_threshold`, `recovery_threshold`, and a `check` snapshot (`checked_at`, `ok`, `latency_ms`, `status_code`, `reason`). When applicable, `monitor_group`, `monitor_type`, `grace_seconds`, and `expected_status` are included. These fields are additive; receivers should tolerate absent optional fields and unknown fields.

Alerts are queued only when a failure or recovery threshold is reached. Delivery is attempted during scheduler ticks, not directly on heartbeat receipt or channel creation. Monitor cadence, heartbeat grace, consecutive-check thresholds, the scheduler invocation schedule, and retry backoff all affect when an alert arrives. A change to degraded alone does not create an incident alert.

The recovery type is `incident.resolved`. Delivery is **at least once**. Each channel has a stable `X-Uptivra-Delivery` ID across retries. Verify before parsing or taking action:

1. Read the raw request body with a size limit.
2. Parse `X-Uptivra-Timestamp` as Unix seconds; reject timestamps more than five minutes in the past or future.
3. Calculate HMAC-SHA256 with your channel secret over the exact bytes `timestamp + "." + rawBody`.
4. Compare `X-Uptivra-Signature` to `sha256=` followed by the lowercase hex digest, using constant-time comparison.
5. Persist the delivery ID with a unique constraint and apply your side effect in the same transaction. Duplicate IDs should return success without repeating the effect.

Timestamps and signatures are renewed on retries; do not deduplicate by timestamp. A receiver should acknowledge with 2xx promptly after durable acceptance. Non-2xx, network failures and timeouts retry up to **eight total attempts**. Backoff is 1, 2, 4, 8, 16, 32 and 64 minutes, rounded to scheduler runs. Terminal failures remain in the database for 30 days; there is no retry/replay UI yet. Monitor the outbox as described in [operations](operations.md).

Discord channels accept a Discord webhook URL and send a branded embed: red for an incident opening and green for recovery. Details include monitor name and group, monitor type, check result, interval and thresholds, incident identifiers, localized detection/opening/recovery times, and incident duration on recovery. HTTP events include response code and latency; heartbeat events include grace period instead. Targets, request headers, response bodies and credentials are excluded. Mention parsing is disabled; monitor labels are escaped and bounded to [Discord's embed limits](https://github.com/discord/discord-api-docs/blob/main/developers/resources/message.mdx#embed-limits). Older queued events still send an embed with only their available details. They share durable retry behavior, but do not use Uptivra HMAC headers. Destinations use the same guarded public-network client as monitor checks.

Discord embeds include a **View status page** link for service status and incident updates, and the embed title is clickable. The destination is `APP_ORIGIN` followed by `/status`; set the backend’s `APP_ORIGIN` to your frontend origin (for example, `https://uptime.timmsy.dev`). This also applies to previously queued notifications.

### Group routing for Discord and signed webhooks

In **Notification channels**, choose **Monitor group** when adding a destination. Each destination can receive all groups, one exact group, or only ungrouped monitors. Change an existing destination with its **Group for …** selector; its URL and signing secret are retained. Group names are case-sensitive. Renaming or moving a monitor changes which destinations receive its future events. A destination whose group no longer exists receives no new events until a matching group exists again.

API clients may include `"group":"Discord bots"` in `POST /api/v1/channels`, or send `{"group":"Discord bots"}` to `PUT /api/v1/channels/{id}/group`. Use `null` for all groups or `""` for ungrouped monitors. Channel administration requires a signed-in administrator and a valid Origin; API keys cannot change destinations.

Routing is evaluated when an incident event is queued. Changing the filter preserves already queued deliveries and retries, and a recovery is routed using the monitor's group at recovery time. Filters apply to both public and private monitors and are not an access-control boundary.

Run `uptivra migrate` before deploying this version of the backend. Migration 003 preserves existing destinations with the all-groups default.
