CREATE TABLE IF NOT EXISTS schema_versions (version integer PRIMARY KEY);
CREATE TABLE IF NOT EXISTS accounts (
 id integer PRIMARY KEY CHECK(id=1), email text NOT NULL, password_hash text NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
 token_hash text PRIMARY KEY, expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS rate_limits (
 bucket text PRIMARY KEY, attempts integer NOT NULL, resets_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS api_keys (
 id text PRIMARY KEY, name text NOT NULL, token_hash text UNIQUE NOT NULL,
 scope text NOT NULL CHECK(scope IN ('read','write')), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS monitors (
 id text PRIMARY KEY, config jsonb NOT NULL, secrets text NOT NULL,
 enabled boolean NOT NULL DEFAULT true, public boolean NOT NULL DEFAULT false,
 interval_seconds integer NOT NULL CHECK(interval_seconds BETWEEN 60 AND 86400),
 maintenance_until timestamptz, next_check timestamptz NOT NULL DEFAULT now(),
 lease_token text, lease_until timestamptz, status text NOT NULL DEFAULT 'unknown',
 failures integer NOT NULL DEFAULT 0, successes integer NOT NULL DEFAULT 0,
 last_check timestamptz, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS monitors_due ON monitors(next_check) WHERE enabled;
CREATE TABLE IF NOT EXISTS checks (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, monitor_id text NOT NULL REFERENCES monitors ON DELETE CASCADE,
 checked_at timestamptz NOT NULL DEFAULT now(), ok boolean NOT NULL, latency_ms double precision NOT NULL,
 status_code integer NOT NULL, reason text NOT NULL, UNIQUE(monitor_id,checked_at)
);
CREATE INDEX IF NOT EXISTS checks_history ON checks(monitor_id,checked_at DESC);
CREATE INDEX IF NOT EXISTS checks_retention ON checks(checked_at);
CREATE TABLE IF NOT EXISTS incidents (
 id text PRIMARY KEY, monitor_id text NOT NULL REFERENCES monitors ON DELETE CASCADE,
 opened_at timestamptz NOT NULL DEFAULT now(), resolved_at timestamptz
);
CREATE UNIQUE INDEX IF NOT EXISTS one_open_incident ON incidents(monitor_id) WHERE resolved_at IS NULL;
CREATE TABLE IF NOT EXISTS channels (
 id text PRIMARY KEY, name text NOT NULL, kind text NOT NULL CHECK(kind IN ('webhook','discord')),
 secret_config text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS deliveries (
 id text PRIMARY KEY, channel_id text NOT NULL REFERENCES channels ON DELETE CASCADE,
 payload jsonb NOT NULL, attempts integer NOT NULL DEFAULT 0,
 next_attempt timestamptz NOT NULL DEFAULT now(), lease_token text, lease_until timestamptz,
 delivered_at timestamptz, failed_at timestamptz
);
CREATE INDEX IF NOT EXISTS deliveries_due ON deliveries(next_attempt) WHERE delivered_at IS NULL AND failed_at IS NULL;
CREATE TABLE IF NOT EXISTS settings (
 id integer PRIMARY KEY CHECK(id=1), title text NOT NULL DEFAULT 'Service status',
 description text NOT NULL DEFAULT 'Live availability and incident history.',
 retention_days integer NOT NULL DEFAULT 30 CHECK(retention_days BETWEEN 7 AND 365)
);
INSERT INTO settings(id) VALUES(1) ON CONFLICT DO NOTHING;
INSERT INTO schema_versions(version) VALUES(1) ON CONFLICT DO NOTHING;
