CREATE TABLE device_monitoring_points (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 bucket INTEGER NOT NULL, resolution INTEGER NOT NULL CHECK(resolution IN (60,3600)),
 first_at INTEGER NOT NULL, last_at INTEGER NOT NULL, session_at INTEGER,
 min_online INTEGER, max_online INTEGER, last_online INTEGER,
 health_json TEXT NOT NULL DEFAULT '' CHECK(length(CAST(health_json AS BLOB))<=2048), health_at INTEGER,
 gap INTEGER NOT NULL CHECK(gap IN (0,1)), drop_seen INTEGER NOT NULL CHECK(drop_seen IN (0,1)), samples INTEGER NOT NULL,
 PRIMARY KEY(device_id,resolution,bucket)
);
CREATE INDEX device_monitoring_retention ON device_monitoring_points(bucket);
CREATE TABLE device_session_transitions (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 observation_id TEXT NOT NULL, login TEXT NOT NULL CHECK(length(CAST(login AS BLOB)) BETWEEN 1 AND 256),
 at INTEGER NOT NULL, previous_state TEXT NOT NULL CHECK(previous_state IN ('ONLINE','OFFLINE')),
 next_state TEXT NOT NULL CHECK(next_state IN ('ONLINE','OFFLINE')),
 PRIMARY KEY(device_id,observation_id,login)
);
CREATE INDEX device_session_transitions_window ON device_session_transitions(at);
CREATE TABLE device_monitoring_checkpoint (
 device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE RESTRICT,
 observation_id TEXT NOT NULL, continuous INTEGER NOT NULL CHECK(continuous IN (0,1))
);
CREATE TABLE device_monitoring_limits (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), pruned_at INTEGER NOT NULL
);

CREATE TABLE device_monitoring_generation (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1), generation INTEGER NOT NULL CHECK(generation>=0 AND typeof(generation)='integer')
);
INSERT INTO device_monitoring_generation VALUES(1,0);
CREATE TABLE device_monitoring_packages (
 device_id TEXT NOT NULL REFERENCES devices(id) ON DELETE RESTRICT,
 name TEXT NOT NULL CHECK(length(CAST(name AS BLOB)) BETWEEN 1 AND 256),
 version TEXT NOT NULL CHECK(length(CAST(version AS BLOB))<=256), disabled INTEGER,
 PRIMARY KEY(device_id,name)
);
CREATE TABLE device_monitoring_package_status (
 device_id TEXT PRIMARY KEY REFERENCES devices(id) ON DELETE RESTRICT,
 at INTEGER NOT NULL, state TEXT NOT NULL CHECK(state IN ('complete','unavailable','limited'))
);
