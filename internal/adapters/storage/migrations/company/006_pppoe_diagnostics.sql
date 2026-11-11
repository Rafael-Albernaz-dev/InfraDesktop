CREATE TABLE pppoe_diagnostic_observations (
 id TEXT PRIMARY KEY, device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope IN ('catalog','radius','ping')),
 meta_json TEXT NOT NULL CHECK(length(CAST(meta_json AS BLOB)) BETWEEN 1 AND 16384),
 row_count INTEGER NOT NULL CHECK(row_count BETWEEN 0 AND 2048),
 payload_bytes INTEGER NOT NULL CHECK(payload_bytes BETWEEN 1 AND 524288),
 UNIQUE(id,device_id,scope)
);
CREATE TABLE pppoe_diagnostic_rows (
 diagnostic_id TEXT NOT NULL REFERENCES pppoe_diagnostic_observations(id) ON DELETE CASCADE,
 row_id TEXT NOT NULL, ordinal INTEGER NOT NULL CHECK(ordinal BETWEEN 0 AND 2047),
 row_json TEXT NOT NULL CHECK(length(CAST(row_json AS BLOB)) BETWEEN 1 AND 2048),
 PRIMARY KEY(diagnostic_id,row_id), UNIQUE(diagnostic_id,ordinal)
);
CREATE TABLE pppoe_diagnostic_current (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope IN ('catalog','radius','ping')), diagnostic_id TEXT NOT NULL,
 PRIMARY KEY(device_id,scope),
 FOREIGN KEY(diagnostic_id,device_id,scope) REFERENCES pppoe_diagnostic_observations(id,device_id,scope) ON DELETE RESTRICT
);
CREATE TABLE pppoe_diagnostic_attempts (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope IN ('catalog','radius','ping')),
 operation_id TEXT NOT NULL REFERENCES equipment_operations(id) ON DELETE RESTRICT,
 job_id TEXT NOT NULL, meta_json TEXT NOT NULL CHECK(length(CAST(meta_json AS BLOB)) BETWEEN 1 AND 4096),
 PRIMARY KEY(device_id,scope)
);
