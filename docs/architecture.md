# Architecture

Uptivra separates network access, domain transitions, persistence and hosting. The Go API and scheduler share the same PostgreSQL store. React renders wire data and submits configuration; it does not decide whether an incident should open.

| Location | Responsibility |
| --- | --- |
| `internal/domain` | Configuration validation, observed health and incident transition rules |
| `internal/netguard` | URL policy, address validation, DNS-pinned transport and redirect policy |
| `internal/worker` | HTTP execution, bounded scheduling and notification delivery |
| `internal/store` | Transactions, auth, encrypted configuration, claims, history and retention |
| `internal/server` | Versioned routes, authorization, CSRF and public projections |
| `cmd/uptivra` | CLI: serve, migrate, bootstrap, tick and schedule |
| `api/index.go` | Vercel Go Function adapter; lazy, retryable database initialization |
| `web` | Next.js workspace, public page and fixed-origin API proxy |
| `examples` | Optional public-status consumers |

## Execution and durability

Each tick claims the oldest 32 due, enabled monitors with `FOR UPDATE SKIP LOCKED`. Claims contain random fencing tokens and expire after 90 seconds. Eight workers execute checks within a 42-second check budget. Results are committed only if the original lease remains current and unexpired. Editing a monitor clears its lease, so an old result cannot overwrite new configuration.

A result transaction inserts history, updates counters, opens/resolves an incident and writes notification events atomically. Duplicate completion is ignored after the lease is cleared; `(monitor_id, checked_at)` also has a uniqueness constraint. An incident has at most one open row per monitor. A killed process leaves reclaimable work; no in-memory coordination is required.

The next due time advances along the monitor's existing schedule to the first slot after transaction start. Missed slots are skipped rather than replayed in a burst. Minute cron rounds actual execution to available ticks; configured intervals are scheduling targets, not real-time guarantees. A monitor edited or resumed becomes due immediately. Maintenance prevents claims until its deadline; it does not invent successful checks or resolve existing incidents.

After checks, the tick claims up to 16 outbox entries and delivers with eight workers and a five-second request timeout. The whole tick has a 55-second deadline under the configured 60-second Go Function limit. There are no goroutines continuing after the handler returns. Delivery is at least once; a crash after remote acceptance but before the database commit can duplicate an event. Receivers must deduplicate persistently.

## Health and statistics

A successful check requires the configured response status and, if supplied, a matching substring in a response body no larger than one MiB. Response times include request and bounded body reading. Error messages use stable, redacted reason codes.

Below the consecutive failure threshold a failing service is degraded. At the threshold it is down and opens an incident. A down service stays down through partial recovery until the recovery threshold is met. Successful checks without an open incident are healthy immediately. Thresholds count observations; gaps do not supply failures or successes. Editing resets counters while preserving an open incident.

Paused and maintenance states override observed health. With no result, or a result older than `2 × interval + 30 seconds`, health is unknown. Open incidents remain visible even when checks become stale. The public aggregate selects the worst published state: down, degraded, unknown, maintenance, paused, healthy; an empty page is unknown.

Statistics use recorded samples in the previous 24 hours. Uptime is successful samples divided by all samples; mean/p95 latency includes failed samples. Missing observations, pauses and maintenance have no fabricated measurements. This is neither time-weighted uptime nor a contractual SLA calculation.

## Capacity and retention

The database pool is capped at four connections per process/function instance. Horizontal instances multiply that number: size a managed pooler and PostgreSQL limits accordingly. Concurrent ticks safely claim disjoint work but increase outbound traffic and database load.

One tick processes at most 32 checks and 16 notifications. A sustained backlog, slow endpoints or provider delays reduce coverage. Start with fewer than 32 minute-interval monitors per scheduled invocation and measure actual coverage; no throughput or SLA benchmark is claimed. The UI/API lists at most 1,000 monitors and the newest 100 checks/incidents. There is no cursor pagination or bulk archival export yet.

Retention is 7–365 days, default 30. Cleanup runs after scheduler work and deletes checks in batches of 5,000, expired sessions/throttle buckets, old resolved incidents and deliveries completed/failed more than 30 days ago. Open incidents and pending deliveries remain. Scheduler outages therefore delay cleanup. A large retention backlog may take several runs to drain; monitor database growth.

## Schema and hosting

The initial schema is an embedded SQL migration guarded by a PostgreSQL advisory lock and transaction. `schema_versions` records applied versions. Migration is an explicit operator/Compose step; HTTP requests never migrate. Future changes must add numbered, backward-compatible migrations and an upgrade path rather than silently changing the initial schema.

Vercel runs the Go Function and Next.js frontend as separate projects. The root `vercel.json` routes API requests to one adapter and calls `/api/cron` every minute. Frontend cookies remain on the frontend origin; only selected headers pass through a fixed `API_ORIGIN` proxy. Long scheduler requests must call the backend directly because the frontend proxy has a 15-second timeout.

Native/container deployments use `serve` and `schedule`, or an external minute scheduler invoking `tick`. Run one chosen scheduling mechanism per installation. The same durable claims handle accidental overlap. PostgreSQL is the only persistent dependency. Local filesystem state is unnecessary for application correctness.
