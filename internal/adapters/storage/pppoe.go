package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"infradesktop/internal/modules/equipment"
	"strings"
	"time"
)

type ppTx func(*sql.Tx) equipment.Code

func (s *Store) ppWrite(ctx context.Context, company string, fn ppTx) equipment.Code {
	s.observe("pp-phase-lock-start")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observe("pp-phase-lock-end")
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
	s.observe("pp-phase-commit-end")
	return equipment.OK
}
func ppTarget(ctx context.Context, q queryer, installation, company, id string, reveal bool) (equipment.PPTarget, equipment.Code) {
	t := equipment.PPTarget{Installation: installation, Company: company}
	d := &t.Device
	e := q.QueryRowContext(ctx, "SELECT id,name,address,port,transport,COALESCE(credential_id,''),revision,active,fingerprint,COALESCE((SELECT role FROM device_radio_roles WHERE device_id=devices.id),role) FROM devices WHERE id=?", id).Scan(&d.ID, &d.Name, &d.Address, &d.Port, &d.Transport, &d.CredentialID, &d.Revision, &d.Active, &d.Fingerprint, &d.Role)
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
		if job == "" {
			return equipment.OK
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
		s.observe("pp-phase-read-start")
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
			var previous *equipment.PPObservation
			var previousID string
			err := tx.QueryRowContext(ctx, "SELECT observation_id FROM pppoe_current WHERE device_id=? AND scope=?", o.DeviceID, equipment.PPScope).Scan(&previousID)
			if err == nil {
				var code equipment.Code
				previous, code = ppMeta(ctx, tx, o.DeviceID, previousID)
				if code != equipment.OK {
					return code
				}
				rows, e := tx.QueryContext(ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal", previousID)
				if e != nil {
					return equipment.Unavailable
				}
				for rows.Next() {
					var b string
					var r equipment.PPRow
					if rows.Scan(&b) != nil || json.Unmarshal([]byte(b), &r) != nil {
						rows.Close()
						return equipment.Unavailable
					}
					previous.Rows = append(previous.Rows, r)
				}
				e = rows.Err()
				rows.Close()
				if e != nil {
					return equipment.Unavailable
				}
			} else if err != sql.ErrNoRows {
				return equipment.Unavailable
			}
			if code := equipment.ReconcilePP(previous, o); code != equipment.OK {
				return code
			}
			s.observe("pp-phase-serialize-start")
			if !ppWithinBudget(o) {
				return equipment.Limit
			}
			// A+B share the company budget. Only an unreferenced previous current
			// can be reclaimed by the GC below; never edit a protected snapshot.
			existing, err := ppTotalBytes(ctx, tx)
			if err != nil {
				return equipment.Unavailable
			}
			var reclaim int64
			if previousID != "" && tx.QueryRowContext(ctx, "SELECT CASE WHEN EXISTS(SELECT 1 FROM pppoe_references r WHERE r.observation_id=o.id) THEN 0 ELSE o.payload_bytes END FROM pppoe_observations o WHERE o.id=?", previousID).Scan(&reclaim) != nil {
				return equipment.Unavailable
			}
			if existing-reclaim+int64(o.Bytes) > equipment.PPCompanyBytes {
				if e := trimMonitoring(ctx, tx, equipment.PPCompanyBytes+reclaim-int64(o.Bytes), time.Now().Unix()); e != nil && !errors.Is(e, errMonitoringQuota) {
					return equipment.Unavailable
				}
				existing, err = ppTotalBytes(ctx, tx)
				if err != nil {
					return equipment.Unavailable
				}
			}
			if existing-reclaim+int64(o.Bytes) > equipment.PPCompanyBytes {
				ppOmitContext(o)
				if !ppWithinBudget(o) || existing-reclaim+int64(o.Bytes) > equipment.PPCompanyBytes {
					return equipment.Limit
				}
			}
			b, _ := json.Marshal(o)
			s.observe("pp-phase-inserts-start")
			if _, e := tx.ExecContext(ctx, "INSERT INTO pppoe_observations VALUES(?,?,?,?,?,?,?)", o.ID, o.DeviceID, equipment.PPScope, string(b), len(b), o.Count, o.Bytes); e != nil {
				return equipment.Unavailable
			}
			var e error
			valuesFor := func(n int) string { return strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?,?,?,?,?),", n), ",") }
			full, e := tx.PrepareContext(ctx, "INSERT INTO pppoe_rows VALUES"+valuesFor(equipment.PageSize))
			if e != nil {
				return equipment.Unavailable
			}
			defer full.Close()
			for start := 0; start < len(o.Rows); start += equipment.PageSize {
				end := min(start+equipment.PageSize, len(o.Rows))
				args := make([]any, 0, (end-start)*10)
				for i := start; i < end; i++ {
					r := o.Rows[i]
					body, _ := json.Marshal(r)
					var uptime any
					if r.Uptime.Known {
						uptime = r.Uptime.Min
					}
					args = append(args, o.ID, r.ID, i, r.Login, nullablePP(r.Address), nullablePP(r.Caller), strings.ToLower(r.Login+" "+r.Address+" "+r.Caller), uptime, string(body), len(body))
				}
				var err error
				if end-start == equipment.PageSize {
					_, err = full.ExecContext(ctx, args...)
				} else {
					_, err = tx.ExecContext(ctx, "INSERT INTO pppoe_rows VALUES"+valuesFor(end-start), args...)
				}
				if err != nil {
					s.observe("pp-phase-inserts-error")
					return equipment.Unavailable
				}
			}

			if _, e = tx.ExecContext(ctx, "INSERT INTO pppoe_current VALUES(?,?,?) ON CONFLICT(device_id,scope) DO UPDATE SET observation_id=excluded.observation_id", o.DeviceID, equipment.PPScope, o.ID); e != nil {
				return equipment.Unavailable
			}
			s.observe("pp-phase-cascade-start")
			if ppGC(ctx, tx) != nil {
				return equipment.Unavailable
			}
			bytes, err := ppTotalBytes(ctx, tx)
			if err != nil {
				return equipment.Unavailable
			}
			if bytes > equipment.PPCompanyBytes {
				return equipment.Limit
			}
			if e := recordSessionMonitoring(ctx, tx, previous, o); e != nil {
				if errors.Is(e, errMonitoringQuota) {
					return equipment.Limit
				}
				return equipment.Unavailable
			}
			finalBytes, e := ppTotalBytes(ctx, tx)
			if e != nil {
				return equipment.Unavailable
			}
			if finalBytes > equipment.PPCompanyBytes {
				return equipment.Limit
			}
			s.observe("pp-phase-cascade-end")
		} else {
			if _, err := tx.ExecContext(ctx, "UPDATE device_monitoring_checkpoint SET continuous=0 WHERE device_id=?", t.Device.ID); err != nil {
				return equipment.Unavailable
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
		where += " AND COALESCE(json_extract(r.row_json,'$.state'),'ONLINE')='ONLINE' AND r.login<>'' AND (SELECT count(*) FROM pppoe_rows d WHERE d.observation_id=r.observation_id AND d.login=r.login AND COALESCE(json_extract(d.row_json,'$.state'),'ONLINE')='ONLINE')>1"
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
	rows, e := s.unit.QueryContext(ctx, "SELECT r.row_json,(SELECT count(*) FROM pppoe_rows d WHERE d.observation_id=r.observation_id AND d.login=r.login AND COALESCE(json_extract(d.row_json,'$.state'),'ONLINE')='ONLINE') FROM pppoe_rows r WHERE "+where+" ORDER BY r."+column+" IS NULL,r."+column+direction+",r.row_id"+direction+" LIMIT ? OFFSET ?", args...)
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
		if r.Login != "" && r.State != "OFFLINE" {
			r.Duplicates = count
		}
		if r.State == "" {
			r.State = "ONLINE"
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
	if r.State == "" {
		r.State = "ONLINE"
	}
	v.Rows = []equipment.PPRow{r}
	return v, equipment.OK
}

const ppReferenceProjection = "SELECT r.id,r.observation_id,r.created_at,o.row_count FROM pppoe_references r JOIN pppoe_observations o ON o.id=r.observation_id "

func scanPPReference(row interface{ Scan(...any) error }) (equipment.PPReference, error) {
	var ref equipment.PPReference
	err := row.Scan(&ref.ID, &ref.ObservationID, &ref.Created, &ref.Count)
	return ref, err
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
			ref, err := scanPPReference(tx.QueryRowContext(ctx, ppReferenceProjection+"WHERE r.id=? AND r.device_id=?", entity, in.DeviceID))
			if err == nil {
				v.Reference = &ref
				v.Effect = "committed"
			} else if err == sql.ErrNoRows {
				v.Effect = "removed"
			} else {
				return equipment.Unavailable
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
		ref, e := scanPPReference(tx.QueryRowContext(ctx, ppReferenceProjection+"WHERE r.device_id=? AND r.scope=? AND r.observation_id=?", in.DeviceID, equipment.PPScope, o.ID))
		if e == sql.ErrNoRows {
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
			if _, e = tx.ExecContext(ctx, "INSERT INTO pppoe_references VALUES(?,?,?,?,?)", in.ReferenceID, in.DeviceID, equipment.PPScope, o.ID, in.SavedAt); e != nil {
				return equipment.Unavailable
			}
			ref, e = scanPPReference(tx.QueryRowContext(ctx, ppReferenceProjection+"WHERE r.id=? AND r.device_id=?", in.ReferenceID, in.DeviceID))
			if e != nil {
				return equipment.Unavailable
			}
		} else if e != nil {
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
	rows, e := s.unit.QueryContext(ctx, ppReferenceProjection+"WHERE r.device_id=? ORDER BY r.created_at,r.id LIMIT ? OFFSET ?", in.DeviceID, equipment.PageSize, in.Offset)
	if e != nil {
		return v, equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanPPReference(rows)
		if err != nil {
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
	oversized := false
	seen := map[string]bool{}
	for _, r := range o.Rows {
		b, e := json.Marshal(r)
		if e != nil || seen[r.ID] || r.ID == "" {
			return false
		}
		oversized = oversized || len(b) > equipment.PPRowBytes
		seen[r.ID] = true
		rowsBytes += len(b)
	}
	if oversized {
		if !ppOmitContext(o) {
			return false
		}
		return ppWithinBudget(o)
	}
	o.Count = len(o.Rows)
	// Solve metadata self-size independently from the finite number of retained
	// absences. A removal changes digit widths and can require another fixed point.
	originalRows := len(o.Rows)
	for removed := 0; removed <= originalRows; removed++ {
		stable := false
		for iteration := 0; iteration < 16; iteration++ {
			b, e := json.Marshal(o)
			if e != nil || len(b) > 16384 {
				return false
			}
			n := rowsBytes + len(b)
			if n == o.Bytes {
				stable = true
				break
			}
			o.Bytes = n
		}
		if !stable {
			return false
		}
		if o.Bytes <= equipment.PPObservationBytes {
			return true
		}
		// Optional enrichment yields before any base row or retained absence.
		if ppOmitContext(o) {
			return ppWithinBudget(o)
		}
		if o.Offline == 0 || len(o.Rows) == 0 || o.Rows[len(o.Rows)-1].State != "OFFLINE" {
			return false
		}
		last, _ := json.Marshal(o.Rows[len(o.Rows)-1])
		rowsBytes -= len(last)
		o.Rows = o.Rows[:len(o.Rows)-1]
		o.Offline--
		o.Count--
		o.Bytes = 0
	}

	return false
}

func ppOmitContext(o *equipment.PPObservation) bool {
	removed := false
	for i := range o.Rows {
		if o.Rows[i].Context != nil {
			o.Rows[i].Context = nil
			removed = true
		}
	}
	if removed {
		o.ContextOmitted = true
		o.Bytes = 0
	}
	return removed
}

func (s *Store) PPContextCounts(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{}
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
	if o.ContextOmitted {
		// No grouping is asserted when optional context was budget-omitted.
		// Consumers use Current.ContextOmitted to distinguish this from empty.
		return v, equipment.OK
	}
	q, e := s.unit.QueryContext(ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal", o.ID)
	if e != nil {
		return v, equipment.Unavailable
	}
	for q.Next() {
		var raw string
		var row equipment.PPRow
		if e = q.Scan(&raw); e != nil {
			q.Close()
			return v, equipment.Unavailable
		}
		if e = json.Unmarshal([]byte(raw), &row); e != nil {
			q.Close()
			return v, equipment.Unavailable
		}
		o.Rows = append(o.Rows, row)
	}
	e = q.Err()
	q.Close()
	if e != nil {
		return v, equipment.Unavailable
	}
	window, e := time.Parse(time.RFC3339Nano, o.Provenance.Ended)
	if e != nil {
		return v, equipment.Unavailable
	}
	all, complete, e := contextPeakGroups(ctx, s.unit, o, window)
	if e != nil {
		return v, equipment.Unavailable
	}
	if complete {
		v.Total = len(all)
		if in.Offset > v.Total {
			return v, equipment.CursorInvalid
		}
		end := min(in.Offset+equipment.PageSize, v.Total)
		v.Groups = all[in.Offset:end]
		v.NextOffset = end
		v.More = end < v.Total
		return v, equipment.OK
	}
	query := `SELECT COALESCE(json_extract(row_json,'$.context.service'),''),json_extract(row_json,'$.context.vlan'),count(*) FROM pppoe_rows WHERE observation_id=? AND COALESCE(json_extract(row_json,'$.state'),'ONLINE')='ONLINE' GROUP BY 1,2 ORDER BY 1,2`
	rows, e := s.unit.QueryContext(ctx, query, in.ObservationID)
	if e != nil {
		return v, equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		var group equipment.PPGroup
		var vlan sql.NullInt64
		if rows.Scan(&group.Service, &vlan, &group.Count) != nil {
			return v, equipment.Unavailable
		}
		if vlan.Valid {
			n := int(vlan.Int64)
			group.VLAN = &n
		}
		if v.Total >= in.Offset && len(v.Groups) < equipment.PageSize {
			v.Groups = append(v.Groups, group)
		}
		v.Total++
	}
	if rows.Err() != nil {
		return v, equipment.Unavailable
	}
	if in.Offset > v.Total {
		return v, equipment.CursorInvalid
	}
	v.NextOffset = in.Offset + len(v.Groups)
	v.More = v.NextOffset < v.Total
	return v, equipment.OK
}
