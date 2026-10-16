# Operations

## Acceptance

Before relying on a deployment:

1. Check direct backend `GET /healthz` and frontend `GET /api/v1/public/status`. Readiness checks PostgreSQL connectivity; it does not prove checks or notifications are running.
2. Sign in, create a public monitor for an endpoint you control, and observe real history after scheduled execution. Confirm measured timestamps advance across multiple minutes. Confirm a private monitor's name, target and headers are absent from the anonymous status response.
3. Configure a webhook receiver or Discord channel. Make your controlled public endpoint fail long enough to cross its threshold, verify one open incident and a delivered alert, then restore it and verify recovery. Localhost test targets are intentionally blocked. Use a public test environment you are authorized to probe.
4. Make the receiver temporarily unavailable. Confirm an outbox retry, then restore it and verify delivery/deduplication. Webhooks are at least once; receivers must validate signatures and persist delivery IDs. Unit tests exercise signing and Discord payloads, but a live Discord account was not used during development.
5. Stop scheduled execution. After `2 × interval + 30 seconds`, the monitor must become unknown; recorded uptime must not grow from the gap. Restore the scheduler and verify checks resume without duplicate incidents or an immediate burst of missed checks.
6. Verify HTTPS, origin checks, logout, a read key's rejected writes, persistence across restart and a backup restore in a separate environment.

## Coverage and capacity

Monitor the monitor from an independent system. Alert on scheduler silence, stale observations, oldest overdue work, outbox age, terminal delivery failures and database/storage pressure. A healthy API alone is not evidence of monitoring coverage.

On Vercel, inspect cron/function logs and the actual invocation schedule. For native services use `journalctl -u uptivra-scheduler`; for Compose use `docker compose logs scheduler`. Logs intentionally omit targets, headers, payloads and secret-bearing errors. Tick counters describe completed execution attempts; a fenced stale result may be ignored, so counters are not an audit ledger of committed rows.

The **Delivery & scheduler** panel shows the last durable run, overdue monitors and notification backlog. An unfinished run may indicate interruption; no recorded run means no scheduler invocation has been recorded since migration. Use an independent monitor to detect scheduler silence.

Useful read-only database queries (run with a trusted, appropriately restricted operator account):

```sql
-- Enabled monitors currently eligible for scheduling, including reclaimable leases.
SELECT count(*) AS due,
       max(now() - next_check) AS oldest_due
FROM monitors
WHERE enabled AND next_check <= now()
  AND (maintenance_until IS NULL OR maintenance_until <= now())
  AND active_maintenance_end(id, now()) IS NULL
  AND (lease_until IS NULL OR lease_until <= now());

-- Coverage gaps outside maintenance; never treat these as successful checks.
SELECT count(*) AS stale
FROM monitors
WHERE enabled
  AND (maintenance_until IS NULL OR maintenance_until <= now())
  AND active_maintenance_end(id, now()) IS NULL
  AND (last_check IS NULL OR last_check < now() - (2 * interval_seconds + 30) * interval '1 second');

SELECT count(*) FILTER (WHERE delivered_at IS NULL AND failed_at IS NULL) AS pending,
       count(*) FILTER (WHERE failed_at IS NOT NULL) AS failed,
       min(next_attempt) FILTER (WHERE delivered_at IS NULL AND failed_at IS NULL) AS oldest_pending
FROM deliveries;
```

A tick claims at most 32 checks and 16 deliveries with eight workers. Checks get a 42-second budget and the entire tick 55 seconds. A 90-second expired lease becomes reclaimable. Large populations, slow targets, database contention or provider scheduling delays reduce coverage. Start below capacity, measure and adapt; there is no established performance SLA. Monitor sustained overdue work rather than relying on monitor count alone.

Notifications have eight total attempts, with nominal delays of 1, 2, 4, 8, 16, 32 and 64 minutes after successive failures. Actual retries are quantized by scheduler ticks. Exhausted entries set `failed_at`. Fix delivery configuration, use **Channels → Test**, and inspect the result in **Delivery & scheduler**. Failed deliveries can be retried there with a fresh eight-attempt budget; retries run on a subsequent scheduler tick. Retried notifications can be delivered more than once. Deleting a channel deletes its queued deliveries; deleting a monitor removes its checks and incidents. The UI confirms destructive actions.

## Backups and restore

Back up PostgreSQL and the matching encryption key. Losing the key makes encrypted monitor headers and channel configuration unreadable. Keep the key in a separate secret store, not beside an unencrypted database dump. Session/API token hashes and password hashes are also sensitive.

For native/managed PostgreSQL, use your provider's point-in-time recovery plus periodic tested logical backups. A local example with a separately provisioned backup destination:

```sh
umask 077
pg_dump --format=custom --file=uptivra.dump "$DATABASE_URL"
# Restore only to an empty, separate database for a drill.
pg_restore --dbname="$RESTORE_DATABASE_URL" --no-owner --no-acl uptivra.dump
```

For Compose, stream `docker compose exec -T db pg_dump -U uptivra -d uptivra -Fc` to protected storage. Do not commit dumps. During a production restore, stop API/scheduler writers, restore the complete schema/data, configure the matching key, run explicit migrations, then resume and test health. Prevent the restored test environment from delivering production alerts or exposing production data; use outbound firewall rules until isolated configuration is ready. Existing leases expire naturally. Rotate/revoke copied credentials before enabling a restored installation publicly.

Retention deletes checks in bounded batches and old resolved incidents according to the configured 7–365 days. Completed/failed deliveries expire after 30 days. Open incidents and pending notifications remain. Schedule outages also delay cleanup; monitor database growth and verify backups independently of retention.

## Changes and recovery

Pause/maintenance controls suppress new claims but preserve incidents and history. Edits invalidate an outstanding claim and reset consecutive counters; an existing incident stays open until recovery thresholds are reached. Configuration updates replace complete objects rather than patching fields. There is no concurrent-edit version token; the last accepted edit wins.

Bootstrap and verified browser setup create only the first owner and refuse replacement. The owner manages additional administrators and ownership transfers in Settings. For a lost account password, use the trusted terminal [recovery command](deployment.md#native-development) with that account’s email; it revokes the affected account’s browser sessions. There is no password-reset email flow. Encryption-key rotation still needs explicit future tooling.

Before an upgrade: back up, inspect migration compatibility, run tests, apply migrations explicitly and verify actual checks/alerts afterward. This version includes migrations through schema version 8. Do not edit an already-applied migration to change production schema. Roll back application code only when the resulting schema is compatible; otherwise restore a tested backup with an understood data-loss window.
