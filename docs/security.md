# Security model

Uptivra executes administrator-configured network requests. Treat it as an outbound network client with privileged configuration, even when all users are trusted. It is not a substitute for an egress firewall, a secret manager or an independent coverage monitor.

## Destinations and isolation

Only HTTP(S) GET checks on ports 80/443 are supported. Configuration rejects credentials in URLs, malformed/noncanonical IP forms, single-label/internal names and prohibited IPv4/IPv6 destinations. All DNS answers must pass the public-address policy; a mixed public/private answer is rejected. The transport pins a validated address while retaining the original hostname for TLS validation. Environment proxies are disabled. Every redirect is validated, limited to five and constrained to the original hostname/port without an HTTPS downgrade. Response bodies, headers, timeouts and concurrency are bounded.

Webhook/Discord delivery uses the same guarded transport. Custom request headers reject Host, Cookie, hop-by-hop headers and newline injection. Operators explicitly acknowledge authorization when configuring monitors. This is an authorization declaration, not proof of domain ownership. The application cannot stop an authorized operator from monitoring or sending traffic to a third-party public service. Keep outbound quotas/firewall rules and administrative access appropriate to your environment. Private-network monitoring is intentionally unsupported.

Reserved ranges evolve; maintain the address policy and its bypass tests. Application validation does not protect against a compromised host, database, administrator or DNS/network stack. Deploy without access to sensitive internal services wherever possible.

## Credentials and authorization

There is one administrator role. Passwords use bcrypt with cost 12; the 14–72-byte limit avoids silent bcrypt truncation. Random session tokens and scoped API keys are stored only as SHA-256 hashes. Sessions expire after 12 hours. Cookies are HttpOnly, SameSite=Strict, scoped to `/api`, and Secure for HTTPS installations. Cookie mutations require the configured exact Origin. Changing the password requires the current password and revokes every session. Account-row locking coordinates session issuance with password changes, including logins already in progress. Logout revokes the current token.

API keys have `read` or `write` scope; write includes read. Key/channel/account management requires an administrator cookie, not an API key. Keys have no expiry mechanism yet: revoke and replace them regularly. Read keys can see operational configuration, including target URLs, though decrypted secret values are omitted. Never put keys in client components, browser storage, public widgets, URLs or logs.

Durable database throttles limit login attempts globally (60) and per submitted email (10) in 15-minute windows; password changes have a separate account bucket. These are not a general public API/WAF rate limiter. An attacker can consume login capacity and temporarily deny legitimate logins. Use host-level abuse controls; the application does not trust forwarded IP headers for identity. MFA, multiple users, audit trails, password reset and object-level access roles are not implemented.

## Stored secrets and public data

Monitor headers and complete notification destination/signing configuration use AES-256-GCM with random nonces and authenticated ciphertext. The encryption key is external to PostgreSQL. There is no online key-rotation/multi-key tool: changing it without re-encryption breaks access to existing ciphertext. Preserve the key with protected backup and disaster-recovery procedures.

Monitor targets, names, descriptions, check history and outbox payloads are stored in plaintext. Do not embed credentials or tokens in monitor URLs; use encrypted headers instead. Notification channels are trusted operator destinations: each channel receives private and public incident names for its selected group (all groups by default). Group routing is not an authorization boundary; administrators control monitor groups and destinations. Do not send operator alerts into a public channel unless that disclosure is intended.

Anonymous status is built from explicit public response types. It omits targets, request headers, private monitors and channel/key configuration. Published names, groups, descriptions, incident times and measurements are intentionally public. Admin screens must not be published through cached responses. React escapes text; the SVG badge escapes XML. API responses disable caching, reject MIME sniffing and use restrictive security headers. Next.js HTML has its own headers and inline-script requirements; it does not use the API's `default-src 'none'` policy. No arbitrary administrator HTML is rendered.

## Webhooks

Payloads include stable event IDs; deliveries have stable per-channel IDs. HMAC-SHA256 covers the timestamp and exact transmitted body. See [integration verification](integrations.md). Receivers must enforce timestamp freshness, compare in constant time and persist deduplication. Retries may duplicate remotely accepted deliveries. Secrets are shown only at creation/configuration and should be stored securely by the receiver. Discord requests disable mentions.

## Deployment and maintenance

Use HTTPS and the exact frontend `APP_ORIGIN`. Restrict direct Go/database ports, apply deployment protection deliberately, and keep the Vercel bypass/cron secrets server-side. The frontend forwards only a selected header set to the configured backend origin and bounds input size; it cannot choose arbitrary destinations. Avoid untrusted frontend previews with production backend access.

The Go pool and scheduler bound work per invocation, not globally across unlimited instances. Infrastructure concurrency and database limits are still necessary. Endpoint ownership verification, general rate limiting and quotas would strengthen a hosted multi-tenant service; this release is a self-hosted, single-administrator tool.

CI checks Go formatting, vet/race tests with PostgreSQL, reachable Go vulnerabilities, npm advisories, frontend formatting/types/build, Compose configuration and Chromium workflows. Actions are pinned to reviewed commit hashes; dependencies use Go checksums/npm lockfile. Audits are a point-in-time check, not a guarantee. At development verification, govulncheck found no vulnerabilities in imported packages or reachable code; the required `x/crypto` module has an advisory for its unused, unmaintained OpenPGP package (GO-2026-5932, no fixed version). Uptivra uses bcrypt, not OpenPGP. Keep reviewing advisories and updating maintained dependencies.

Tests cover address bypasses, rebinding-style resolution, redirects, timeouts, encryption tampering, auth/CSRF/scopes, concurrent password changes and logins, secret redaction, stale claims and duplicate completion. Remote provider controls and infrastructure isolation need deployment acceptance checks. Report suspected vulnerabilities through [SECURITY.md](../SECURITY.md).

Heartbeat tokens authorize only receipt for one enabled monitor. Session-only, origin-validated rotation returns plaintext once and stores a hash. Receipts serialize with result recording, edits and deletion; rotation is rechecked when updating the heartbeat row. A stolen token can falsify that monitor’s health until rotated. Receipt requests have a ten-second database deadline; edge rate limiting remains an operator responsibility. See [heartbeat setup](heartbeats.md).

## Initial setup and administrator ownership

Browser setup is disabled unless `SETUP_CODE` contains exactly four ASCII digits. It requires the configured Origin, a durable global five-attempt/15-minute limit and an empty account table. Creation is serialized with CLI bootstrap. A four-digit code has limited entropy; deploy on a trusted network, complete setup promptly and remove the code afterward. The global throttle can delay legitimate setup after malicious attempts.

Sessions reference individual accounts and cascade on account deletion. Password changes revoke only the affected account’s sessions. Administrator creation, deletion and ownership transfer require a live owner session, rechecked under a transaction lock. The current owner cannot be deleted or delete themselves; ownership transfer is atomic and retains exactly one owner. Ordinary administrators retain full monitoring/configuration access, including outbound channel configuration; grant this role only to trusted operators.

Delivery diagnostics expose channel names, event types, attempt counts and bounded outcome categories only. Test notifications and terminal-failure retries require administrator sessions and origin validation, with durable global rate limits. Tests use the same encrypted destinations, network restrictions, signatures and fenced delivery queue as incident alerts. A retry resets the eight-attempt budget and invalidates the previous lease; delivery remains at least once. Old deliveries have unknown creation timestamps rather than invented dates.

Scheduler runs are recorded durably before claiming work. A timeout or interrupted invocation may leave an unfinished run. Health counts exclude paused monitors and active maintenance and consider a monitor overdue 90 seconds after its due time. Operational history follows retention limits; an absence of run records is not evidence of successful monitoring.

Maintenance creation and cancellation require administrator sessions and origin validation. The database caps schedules at 1,000 and enforces window durations. Creation locks the monitor and invalidates its current lease; recording rechecks active windows inside the locked transaction. Public projections contain only schedule text, public monitor names/IDs and occurrence times. Weekly recurrence uses absolute 168-hour intervals. Maintenance does not silently resolve an existing incident or count unmeasured time as successful monitoring.
