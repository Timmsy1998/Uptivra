CREATE TABLE heartbeats (
 monitor_id text PRIMARY KEY REFERENCES monitors ON DELETE CASCADE,
 token_hash text UNIQUE, last_received timestamptz
);
INSERT INTO schema_versions(version) VALUES(2);
