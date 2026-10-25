CREATE TABLE installation (id INTEGER PRIMARY KEY CHECK(id=1), identity TEXT NOT NULL CHECK(length(identity)=64));
INSERT INTO installation VALUES(1, lower(hex(randomblob(32))));
CREATE TABLE unit_intents (id TEXT PRIMARY KEY, name TEXT NOT NULL, ready INTEGER NOT NULL DEFAULT 0 CHECK(ready IN (0,1)));
INSERT INTO unit_intents(id,name) SELECT id,name FROM companies ORDER BY rowid;
