ALTER TABLE channels ADD COLUMN monitor_group text
 CHECK (monitor_group IS NULL OR octet_length(monitor_group) <= 80);
INSERT INTO schema_versions(version) VALUES(3);
