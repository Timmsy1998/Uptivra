# Architecture

Uptivra is a single-tenant HTTP(S) monitoring application. Go owns authentication,
monitor validation, scheduling, incidents, notification delivery and persistence.
Next.js provides the interface and forwards same-origin API requests to Go.
PostgreSQL is the only durable dependency; there is no Redis or permanent worker requirement.

A scheduler invocation claims at most 32 due monitors with `FOR UPDATE SKIP LOCKED`,
a random fencing token and a 90-second lease. Eight workers perform bounded checks.
Results and incident transitions commit together only while the token is current.
Configuration edits invalidate claims. Crashes leave reclaimable work rather than
inventing downtime. An outbox commits notification events alongside incident changes.
Delivery uses leases, stable event IDs, exponential retry and at-least-once semantics.

Initial scope: one administrator, HTTP(S), one public status page, webhook/Discord
channels, scoped API keys, 30-day history by default and one-minute minimum intervals.
TCP/DNS checks, email, multi-user RBAC, geographic probes and custom domains are
outside this release. TLS is validated by the HTTP transport; expiry alerts are deferred.

Public projections contain only selected monitor names, groups, measured health,
check aggregates and incident timestamps. Targets, headers and errors stay private.
Uptime is the percentage of successful recorded checks, not inferred wall-clock uptime.
Missing/stale checks are unknown. Maintenance skips checks and is not counted as downtime.
