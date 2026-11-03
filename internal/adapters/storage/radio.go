package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
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
	profile := &equipment.RadioProfile{DeviceID: device, Family: equipment.RadioUnsupported, Warning: 15, Critical: 20}
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
func (s *Store) RadioCurrent(ctx context.Context,
	company,
	device string) (*equipment.RadioReading,
	*equipment.RadioProfile,
	equipment.Code) {
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
	if !radioStoredProfileMatches(p, t, c) {
		p.Confirmed = false
	}
	if r != nil {
		r.Compatible = t.Device.Active &&
			r.Revision == t.Device.Revision &&
			r.CredentialRevision == t.Credential.Metadata.Revision &&
			r.Connection == equipment.PPConnection(t.Device)
	}
	return r, p, equipment.OK
}
func (s *Store) SaveRadio(ctx context.Context, company string,
	device equipment.Device, credential uint64, reading *equipment.RadioReading) equipment.Code {
	if !validRadioPublication(device, credential, reading) {
		return equipment.Invalid
	}
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		target, code := ppTarget(ctx, tx, s.installation, company, device.ID, false)
		if code != equipment.OK {
			return code
		}
		if !radioPublicationMatchesTarget(target, device, credential) {
			return equipment.Revision
		}
		old, profile, code := radioCurrent(ctx, tx, device.ID)
		if code != equipment.OK {
			return code
		}
		if code = inheritRadioHistory(old, reading, credential); code != equipment.OK {
			return code
		}
		body, code := encodeRadioPublication(ctx, tx, device.ID, reading)
		if code != equipment.OK {
			return code
		}
		applyRadioProfileContext(profile, reading, device, credential)
		return writeRadioPublication(ctx, tx, device.ID, reading.ID, body, profile)
	})
}

func (s *Store) ConfirmRadioFamily(ctx context.Context,
	company string,
	d equipment.Device,
	credential uint64,
	family equipment.RadioFamily,
	warning,
	critical int) equipment.Code {
	if !validRadioConfirmationSettings(family, warning, critical) {
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
		if !radioConfirmationMatches(r, t, d, credential, family) {
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
	rows,
		err := s.unit.QueryContext(ctx,
		"SELECT body FROM radio_links WHERE concentrator_id=? AND scope=? AND login=?",
		concentrator,
		equipment.PPScope,
		login)
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
func (s *Store) SaveRadioLink(ctx context.Context, company string,
	link equipment.RadioLink, operation string, source equipment.PPInput) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		body, stamp, code := encodeRadioLinkEffect(link)
		if code != equipment.OK {
			return code
		}
		_, known, code := ppReceipt(ctx, tx, operation, stamp)
		if code != equipment.OK || known {
			return code
		}
		row, code := readRadioLinkSource(ctx, tx, link, source)
		if code != equipment.OK {
			return code
		}
		if code = enrollRadioLinkCPE(ctx, tx, link, row); code != equipment.OK {
			return code
		}
		if code = validateStoredRadioLinkEndpoints(ctx, tx, link, row); code != equipment.OK {
			return code
		}
		return writeRadioLinkEffect(ctx, tx, link, operation, stamp, body)
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
	rows,
		err := s.unit.QueryContext(ctx,
		("SELECT r.body FROM radio_current r JOIN devices d ON d.id=r.device_id JOIN cre" +
			"dentials c ON c.id=d.credential_id WHERE d.active=1 AND d.role='ap' AND json_e" +
			"xtract(r.body,'$.revision')=d.revision AND json_extract(r.body,'$.credential_r" +
			"evision')=c.revision"))
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
