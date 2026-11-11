CREATE TABLE device_equipment_snapshot (
 device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE RESTRICT,
 revision INTEGER NOT NULL, credential_revision INTEGER NOT NULL,
 connection TEXT NOT NULL, at INTEGER NOT NULL, stamp TEXT NOT NULL,
 health_json TEXT NOT NULL CHECK(length(CAST(health_json AS BLOB))<=2048),
 ports_json TEXT NOT NULL CHECK(length(CAST(ports_json AS BLOB))<=65536)
);
CREATE TABLE device_equipment_events (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 stamp TEXT NOT NULL, at INTEGER NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('port_change','reboot')),
 name TEXT NOT NULL CHECK(length(CAST(name AS BLOB))<=256),
 row_json TEXT NOT NULL CHECK(length(CAST(row_json AS BLOB))<=2048),
 PRIMARY KEY(device_id,stamp,kind,name)
);
CREATE INDEX device_equipment_events_window ON device_equipment_events(at);
