CREATE TABLE pppoe_observations (
 id TEXT PRIMARY KEY, device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope='pppoe-active-v1'),
 meta_json TEXT NOT NULL, meta_bytes INTEGER NOT NULL CHECK(meta_bytes BETWEEN 1 AND 16384),
 row_count INTEGER NOT NULL CHECK(row_count BETWEEN 0 AND 2048),
 payload_bytes INTEGER NOT NULL CHECK(payload_bytes BETWEEN 1 AND 524288),
 UNIQUE(id,device_id,scope)
);
CREATE TABLE pppoe_rows (
 observation_id TEXT NOT NULL REFERENCES pppoe_observations(id) ON DELETE CASCADE,
 row_id TEXT NOT NULL, ordinal INTEGER NOT NULL CHECK(ordinal BETWEEN 0 AND 2047),
 login TEXT NOT NULL, address TEXT, caller_id TEXT, search_text TEXT NOT NULL, uptime_ns INTEGER,
 row_json TEXT NOT NULL, row_bytes INTEGER NOT NULL CHECK(row_bytes BETWEEN 1 AND 2048),
 PRIMARY KEY(observation_id,row_id), UNIQUE(observation_id,ordinal)
);
CREATE INDEX pppoe_rows_login ON pppoe_rows(observation_id,login,row_id);
CREATE INDEX pppoe_rows_address ON pppoe_rows(observation_id,address,row_id);
CREATE INDEX pppoe_rows_uptime ON pppoe_rows(observation_id,uptime_ns,row_id);
CREATE TABLE pppoe_current (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope='pppoe-active-v1'), observation_id TEXT NOT NULL,
 PRIMARY KEY(device_id,scope),
 FOREIGN KEY(observation_id,device_id,scope)
  REFERENCES pppoe_observations(id,device_id,scope) ON DELETE RESTRICT
);
CREATE TABLE pppoe_references (
 id TEXT PRIMARY KEY, device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope='pppoe-active-v1'), observation_id TEXT NOT NULL,
 created_at TEXT NOT NULL, UNIQUE(device_id,scope,observation_id),
 FOREIGN KEY(observation_id,device_id,scope)
  REFERENCES pppoe_observations(id,device_id,scope) ON DELETE RESTRICT
);
CREATE INDEX pppoe_references_device ON pppoe_references(device_id,scope,created_at,id);
CREATE TABLE pppoe_attempts (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 scope TEXT NOT NULL CHECK(scope='pppoe-active-v1'),
 operation_id TEXT NOT NULL REFERENCES equipment_operations(id) ON DELETE RESTRICT,
 job_id TEXT NOT NULL, meta_json TEXT NOT NULL,
 CHECK(length(CAST(meta_json AS BLOB)) BETWEEN 1 AND 4096),
 PRIMARY KEY(device_id,scope)
);
