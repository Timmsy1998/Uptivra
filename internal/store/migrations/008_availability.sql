-- Existing samples lack their original cadence: do not invent historical coverage.
ALTER TABLE checks ADD COLUMN interval_seconds integer CHECK(interval_seconds BETWEEN 60 AND 86400);
INSERT INTO schema_versions(version) VALUES(8);
