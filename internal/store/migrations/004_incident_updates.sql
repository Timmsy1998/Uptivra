CREATE TABLE incident_updates (
 id text PRIMARY KEY,
 incident_id text NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
 status text NOT NULL CHECK (status IN ('update','investigating','identified','monitoring')),
 message text NOT NULL CHECK (octet_length(message) BETWEEN 1 AND 2000 AND btrim(message) <> ''),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX incident_updates_timeline ON incident_updates(incident_id,created_at,id);
INSERT INTO schema_versions(version) VALUES(4);
