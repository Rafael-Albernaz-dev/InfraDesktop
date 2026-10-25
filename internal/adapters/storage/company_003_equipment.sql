CREATE TABLE vault_key(singleton INTEGER PRIMARY KEY CHECK(singleton=1), envelope BLOB NOT NULL);
CREATE TABLE credentials(id TEXT PRIMARY KEY, alias TEXT NOT NULL, username TEXT NOT NULL, revision INTEGER NOT NULL CHECK(revision>0), envelope BLOB NOT NULL);
CREATE TABLE devices(id TEXT PRIMARY KEY, name TEXT NOT NULL, address TEXT NOT NULL, port INTEGER NOT NULL CHECK(port BETWEEN 1 AND 65535), transport TEXT NOT NULL CHECK(transport='ssh'), credential_id TEXT REFERENCES credentials(id), revision INTEGER NOT NULL CHECK(revision>0), active INTEGER NOT NULL CHECK(active IN (0,1)), fingerprint TEXT NOT NULL, trust_origin TEXT NOT NULL, observation TEXT);
CREATE TABLE equipment_operations(id TEXT PRIMARY KEY, stamp TEXT NOT NULL, entity TEXT NOT NULL);
