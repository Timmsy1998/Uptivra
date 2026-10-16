# Monitoring a Discord bot with heartbeats

Use a heartbeat monitor when a service can make outbound HTTPS requests but cannot expose a public health endpoint. This requires a live PostgreSQL-backed Uptivra installation and a running scheduler. The browser-only demo cannot receive live signals.

1. Redeploy the updated backend and frontend. Backend startup automatically applies pending migrations, including migration 2, without resetting existing monitors. `uptivra migrate` remains available as an optional preflight.
2. In the dashboard choose **New monitor → Heartbeat (outbound signal)**. Start with interval **60 seconds**, grace **120 seconds**, failure threshold **1**, recovery threshold **1**. Save, then open the monitor.
3. Choose **Generate / replace heartbeat token** and save the displayed URL and token to the bot's environment. Replacing a token invalidates its predecessor immediately. Tokens are shown once and only their hashes are stored in PostgreSQL.
4. Send `POST` to the displayed URL with `Authorization: Bearer <token>`. No body is required. Success returns `204`. Never put the token in a URL, logs, source control, or Discord messages. An administrator API key is not a heartbeat token.

For the Ash Esports bot, configure:

```dotenv
UPTIVRA_HEARTBEAT_URL=https://uptime.timmsy.dev/api/v1/heartbeats/<monitor-id>
UPTIVRA_HEARTBEAT_TOKEN=<one-time-monitor-token>
```

Restart the bot after changing its environment. Its sender runs every 60 seconds while `client.isReady()` is true, including an immediate signal on Discord readiness. It exposes no incoming endpoint. The integration is disabled when both variables are absent. HTTP failures log a generic warning and retry on the next cycle; they do not stop the bot or reveal credentials.

A signal is fresh for interval + grace (180 seconds with these settings). The scheduler evaluates it on the monitor's interval, rounded to available cron invocations; failures then count toward the incident threshold. Creation/edit/resume grants an initial interval + grace before the first evaluation. Maintenance skips evaluations. Receipt alone does not immediately change displayed health. If the scheduler stops, stale results become **unknown**, so a scheduler outage is not reported as a measured bot failure. Keep the existing scheduler enabled; minute evaluation on Vercel requires Pro/Enterprise cron or an independent minute scheduler calling the backend `/api/cron` with `Authorization: Bearer <CRON_SECRET>`. Hobby built-in cron runs at most daily ([Vercel limits](https://vercel.com/docs/cron-jobs/usage-and-pricing)).

Heartbeat uptime is the percentage of successful scheduled freshness evaluations, not exact time connected to Discord. Latency metrics are unavailable; raw heartbeat check records use zero for unused latency/HTTP fields. Readiness does not prove every command or upstream dependency works. Anyone holding the token can spoof that monitor's health; rotate a leaked token. Rate-limit the receipt route at your hosting edge if exposed to abusive traffic; each accepted request consumes a short database transaction but does not append history. Paused/deleted monitors reject receipts.
