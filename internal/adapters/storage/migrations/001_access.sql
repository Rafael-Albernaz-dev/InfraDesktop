CREATE TABLE account (
 id INTEGER PRIMARY KEY CHECK(id=1), name TEXT NOT NULL,
 algorithm TEXT NOT NULL, memory INTEGER NOT NULL, iterations INTEGER NOT NULL, parallelism INTEGER NOT NULL,
 salt BLOB NOT NULL CHECK(length(salt)=16), hash BLOB NOT NULL CHECK(length(hash)=32),
 recovery BLOB NOT NULL CHECK(length(recovery)=32), epoch INTEGER NOT NULL CHECK(epoch>0),
 theme TEXT NOT NULL CHECK(theme IN ('light','dark')), profile TEXT NOT NULL DEFAULT ''
);
CREATE TABLE companies (id TEXT PRIMARY KEY, account_id INTEGER NOT NULL DEFAULT 1 REFERENCES account(id), name TEXT NOT NULL);
CREATE TABLE grants (digest BLOB PRIMARY KEY CHECK(length(digest)=32), account_id INTEGER NOT NULL DEFAULT 1 REFERENCES account(id), epoch INTEGER NOT NULL, expires INTEGER NOT NULL);
CREATE TABLE attempts (id INTEGER PRIMARY KEY CHECK(id=1), failures INTEGER NOT NULL, next_attempt INTEGER NOT NULL);
INSERT INTO attempts VALUES (1,0,0);
