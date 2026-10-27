package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
)

// All callbacks are short and serialized with active-unit changes. Network IO is
// never performed inside this transaction or while retaining the active unit.
func (s *Store) ReadEquipment(ctx context.Context, id string) (equipment.State, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != id {
		return equipment.State{}, equipment.Changed
	}
	if s.checkActive() != nil {
		return equipment.State{}, equipment.Unavailable
	}
	st, e := s.loadEquipment(ctx, s.unit, id)
	if e != nil {
		return equipment.State{}, equipment.Unavailable
	}
	return st, equipment.OK
}
func (s *Store) loadEquipment(ctx context.Context, q queryer, id string) (equipment.State, error) {
	st := equipment.State{Installation: s.installation, Company: id, Credentials: []equipment.SealedCredential{}, Devices: []equipment.Device{}, Operations: []equipment.Operation{}}
	e := q.QueryRowContext(ctx, "SELECT envelope FROM vault_key WHERE singleton=1").Scan(&st.WrappedKey)
	if e != nil && e != sql.ErrNoRows {
		return st, e
	}
	rows, e := q.QueryContext(ctx, "SELECT id,alias,username,revision,envelope FROM credentials ORDER BY rowid")
	if e != nil {
		return st, e
	}
	for rows.Next() {
		var c equipment.SealedCredential
		if e = rows.Scan(&c.Metadata.ID, &c.Metadata.Alias, &c.Metadata.User, &c.Metadata.Revision, &c.Envelope); e != nil {
			rows.Close()
			return st, e
		}
		st.Credentials = append(st.Credentials, c)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return st, e
	}
	rows, e = q.QueryContext(ctx, "SELECT id,name,address,port,transport,COALESCE(credential_id,''),revision,active,fingerprint,trust_origin,observation,trust_acceptance,last_attempt,role FROM devices ORDER BY rowid")
	if e != nil {
		return st, e
	}
	for rows.Next() {
		var d equipment.Device
		var observation, acceptance, attempt sql.NullString
		if e = rows.Scan(&d.ID, &d.Name, &d.Address, &d.Port, &d.Transport, &d.CredentialID, &d.Revision, &d.Active, &d.Fingerprint, &d.TrustOrigin, &observation, &acceptance, &attempt, &d.Role); e != nil {
			rows.Close()
			return st, e
		}
		if observation.Valid {
			d.Result = &equipment.Observation{}
			if e = json.Unmarshal([]byte(observation.String), d.Result); e != nil {
				rows.Close()
				return st, e
			}
		}
		if acceptance.Valid {
			d.TrustAcceptance = &equipment.TrustAcceptance{}
			if e = json.Unmarshal([]byte(acceptance.String), d.TrustAcceptance); e != nil {
				rows.Close()
				return st, e
			}
		}
		if attempt.Valid {
			d.LastAttempt = &equipment.Attempt{}
			if e = json.Unmarshal([]byte(attempt.String), d.LastAttempt); e != nil {
				rows.Close()
				return st, e
			}
		}
		st.Devices = append(st.Devices, d)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return st, e
	}
	for i := range st.Devices {
		d := &st.Devices[i]
		if d.Role != "concentrator" {
			continue
		}
		var body string
		err := q.QueryRowContext(ctx, "SELECT o.meta_json FROM pppoe_current c JOIN pppoe_observations o ON o.id=c.observation_id WHERE c.device_id=?", d.ID).Scan(&body)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return st, err
		}
		var o equipment.PPObservation
		if err = json.Unmarshal([]byte(body), &o); err != nil {
			return st, err
		}
		online := o.Online
		if online == 0 && o.Offline == 0 {
			online = o.Count
		}
		d.PPPoE = &equipment.PPSummary{Online: online, Offline: o.Offline, At: o.Provenance.Ended}
	}
	rows, e = q.QueryContext(ctx, "SELECT id,stamp,entity FROM equipment_operations ORDER BY rowid")
	if e != nil {
		return st, e
	}
	defer rows.Close()
	for rows.Next() {
		var op equipment.Operation
		if e = rows.Scan(&op.ID, &op.Stamp, &op.Entity); e != nil {
			return st, e
		}
		st.Operations = append(st.Operations, op)
	}
	return st, rows.Err()
}
func (s *Store) UpdateEquipment(ctx context.Context, id string, fn func(*equipment.State) equipment.Code) equipment.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != id {
		return equipment.Changed
	}
	if s.checkActive() != nil {
		return equipment.Unavailable
	}
	tx, e := s.unit.BeginTx(ctx, nil)
	if e != nil {
		return equipment.Unavailable
	}
	defer tx.Rollback()
	st, e := s.loadEquipment(ctx, tx, id)
	if e != nil {
		return equipment.Unavailable
	}
	before, _ := json.Marshal(st)
	old := equipment.State{}
	if json.Unmarshal(before, &old) != nil {
		return equipment.Unavailable
	}
	oldCredentials := map[string][]byte{}
	oldDevices := map[string][]byte{}
	oldOps := map[string]bool{}
	for _, c := range old.Credentials {
		b, _ := json.Marshal(c)
		oldCredentials[c.Metadata.ID] = b
	}
	for _, d := range old.Devices {
		b, _ := json.Marshal(d)
		oldDevices[d.ID] = b
	}
	for _, op := range old.Operations {
		oldOps[op.ID] = true
	}
	if c := fn(&st); c != equipment.OK {
		return c
	}
	if len(st.WrappedKey) > 0 && !bytes.Equal(st.WrappedKey, old.WrappedKey) {
		if _, e = tx.ExecContext(ctx, "INSERT INTO vault_key VALUES(1,?) ON CONFLICT(singleton) DO UPDATE SET envelope=excluded.envelope", st.WrappedKey); e != nil {
			return equipment.Unavailable
		}
	}
	for _, c := range st.Credentials {
		b, _ := json.Marshal(c)
		if bytes.Equal(b, oldCredentials[c.Metadata.ID]) {
			continue
		}
		m := c.Metadata
		if _, e = tx.ExecContext(ctx, "INSERT INTO credentials VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET alias=excluded.alias,username=excluded.username,revision=excluded.revision,envelope=excluded.envelope", m.ID, m.Alias, m.User, m.Revision, c.Envelope); e != nil {
			return equipment.Unavailable
		}
	}
	for _, d := range st.Devices {
		b, _ := json.Marshal(d)
		if bytes.Equal(b, oldDevices[d.ID]) {
			continue
		}
		if d.Role == "" {
			d.Role = "other"
		}
		var observation any
		if d.Result != nil {
			b, err := json.Marshal(d.Result)
			if err != nil || len(b) > 48*1024 {
				return equipment.Limit
			}
			observation = string(b)
		}
		var credentialID any
		if d.CredentialID != "" {
			credentialID = d.CredentialID
		}
		var acceptance, attempt any
		if d.TrustAcceptance != nil {
			b, err := json.Marshal(d.TrustAcceptance)
			if err != nil {
				return equipment.Unavailable
			}
			acceptance = string(b)
		}
		if d.LastAttempt != nil {
			b, err := json.Marshal(d.LastAttempt)
			if err != nil {
				return equipment.Unavailable
			}
			attempt = string(b)
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation,trust_acceptance,last_attempt,role) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,address=excluded.address,port=excluded.port,transport=excluded.transport,credential_id=excluded.credential_id,revision=excluded.revision,active=excluded.active,fingerprint=excluded.fingerprint,trust_origin=excluded.trust_origin,observation=excluded.observation,trust_acceptance=excluded.trust_acceptance,last_attempt=excluded.last_attempt,role=excluded.role", d.ID, d.Name, d.Address, d.Port, d.Transport, credentialID, d.Revision, d.Active, d.Fingerprint, d.TrustOrigin, observation, acceptance, attempt, d.Role); e != nil {
			return equipment.Unavailable
		}
	}
	for _, op := range st.Operations {
		if oldOps[op.ID] {
			continue
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?) ON CONFLICT(id) DO NOTHING", op.ID, op.Stamp, op.Entity); e != nil {
			return equipment.Unavailable
		}
	}
	s.observe("before-equipment-commit")
	if tx.Commit() != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}
