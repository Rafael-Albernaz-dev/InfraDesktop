-- Preserve the historical devices CHECK; the new PTP presentation role is
-- additive metadata over the existing 'other' role, never a readable driver.
CREATE TABLE device_radio_roles (
  device_id TEXT PRIMARY KEY REFERENCES devices(id),
  role TEXT NOT NULL CHECK(role='ptp')
);
CREATE TABLE radio_current (
  device_id TEXT PRIMARY KEY REFERENCES devices(id),
  reading_id TEXT NOT NULL,
  body TEXT NOT NULL,
  payload_bytes INTEGER NOT NULL CHECK(payload_bytes>=0)
);
CREATE TABLE radio_profiles (
  device_id TEXT PRIMARY KEY REFERENCES devices(id),
  body TEXT NOT NULL
);
CREATE TABLE radio_links (
  concentrator_id TEXT NOT NULL REFERENCES devices(id),
  scope TEXT NOT NULL,
  login TEXT NOT NULL,
  body TEXT NOT NULL,
  PRIMARY KEY(concentrator_id,scope,login)
);
