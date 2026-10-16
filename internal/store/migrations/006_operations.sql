-- Historical rows retain unknown creation/attempt times rather than invented dates.
ALTER TABLE deliveries ADD COLUMN created_at timestamptz;
ALTER TABLE deliveries ALTER COLUMN created_at SET DEFAULT now();
ALTER TABLE deliveries ADD COLUMN last_attempt_at timestamptz;
ALTER TABLE deliveries ADD COLUMN outcome text NOT NULL DEFAULT '';
CREATE INDEX deliveries_recent ON deliveries(created_at DESC NULLS LAST,id);
CREATE INDEX deliveries_failed ON deliveries(failed_at) WHERE failed_at IS NOT NULL;
CREATE TABLE scheduler_runs (
 id text PRIMARY KEY, started_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz, success boolean, claimed integer NOT NULL DEFAULT 0,
 recorded bigint NOT NULL DEFAULT 0, delivered bigint NOT NULL DEFAULT 0
);
CREATE INDEX scheduler_runs_recent ON scheduler_runs(started_at DESC);
INSERT INTO schema_versions(version) VALUES(6);
