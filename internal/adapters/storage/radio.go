package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"time"
)

func radioCurrent(ctx context.Context, q queryer, device string) (*equipment.RadioReading, *equipment.RadioProfile, equipment.Code) {
	var body string
	var reading *equipment.RadioReading
	err := q.QueryRowContext(ctx, "SELECT body FROM radio_current WHERE device_id=?", device).Scan(&body)
	if err == nil {
		reading = &equipment.RadioReading{}
		if json.Unmarshal([]byte(body), reading) != nil {
			return nil, nil, equipment.Unavailable
		}
	} else if err != sql.ErrNoRows {
		return nil, nil, equipment.Unavailable
	}
	profile := &equipment.RadioProfile{DeviceID: device, Family: "unsupported", Warning: 15, Critical: 20}
	err = q.QueryRowContext(ctx, "SELECT body FROM radio_profiles WHERE device_id=?", device).Scan(&body)
	if err == nil {
		if json.Unmarshal([]byte(body), profile) != nil {
			return nil, nil, equipment.Unavailable
		}
	} else if err != sql.ErrNoRows {
		return nil, nil, equipment.Unavailable
	}
	return reading, profile, equipment.OK
}
func (s *Store) RadioCurrent(ctx context.Context, company, device string) (*equipment.RadioReading, *equipment.RadioProfile, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return nil, nil, equipment.Changed
	}
	if s.checkActive() != nil {
		return nil, nil, equipment.Unavailable
	}
	t, c := ppTarget(ctx, s.unit, s.installation, company, device, false)
	if c != equipment.OK && c != equipment.MissingCredential {
		return nil, nil, c
	}
	r, p, c := radioCurrent(ctx, s.unit, device)
	if c != equipment.OK {
		return nil, nil, c
	}
	if !t.Device.Active || c == equipment.MissingCredential || p.Revision != t.Device.Revision || p.CredentialRevision != t.Credential.Metadata.Revision || p.Connection != equipment.PPConnection(t.Device) {
		p.Confirmed = false
	}
	if r != nil {
		r.Compatible = t.Device.Active && r.Revision == t.Device.Revision && r.CredentialRevision == t.Credential.Metadata.Revision && r.Connection == equipment.PPConnection(t.Device)
	}
	return r, p, equipment.OK
}
func (s *Store) SaveRadio(ctx context.Context, company string, device equipment.Device, credential uint64, reading *equipment.RadioReading) equipment.Code {
	if reading == nil || reading.DeviceID != device.ID || reading.Revision != device.Revision || reading.CredentialRevision != credential || len(reading.Clients) > equipment.RadioMaxClients || len(reading.Interfaces) > 32 {
		return equipment.Invalid
	}
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		t, c := ppTarget(ctx, tx, s.installation, company, device.ID, false)
		if c != equipment.OK {
			return c
		}
		if !t.Device.Active || t.Device.Revision != device.Revision || equipment.PPConnection(t.Device) != equipment.PPConnection(device) || t.Credential.Metadata.Revision != credential {
			return equipment.Revision
		}
		old, profile, c := radioCurrent(ctx, tx, device.ID)
		if c != equipment.OK {
			return c
		}
		if old != nil && old.Connection == reading.Connection && old.Revision == reading.Revision && old.CredentialRevision == credential {
			before, err1 := time.Parse(time.RFC3339Nano, old.At)
			after, err2 := time.Parse(time.RFC3339Nano, reading.At)
			if err1 != nil || err2 != nil || !after.After(before) {
				return equipment.Revision
			}
			reading.HadClients = old.HadClients || len(old.Clients) > 0
		}
		reading.HadClients = reading.HadClients || len(reading.Clients) > 0
		for i := range reading.Interfaces {
			current := &reading.Interfaces[i]
			current.HadClients = current.Clients != nil && *current.Clients > 0
			if old != nil && old.Connection == reading.Connection && old.Revision == reading.Revision && old.CredentialRevision == credential {
				for _, before := range old.Interfaces {
					if before.Name == current.Name && before.BSSID == current.BSSID {
						current.HadClients = current.HadClients || before.HadClients || before.Clients != nil && *before.Clients > 0
					}
				}
			}
		}
		body, err := json.Marshal(reading)
		if err != nil || len(body) > equipment.PPObservationBytes {
			return equipment.Limit
		}
		var previous int
		err = tx.QueryRowContext(ctx, "SELECT payload_bytes FROM radio_current WHERE device_id=?", device.ID).Scan(&previous)
		if err != nil && err != sql.ErrNoRows {
			return equipment.Unavailable
		}
		total, err := ppTotalBytes(ctx, tx)
		if err != nil {
			return equipment.Unavailable
		}
		if total-int64(previous)+int64(len(body)) > equipment.PPCompanyBytes {
			return equipment.Limit
		}
		if profile.Family != reading.Family || profile.Revision != device.Revision || profile.CredentialRevision != credential || profile.Connection != reading.Connection {
			profile.Family, profile.Confirmed = reading.Family, false
		}
		profile.Revision, profile.CredentialRevision, profile.Connection = device.Revision, credential, reading.Connection
		p, _ := json.Marshal(profile)
		if _, err = tx.ExecContext(ctx, "INSERT INTO radio_current VALUES(?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET reading_id=excluded.reading_id,body=excluded.body,payload_bytes=excluded.payload_bytes", device.ID, reading.ID, string(body), len(body)); err != nil {
			return equipment.Unavailable
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO radio_profiles VALUES(?,?) ON CONFLICT(device_id) DO UPDATE SET body=excluded.body", device.ID, string(p)); err != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}
func (s *Store) ConfirmRadioFamily(ctx context.Context, company string, d equipment.Device, credential uint64, family string, warning, critical int) equipment.Code {
	if !equipment.RadioFamily(family) || family == "unsupported" || warning < 1 || critical <= warning || critical > equipment.RadioMaxClients {
		return equipment.Invalid
	}
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		t, c := ppTarget(ctx, tx, s.installation, company, d.ID, false)
		if c != equipment.OK {
			return c
		}
		r, p, c := radioCurrent(ctx, tx, d.ID)
		if c != equipment.OK {
			return c
		}
		if r == nil || r.Family != family || t.Device.Revision != d.Revision || t.Credential.Metadata.Revision != credential || r.Revision != d.Revision || r.CredentialRevision != credential || r.Connection != equipment.PPConnection(t.Device) {
			return equipment.Revision
		}
		p.Confirmed, p.Warning, p.Critical = true, warning, critical
		body, _ := json.Marshal(p)
		if _, err := tx.ExecContext(ctx, "UPDATE radio_profiles SET body=? WHERE device_id=?", string(body), d.ID); err != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}
func (s *Store) RadioLinks(ctx context.Context, company, concentrator, login string) ([]equipment.RadioLink, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return nil, equipment.Changed
	}
	if s.checkActive() != nil {
		return nil, equipment.Unavailable
	}
	rows, err := s.unit.QueryContext(ctx, "SELECT body FROM radio_links WHERE concentrator_id=? AND scope=? AND login=?", concentrator, equipment.PPScope, login)
	if err != nil {
		return nil, equipment.Unavailable
	}
	defer rows.Close()
	var out []equipment.RadioLink
	for rows.Next() {
		var body string
		var link equipment.RadioLink
		if rows.Scan(&body) != nil || json.Unmarshal([]byte(body), &link) != nil {
			return nil, equipment.Unavailable
		}
		out = append(out, link)
	}
	if rows.Err() != nil {
		return nil, equipment.Unavailable
	}
	return out, equipment.OK
}
func (s *Store) SaveRadioLink(ctx context.Context, company string, link equipment.RadioLink, operation string, source equipment.PPInput) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		body, err := json.Marshal(link)
		if err != nil || len(body) > 4096 {
			return equipment.Invalid
		}
		identity := link
		identity.At, identity.ConfirmedNow = "", false
		canonical, _ := json.Marshal(identity)
		stamp := "radio-link:" + string(canonical)
		_, known, c := ppReceipt(ctx, tx, operation, stamp)
		if c != equipment.OK || known {
			return c
		}
		var rowJSON string
		if tx.QueryRowContext(ctx, "SELECT r.row_json FROM pppoe_current c JOIN pppoe_rows r ON r.observation_id=c.observation_id WHERE c.device_id=? AND c.observation_id=? AND r.row_id=?", link.Concentrator, source.ObservationID, source.RowID).Scan(&rowJSON) != nil {
			return equipment.ObservationMissing
		}
		var row equipment.PPRow
		if json.Unmarshal([]byte(rowJSON), &row) != nil || row.Login != link.Login || row.State == "OFFLINE" {
			return equipment.ObservationIncompatible
		}
		var cpeAddress string
		if tx.QueryRowContext(ctx, "SELECT address FROM devices WHERE id=? AND role='cpe'", link.CPE).Scan(&cpeAddress) != nil || cpeAddress != row.Address {
			return equipment.ObservationIncompatible
		}
		for _, id := range []string{link.Concentrator, link.CPE, link.AP} {
			var n int
			if tx.QueryRowContext(ctx, "SELECT count(*) FROM devices WHERE id=? AND active=1", id).Scan(&n) != nil || n != 1 {
				return equipment.Revision
			}
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO radio_links VALUES(?,?,?,?) ON CONFLICT(concentrator_id,scope,login) DO UPDATE SET body=excluded.body", link.Concentrator, link.Scope, link.Login, string(body)); err != nil {
			return equipment.Unavailable
		}
		if _, err = tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?)", operation, stamp, link.AP); err != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}
func (s *Store) RadioAPReadings(ctx context.Context, company string) ([]equipment.RadioReading, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return nil, equipment.Changed
	}
	if s.checkActive() != nil {
		return nil, equipment.Unavailable
	}
	rows, err := s.unit.QueryContext(ctx, "SELECT r.body FROM radio_current r JOIN devices d ON d.id=r.device_id JOIN credentials c ON c.id=d.credential_id WHERE d.active=1 AND d.role='ap' AND json_extract(r.body,'$.revision')=d.revision AND json_extract(r.body,'$.credential_revision')=c.revision")
	if err != nil {
		return nil, equipment.Unavailable
	}
	defer rows.Close()
	var out []equipment.RadioReading
	for rows.Next() {
		var body string
		var reading equipment.RadioReading
		if rows.Scan(&body) != nil || json.Unmarshal([]byte(body), &reading) != nil {
			return nil, equipment.Unavailable
		}
		out = append(out, reading)
	}
	if rows.Err() != nil {
		return nil, equipment.Unavailable
	}
	return out, equipment.OK
}
