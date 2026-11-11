-- Only non-secret references; all prior tables and receipts remain intact.
CREATE TABLE radio_credential_defaults (
 class TEXT PRIMARY KEY CHECK(class IN ('ap','cpe','ptp')),
 credential_id TEXT NOT NULL REFERENCES credentials(id)
);
