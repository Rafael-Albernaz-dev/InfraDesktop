package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"strings"
)

type ppTx func(*sql.Tx) equipment.Code

func (s *Store) ppWrite(ctx context.Context, company string, fn ppTx) equipment.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
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
	if c := fn(tx); c != equipment.OK {
		return c
	}
	s.observe("before-pppoe-commit")
	if ctx.Err() != nil || tx.Commit() != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}
func ppTarget(ctx context.Context, q queryer, installation, company, id string, reveal bool) (equipment.PPTarget, equipment.Code) {
	t := equipment.PPTarget{Installation: installation, Company: company}
	d := &t.Device
	e := q.QueryRowContext(ctx, "SELECT id,address,port,transport,COALESCE(credential_id,''),revision,active,fingerprint FROM devices WHERE id=?", id).Scan(&d.ID, &d.Address, &d.Port, &d.Transport, &d.CredentialID, &d.Revision, &d.Active, &d.Fingerprint)
	if e == sql.ErrNoRows {
		return t, equipment.Invalid
	}
	if e != nil {
		return t, equipment.Unavailable
	}
	c := &t.Credential
	if reveal {
		e = q.QueryRowContext(ctx, "SELECT id,revision,username,envelope FROM credentials WHERE id=?", d.CredentialID).Scan(&c.Metadata.ID, &c.Metadata.Revision, &c.Metadata.User, &c.Envelope)
	} else {
		e = q.QueryRowContext(ctx, "SELECT id,revision FROM credentials WHERE id=?", d.CredentialID).Scan(&c.Metadata.ID, &c.Metadata.Revision)
	}
	if e == nil {
		e = q.QueryRowContext(ctx, "SELECT envelope FROM vault_key WHERE singleton=1").Scan(&t.Wrapped)
	}
	if e == sql.ErrNoRows {
		return t, equipment.MissingCredential
	}
	if e != nil {
		return t, equipment.Unavailable
	}
	return t, equipment.OK
}
func (s *Store) ReadPPTarget(ctx context.Context, company, id string, reveal bool) (equipment.PPTarget, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return equipment.PPTarget{}, equipment.Changed
	}
	if s.checkActive() != nil {
		return equipment.PPTarget{}, equipment.Unavailable
	}
	return ppTarget(ctx, s.unit, s.installation, company, id, reveal)
}

// ppReceipt returns known effects before checking the shared quota. It never prunes.
func ppReceipt(ctx context.Context, tx *sql.Tx, id, stamp string) (string, bool, equipment.Code) {
	var old, entity string
	e := tx.QueryRowContext(ctx, "SELECT stamp,entity FROM equipment_operations WHERE id=?", id).Scan(&old, &entity)
	if e == nil {
		if old != stamp {
			return "", true, equipment.Revision
		}
		return entity, true, equipment.OK
	}
	if e != sql.ErrNoRows {
		return "", false, equipment.Unavailable
	}
	var n int
	if tx.QueryRowContext(ctx, "SELECT count(*) FROM equipment_operations").Scan(&n) != nil {
		return "", false, equipment.Unavailable
	}
	if n >= equipment.MaxOperations {
		return "", false, equipment.Limit
	}
	return "", false, equipment.OK
}
func (s *Store) ReservePPCollection(ctx context.Context, company string, in equipment.PPInput, stamp, job string) (string, bool, equipment.Code) {
	entity, known := "", false
	c := s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		var c equipment.Code
		entity, known, c = ppReceipt(ctx, tx, in.Operation, stamp)
		if c != equipment.OK || known || job == "" {
			return c
		}
		t, c := ppTarget(ctx, tx, s.installation, company, in.DeviceID, false)
		if c != equipment.OK {
			return c
		}
		if t.Device.Revision != in.Revision {
			return equipment.Revision
		}
		if !t.Device.Active {
			return equipment.Inactive
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?)", in.Operation, stamp, job); e != nil {
			return equipment.Unavailable
		}
		a := equipment.PPAttempt{Epoch: in.WorkerEpoch, JobID: job, State: "RUNNING", Code: equipment.OK, At: in.SavedAt}
		b, _ := json.Marshal(a)
		_, e := tx.ExecContext(ctx, "INSERT INTO pppoe_attempts VALUES(?,?,?,?,?) ON CONFLICT(device_id,scope) DO UPDATE SET operation_id=excluded.operation_id,job_id=excluded.job_id,meta_json=excluded.meta_json", in.DeviceID, equipment.PPScope, in.Operation, job, string(b))
		if e != nil {
			return equipment.Unavailable
		}
		entity = job
		return equipment.OK
	})
	return entity, known, c
}
func ppGC(ctx context.Context, tx *sql.Tx) error {
	_, e := tx.ExecContext(ctx, "DELETE FROM pppoe_observations WHERE NOT EXISTS(SELECT 1 FROM pppoe_current WHERE observation_id=pppoe_observations.id) AND NOT EXISTS(SELECT 1 FROM pppoe_references WHERE observation_id=pppoe_observations.id)")
	return e
}
func (s *Store) FinishPPCollection(ctx context.Context, company string, target equipment.PPTarget, job string, a equipment.PPAttempt, o *equipment.PPObservation) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		t, c := ppTarget(ctx, tx, s.installation, company, target.Device.ID, false)
		if c != equipment.OK {
			return c
		}
		if !t.Device.Active || t.Device.Revision != target.Device.Revision || equipment.PPConnection(t.Device) != equipment.PPConnection(target.Device) || t.Credential.Metadata.Revision != target.Credential.Metadata.Revision {
			return equipment.Revision
		}
		var currentJob string
		if tx.QueryRowContext(ctx, "SELECT job_id FROM pppoe_attempts WHERE device_id=? AND scope=?", t.Device.ID, equipment.PPScope).Scan(&currentJob) != nil || currentJob != job {
			return equipment.Revision
		}
		if o != nil {
			if !ppWithinBudget(o) {
				return equipment.Limit
			}
			b, _ := json.Marshal(o)
			if _, e := tx.ExecContext(ctx, "INSERT INTO pppoe_observations VALUES(?,?,?,?,?,?,?)", o.ID, o.DeviceID, equipment.PPScope, string(b), len(b), o.Count, o.Bytes); e != nil {
				return equipment.Unavailable
			}
			stmt, e := tx.PrepareContext(ctx, "INSERT INTO pppoe_rows VALUES(?,?,?,?,?,?,?,?,?,?)")
			if e != nil {
				return equipment.Unavailable
			}
			defer stmt.Close()
			for i, r := range o.Rows {
				b, _ := json.Marshal(r)
				var uptime any
				if r.Uptime.Known {
					uptime = r.Uptime.Min
				}
				if _, e = stmt.ExecContext(ctx, o.ID, r.ID, i, r.Login, nullablePP(r.Address), nullablePP(r.Caller), strings.ToLower(r.Login+" "+r.Address+" "+r.Caller), uptime, string(b), len(b)); e != nil {
					return equipment.Unavailable
				}
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO pppoe_current VALUES(?,?,?) ON CONFLICT(device_id,scope) DO UPDATE SET observation_id=excluded.observation_id", o.DeviceID, equipment.PPScope, o.ID); e != nil {
				return equipment.Unavailable
			}
			if ppGC(ctx, tx) != nil {
				return equipment.Unavailable
			}
			var bytes int64
			if tx.QueryRowContext(ctx, "SELECT COALESCE(sum(payload_bytes),0) FROM pppoe_observations").Scan(&bytes) != nil {
				return equipment.Unavailable
			}
			if bytes > equipment.PPCompanyBytes {
				return equipment.Limit
			}
		}
		b, _ := json.Marshal(a)
		_, e := tx.ExecContext(ctx, "UPDATE pppoe_attempts SET meta_json=? WHERE device_id=? AND scope=? AND job_id=?", string(b), t.Device.ID, equipment.PPScope, job)
		if e != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}
func nullablePP(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func ppMeta(ctx context.Context, q queryer, device, id string) (*equipment.PPObservation, equipment.Code) {
	var b string
	e := q.QueryRowContext(ctx, "SELECT meta_json FROM pppoe_observations WHERE id=? AND device_id=? AND scope=?", id, device, equipment.PPScope).Scan(&b)
	if e == sql.ErrNoRows {
		return nil, equipment.ObservationMissing
	}
	if e != nil {
		return nil, equipment.Unavailable
	}
	var o equipment.PPObservation
	if json.Unmarshal([]byte(b), &o) != nil {
		return nil, equipment.Unavailable
	}
	return &o, equipment.OK
}
func (s *Store) PPObservation(ctx context.Context, company, device, id string) (*equipment.PPObservation, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return nil, equipment.Changed
	}
	if s.checkActive() != nil {
		return nil, equipment.Unavailable
	}
	return ppMeta(ctx, s.unit, device, id)
}
func (s *Store) PPCurrent(ctx context.Context, company, device string) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{}
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	var id string
	e := s.unit.QueryRowContext(ctx, "SELECT observation_id FROM pppoe_current WHERE device_id=? AND scope=?", device, equipment.PPScope).Scan(&id)
	if e == nil {
		var c equipment.Code
		v.Current, c = ppMeta(ctx, s.unit, device, id)
		if c != equipment.OK {
			return v, c
		}
	} else if e != sql.ErrNoRows {
		return v, equipment.Unavailable
	}
	var b string
	e = s.unit.QueryRowContext(ctx, "SELECT meta_json FROM pppoe_attempts WHERE device_id=? AND scope=?", device, equipment.PPScope).Scan(&b)
	if e == nil {
		v.Attempt = &equipment.PPAttempt{}
		if json.Unmarshal([]byte(b), v.Attempt) != nil {
			return v, equipment.Unavailable
		}
	} else if e != sql.ErrNoRows {
		return v, equipment.Unavailable
	}
	return v, equipment.OK
}
func (s *Store) PPPage(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{Rows: []equipment.PPRow{}}
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	o, c := ppMeta(ctx, s.unit, in.DeviceID, in.ObservationID)
	if c != equipment.OK {
		return v, c
	}
	v.Current = o
	q := in.Query
	if q == nil {
		q = &equipment.PPQuery{}
	}
	if !equipment.PPValidQuery(q) || in.Offset < 0 || in.Offset > equipment.PPMaxRows {
		return v, equipment.Invalid
	}
	where := "r.observation_id=? AND instr(r.search_text,?)>0"
	args := []any{in.ObservationID, strings.ToLower(q.Search)}
	if q.Duplicates {
		where += " AND r.login<>'' AND (SELECT count(*) FROM pppoe_rows d WHERE d.observation_id=r.observation_id AND d.login=r.login)>1"
	}
	if s.unit.QueryRowContext(ctx, "SELECT count(*) FROM pppoe_rows r WHERE "+where, args...).Scan(&v.Total) != nil {
		return v, equipment.Unavailable
	}
	if in.Offset > v.Total {
		return v, equipment.CursorInvalid
	}
	column := map[string]string{"": "login", "login": "login", "address": "address", "uptime": "uptime_ns", "row": "row_id"}[q.Sort]
	direction := " ASC"
	if q.Desc {
		direction = " DESC"
	}
	args = append(args, equipment.PageSize, in.Offset)
	rows, e := s.unit.QueryContext(ctx, "SELECT r.row_json,(SELECT count(*) FROM pppoe_rows d WHERE d.observation_id=r.observation_id AND d.login=r.login) FROM pppoe_rows r WHERE "+where+" ORDER BY r."+column+" IS NULL,r."+column+direction+",r.row_id"+direction+" LIMIT ? OFFSET ?", args...)
	if e != nil {
		return v, equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		var count int
		if rows.Scan(&b, &count) != nil {
			return v, equipment.Unavailable
		}
		var r equipment.PPRow
		if json.Unmarshal([]byte(b), &r) != nil {
			return v, equipment.Unavailable
		}
		if r.Login != "" {
			r.Duplicates = count
		}
		v.Rows = append(v.Rows, r)
	}
	if rows.Err() != nil {
		return v, equipment.Unavailable
	}
	v.NextOffset = in.Offset + len(v.Rows)
	v.More = v.NextOffset < v.Total
	return v, equipment.OK
}
func (s *Store) PPSession(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{}
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	if _, c := ppMeta(ctx, s.unit, in.DeviceID, in.ObservationID); c != equipment.OK {
		return v, c
	}
	var b string
	if s.unit.QueryRowContext(ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? AND row_id=?", in.ObservationID, in.RowID).Scan(&b) != nil {
		return v, equipment.ObservationMissing
	}
	var r equipment.PPRow
	if json.Unmarshal([]byte(b), &r) != nil {
		return v, equipment.Unavailable
	}
	v.Rows = []equipment.PPRow{r}
	return v, equipment.OK
}
func (s *Store) SavePPReference(ctx context.Context, company string, in equipment.PPInput, stamp string) (equipment.PPView, equipment.Code) {
	v := equipment.PPView{}
	c := s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		entity, known, c := ppReceipt(ctx, tx, in.Operation, stamp)
		if c != equipment.OK {
			v.Reason = "operation_receipts"
			return c
		}
		if known {
			v.Entity = entity
			v.Effect = "removed"
			var ref equipment.PPReference
			if tx.QueryRowContext(ctx, "SELECT id,observation_id,created_at FROM pppoe_references WHERE id=?", entity).Scan(&ref.ID, &ref.ObservationID, &ref.Created) == nil {
				v.Reference = &ref
				v.Effect = "committed"
			}
			return equipment.OK
		}
		o, c := ppMeta(ctx, tx, in.DeviceID, in.ObservationID)
		if c != equipment.OK {
			return c
		}
		var n int
		if tx.QueryRowContext(ctx, "SELECT count(*) FROM equipment_operations WHERE entity=? AND stamp LIKE 'ppref:%'", in.ReferenceID).Scan(&n) != nil {
			return equipment.Unavailable
		}
		if n > 0 {
			return equipment.Revision
		}
		ref := equipment.PPReference{ID: in.ReferenceID, ObservationID: o.ID, Created: in.SavedAt, Count: o.Count}
		var existing string
		e := tx.QueryRowContext(ctx, "SELECT id FROM pppoe_references WHERE device_id=? AND scope=? AND observation_id=?", in.DeviceID, equipment.PPScope, o.ID).Scan(&existing)
		if e == nil {
			ref.ID = existing
		} else if e == sql.ErrNoRows {
			if tx.QueryRowContext(ctx, "SELECT count(*) FROM pppoe_references WHERE device_id=?", in.DeviceID).Scan(&n) != nil {
				return equipment.Unavailable
			}
			if n >= equipment.PPReferenceDevice {
				v.Reason = "references_device"
				return equipment.Limit
			}
			if tx.QueryRowContext(ctx, "SELECT count(*) FROM pppoe_references").Scan(&n) != nil {
				return equipment.Unavailable
			}
			if n >= equipment.PPReferenceCompany {
				v.Reason = "references_company"
				return equipment.Limit
			}
			if _, e = tx.ExecContext(ctx, "INSERT INTO pppoe_references VALUES(?,?,?,?,?)", ref.ID, in.DeviceID, equipment.PPScope, o.ID, ref.Created); e != nil {
				return equipment.Unavailable
			}
		} else {
			return equipment.Unavailable
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?)", in.Operation, stamp, ref.ID); e != nil {
			return equipment.Unavailable
		}
		v.Reference = &ref
		v.Effect = "committed"
		return equipment.OK
	})
	return v, c
}
func (s *Store) PPReferences(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{References: []equipment.PPReference{}}
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	rows, e := s.unit.QueryContext(ctx, "SELECT r.id,r.observation_id,r.created_at,o.row_count FROM pppoe_references r JOIN pppoe_observations o ON o.id=r.observation_id WHERE r.device_id=? ORDER BY r.created_at,r.id LIMIT ? OFFSET ?", in.DeviceID, equipment.PageSize, in.Offset)
	if e != nil {
		return v, equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		var r equipment.PPReference
		if rows.Scan(&r.ID, &r.ObservationID, &r.Created, &r.Count) != nil {
			return v, equipment.Unavailable
		}
		v.References = append(v.References, r)
	}
	if rows.Err() != nil {
		return v, equipment.Unavailable
	}
	v.Total = len(v.References)
	return v, equipment.OK
}
func (s *Store) DeletePPReference(ctx context.Context, company string, in equipment.PPInput, stamp string) (equipment.PPView, equipment.Code) {
	v := equipment.PPView{}
	c := s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		entity, known, c := ppReceipt(ctx, tx, in.Operation, stamp)
		if c != equipment.OK {
			v.Reason = "operation_receipts"
			return c
		}
		if known {
			v.Entity = entity
			v.Effect = "removed"
			return equipment.OK
		}
		var id string
		if tx.QueryRowContext(ctx, "SELECT id FROM pppoe_references WHERE id=? AND device_id=?", in.ReferenceID, in.DeviceID).Scan(&id) != nil {
			return equipment.ObservationMissing
		}
		if _, e := tx.ExecContext(ctx, "DELETE FROM pppoe_references WHERE id=? AND device_id=?", id, in.DeviceID); e != nil {
			return equipment.Unavailable
		}
		if _, e := tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?)", in.Operation, stamp, id); e != nil {
			return equipment.Unavailable
		}
		if ppGC(ctx, tx) != nil {
			return equipment.Unavailable
		}
		v.Entity = id
		v.Effect = "removed"
		return equipment.OK
	})
	return v, c
}

var _ equipment.PPStore = (*Store)(nil)

func ppWithinBudget(o *equipment.PPObservation) bool {
	if o == nil || len(o.Rows) > equipment.PPMaxRows || o.Quality.Status != "complete" || !o.Quality.Terminated || !o.Quality.MembershipChecked {
		return false
	}
	rowsBytes := 0
	seen := map[string]bool{}
	for _, r := range o.Rows {
		b, e := json.Marshal(r)
		if e != nil || len(b) > equipment.PPRowBytes || seen[r.ID] || r.ID == "" {
			return false
		}
		seen[r.ID] = true
		rowsBytes += len(b)
	}
	o.Count = len(o.Rows)
	// Solve the tiny self-size fixed point so payload_bytes counts FINAL metadata.
	for i := 0; i < 16; i++ {
		b, e := json.Marshal(o)
		if e != nil || len(b) > 16384 {
			return false
		}
		n := rowsBytes + len(b)
		if n == o.Bytes {
			return n <= equipment.PPObservationBytes
		}
		o.Bytes = n
	}
	return false
}
