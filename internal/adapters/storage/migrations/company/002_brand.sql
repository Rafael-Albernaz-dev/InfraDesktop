CREATE TABLE branding(singleton INTEGER PRIMARY KEY CHECK(singleton=1), name TEXT NOT NULL, logo TEXT NOT NULL, accent TEXT NOT NULL);
INSERT INTO branding VALUES(1,'','','teal');
