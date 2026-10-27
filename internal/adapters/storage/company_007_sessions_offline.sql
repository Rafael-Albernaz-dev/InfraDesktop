ALTER TABLE devices ADD COLUMN role TEXT NOT NULL DEFAULT 'other' CHECK(role IN ('concentrator','ap','cpe','other'));
ALTER TABLE pppoe_attempts RENAME TO pppoe_attempts_006;
CREATE TABLE pppoe_attempts (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope='pppoe-active-v1'), operation_id TEXT NOT NULL,
 job_id TEXT NOT NULL, meta_json TEXT NOT NULL CHECK(length(CAST(meta_json AS BLOB)) BETWEEN 1 AND 4096),
 PRIMARY KEY(device_id,scope)
);
INSERT INTO pppoe_attempts SELECT * FROM pppoe_attempts_006;
DROP TABLE pppoe_attempts_006;
ALTER TABLE pppoe_diagnostic_attempts RENAME TO pppoe_diagnostic_attempts_006;
CREATE TABLE pppoe_diagnostic_attempts (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope IN ('catalog','radius','ping')), operation_id TEXT NOT NULL,
 job_id TEXT NOT NULL, meta_json TEXT NOT NULL CHECK(length(CAST(meta_json AS BLOB)) BETWEEN 1 AND 4096),
 PRIMARY KEY(device_id,scope)
);
INSERT INTO pppoe_diagnostic_attempts SELECT * FROM pppoe_diagnostic_attempts_006;
DROP TABLE pppoe_diagnostic_attempts_006;
CREATE TABLE equipment_read_receipt_migration (
 class TEXT PRIMARY KEY, removed_count INTEGER NOT NULL CHECK(removed_count>=0)
);
