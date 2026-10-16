CREATE TABLE maintenance_windows (
 id text PRIMARY KEY,
 monitor_id text NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
 title text NOT NULL CHECK(length(title) BETWEEN 1 AND 100),
 description text NOT NULL CHECK(length(description)<=500),
 starts_at timestamptz NOT NULL,
 ends_at timestamptz NOT NULL,
 weekly boolean NOT NULL DEFAULT false,
 CHECK(ends_at>starts_at AND ends_at<=starts_at+interval '30 days'),
 CHECK(NOT weekly OR ends_at<starts_at+interval '7 days')
);
CREATE INDEX maintenance_monitor ON maintenance_windows(monitor_id,starts_at);
-- Absolute seven-day recurrence, independent of database/session timezone and DST.
CREATE FUNCTION active_maintenance_end(target text, at_time timestamptz) RETURNS timestamptz
LANGUAGE sql STABLE AS $$
 SELECT max(ends_at + n*interval '168 hours') FROM (
  SELECT ends_at,starts_at,CASE WHEN weekly THEN floor(extract(epoch FROM (at_time-starts_at))/604800)::integer ELSE 0 END AS n
  FROM maintenance_windows WHERE monitor_id=target AND starts_at<=at_time
 ) w WHERE starts_at+n*interval '168 hours'<=at_time AND ends_at+n*interval '168 hours'>at_time
$$;
INSERT INTO schema_versions(version) VALUES(7);
