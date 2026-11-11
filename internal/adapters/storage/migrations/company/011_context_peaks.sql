CREATE TABLE device_context_samples (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 context TEXT NOT NULL CHECK(length(context)=64),
 observation_id TEXT NOT NULL, at INTEGER NOT NULL,
 service TEXT NOT NULL CHECK(length(CAST(service AS BLOB)) BETWEEN 1 AND 256),
 vlan INTEGER NOT NULL CHECK(vlan BETWEEN 0 AND 4094),
 sessions INTEGER NOT NULL CHECK(sessions>=0 AND sessions<=2048),
 PRIMARY KEY(device_id,observation_id,service,vlan)
);
CREATE INDEX device_context_samples_window ON device_context_samples(device_id,context,at);
