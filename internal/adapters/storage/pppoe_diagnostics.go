package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"strings"
)

func ppTotalBytes(ctx context.Context, tx *sql.Tx) (int64, error) {
	var bytes int64
	err := tx.QueryRowContext(ctx, "SELECT (SELECT COALESCE(sum(payload_bytes),0) FROM pppoe_observations)+(SELECT COALESCE(sum(payload_bytes),0) FROM pppoe_diagnostic_observations)").Scan(&bytes)
	return bytes, err
}
func (s *Store) ReservePPDiagnostic(ctx context.Context, company string, in equipment.PPInput, stamp, job string) (string, bool, equipment.Code) {
	entity, known := "", false
	c := s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		if job == "" {
			return equipment.OK
		}
		target, code := ppTarget(ctx, tx, s.installation, company, in.DeviceID, false)
		if code != equipment.OK {
			return code
		}
		if target.Device.Revision != in.Revision {
			return equipment.Revision
		}
		if !target.Device.Active {
			return equipment.Inactive
		}
		if !equipment.PDValidScope(in.Scope) {
			return equipment.Invalid
		}

		attempt := equipment.PPAttempt{Epoch: in.WorkerEpoch, JobID: job, State: "RUNNING", Code: equipment.OK, At: in.SavedAt, Operation: in.Operation, Scope: in.Scope, SourceObservation: in.ObservationID, SourceRow: in.RowID}
		body, _ := json.Marshal(attempt)
		_, err := tx.ExecContext(ctx, "INSERT INTO pppoe_diagnostic_attempts VALUES(?,?,?,?,?) ON CONFLICT(device_id,scope) DO UPDATE SET operation_id=excluded.operation_id,job_id=excluded.job_id,meta_json=excluded.meta_json", in.DeviceID, in.Scope, in.Operation, job, string(body))
		if err != nil {
			return equipment.Unavailable
		}
		entity = job
		return equipment.OK
	})
	return entity, known, c
}

func pdBudget(o *equipment.PDDiagnostic) bool {
	if o == nil || !equipment.PDValidScope(o.Scope) || o.Quality.Status != "complete" || !o.Quality.Terminated || !o.Quality.MembershipChecked || len(o.Rows) > equipment.PPMaxRows {
		return false
	}
	o.Count = len(o.Rows)
	o.Bytes = 0
	seen := map[string]bool{}
	radius := 0
	for _, row := range o.Rows {
		if row.ID == "" || seen[row.ID] {
			return false
		}
		seen[row.ID] = true
		switch o.Scope {
		case equipment.PDCatalog:
			if row.Kind != "interface" && row.Kind != "vlan" && row.Kind != "service" {
				return false
			}
		case equipment.PDRadius:
			if row.Kind == "radius" {
				radius++
				if row.Counters == nil {
					return false
				}
			} else if row.Kind != "aaa" {
				return false
			}
		case equipment.PDPing:
			if row.Kind != "ping" || row.Ping == nil || row.Ping.Sent != 3 || len(row.Ping.Probes) != 3 || len(o.Rows) != 1 {
				return false
			}
		}
		body, err := json.Marshal(row)
		if err != nil || len(body) > equipment.PPRowBytes {
			return false
		}
		o.Bytes += len(body)
	}
	if radius > equipment.PDMaxRadius {
		return false
	}
	rowsBytes := o.Bytes
	for i := 0; i < 16; i++ {
		body, err := json.Marshal(o)
		if err != nil || len(body) > 16384 {
			return false
		}
		measured := rowsBytes + len(body)
		if measured == o.Bytes {
			return measured <= equipment.PPObservationBytes
		}
		o.Bytes = measured
	}
	return false
}

func (s *Store) FinishPPDiagnostic(ctx context.Context, company string, target equipment.PPTarget, in equipment.PPInput, job string, attempt equipment.PPAttempt, o *equipment.PDDiagnostic) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		s.observe("pp-phase-read-start")
		current, code := ppTarget(ctx, tx, s.installation, company, target.Device.ID, false)
		if code != equipment.OK {
			return code
		}
		if !current.Device.Active || current.Device.Revision != target.Device.Revision || equipment.PPConnection(current.Device) != equipment.PPConnection(target.Device) || current.Credential.Metadata.Revision != target.Credential.Metadata.Revision {
			return equipment.Revision
		}
		var active string
		if tx.QueryRowContext(ctx, "SELECT job_id FROM pppoe_diagnostic_attempts WHERE device_id=? AND scope=?", target.Device.ID, in.Scope).Scan(&active) != nil || active != job {
			return equipment.Revision
		}
		if o != nil {
			s.observe("pp-phase-serialize-start")
			if o.Scope != in.Scope || o.DeviceID != target.Device.ID || !pdBudget(o) {
				return equipment.Limit
			}
			body, _ := json.Marshal(o)
			s.observe("pp-phase-inserts-start")
			if _, err := tx.ExecContext(ctx, "INSERT INTO pppoe_diagnostic_observations VALUES(?,?,?,?,?,?)", o.ID, o.DeviceID, o.Scope, string(body), o.Count, o.Bytes); err != nil {
				return equipment.Unavailable
			}
			// Bound each SQL call to the existing page cardinality. This avoids
			// thousands of driver/context crossings under the unchanged1s commit
			// budget, while the complete observation still commits atomically.
			valuesFor := func(n int) string { return strings.TrimSuffix(strings.Repeat("(?,?,?,?),", n), ",") }
			full, e := tx.PrepareContext(ctx, "INSERT INTO pppoe_diagnostic_rows VALUES"+valuesFor(equipment.PageSize))
			if e != nil {
				return equipment.Unavailable
			}
			defer full.Close()
			for start := 0; start < len(o.Rows); start += equipment.PageSize {
				end := start + equipment.PageSize
				if end > len(o.Rows) {
					end = len(o.Rows)
				}
				args := make([]any, 0, (end-start)*4)
				for ordinal := start; ordinal < end; ordinal++ {
					row := o.Rows[ordinal]
					body, _ := json.Marshal(row)
					args = append(args, o.ID, row.ID, ordinal, string(body))
				}
				var err error
				if end-start == equipment.PageSize {
					_, err = full.ExecContext(ctx, args...)
				} else {
					_, err = tx.ExecContext(ctx, "INSERT INTO pppoe_diagnostic_rows VALUES"+valuesFor(end-start), args...)
				}
				if err != nil {
					s.observe("pp-phase-inserts-error")
					return equipment.Unavailable
				}
			}

			if _, err := tx.ExecContext(ctx, "INSERT INTO pppoe_diagnostic_current VALUES(?,?,?) ON CONFLICT(device_id,scope) DO UPDATE SET diagnostic_id=excluded.diagnostic_id", o.DeviceID, o.Scope, o.ID); err != nil {
				return equipment.Unavailable
			}
			s.observe("pp-phase-cascade-start")
			if _, err := tx.ExecContext(ctx, "DELETE FROM pppoe_diagnostic_observations WHERE NOT EXISTS(SELECT 1 FROM pppoe_diagnostic_current WHERE diagnostic_id=pppoe_diagnostic_observations.id)"); err != nil {
				return equipment.Unavailable
			}
			bytes, err := ppTotalBytes(ctx, tx)
			if err != nil {
				return equipment.Unavailable
			}
			if bytes > equipment.PPCompanyBytes {
				return equipment.Limit
			}
			s.observe("pp-phase-cascade-end")
		}
		body, _ := json.Marshal(attempt)
		if _, err := tx.ExecContext(ctx, "UPDATE pppoe_diagnostic_attempts SET meta_json=? WHERE device_id=? AND scope=? AND job_id=?", string(body), target.Device.ID, in.Scope, job); err != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}

func pdMeta(ctx context.Context, q queryer, device, scope, id string) (*equipment.PDDiagnostic, equipment.Code) {
	var body string
	query := "SELECT o.meta_json FROM pppoe_diagnostic_observations o JOIN pppoe_diagnostic_current c ON c.diagnostic_id=o.id WHERE c.device_id=? AND c.scope=?"
	args := []any{device, scope}
	if id != "" {
		query += " AND o.id=?"
		args = append(args, id)
	}
	err := q.QueryRowContext(ctx, query, args...).Scan(&body)
	if err == sql.ErrNoRows {
		return nil, equipment.ObservationMissing
	}
	if err != nil {
		return nil, equipment.Unavailable
	}
	var result equipment.PDDiagnostic
	if json.Unmarshal([]byte(body), &result) != nil {
		return nil, equipment.Unavailable
	}
	return &result, equipment.OK
}
func (s *Store) PPDiagnosticCurrent(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{}
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	o, c := pdMeta(ctx, s.unit, in.DeviceID, in.Scope, "")
	if c != equipment.OK && c != equipment.ObservationMissing {
		return v, c
	}
	v.Diagnostic = o
	var body string
	err := s.unit.QueryRowContext(ctx, "SELECT meta_json FROM pppoe_diagnostic_attempts WHERE device_id=? AND scope=?", in.DeviceID, in.Scope).Scan(&body)
	if err == nil {
		var attempt equipment.PPAttempt
		if json.Unmarshal([]byte(body), &attempt) != nil {
			return v, equipment.Unavailable
		}
		v.Attempt = &attempt
	} else if err != sql.ErrNoRows {
		return v, equipment.Unavailable
	}
	return v, equipment.OK
}
func (s *Store) PPDiagnosticPage(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := equipment.PPView{}
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	o, c := pdMeta(ctx, s.unit, in.DeviceID, in.Scope, in.DiagnosticID)
	if c != equipment.OK {
		return v, c
	}
	if in.Offset < 0 || in.Offset > o.Count {
		return v, equipment.CursorInvalid
	}
	v.Diagnostic = o
	v.Total = o.Count
	where := "diagnostic_id=?"
	args := []any{o.ID}
	if in.Query != nil && in.Query.Type != "" {
		if in.Scope != equipment.PDCatalog || !equipment.PPValidQuery(in.Query) {
			return v, equipment.Invalid
		}
		where += " AND (json_extract(row_json,'$.type')=? OR json_extract(row_json,'$.kind')=?)"
		args = append(args, in.Query.Type, in.Query.Type)
	}
	if s.unit.QueryRowContext(ctx, "SELECT count(*) FROM pppoe_diagnostic_rows WHERE "+where, args...).Scan(&v.Total) != nil {
		return v, equipment.Unavailable
	}
	if in.Offset > v.Total {
		return v, equipment.CursorInvalid
	}
	args = append(args, equipment.PageSize, in.Offset)
	rows, err := s.unit.QueryContext(ctx, "SELECT row_json FROM pppoe_diagnostic_rows WHERE "+where+" ORDER BY ordinal LIMIT ? OFFSET ?", args...)
	if err != nil {
		return v, equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		var body string
		var row equipment.PDRow
		if rows.Scan(&body) != nil || json.Unmarshal([]byte(body), &row) != nil {
			return v, equipment.Unavailable
		}
		v.DiagnosticRows = append(v.DiagnosticRows, row)
	}
	if rows.Err() != nil {
		return v, equipment.Unavailable
	}
	v.NextOffset = in.Offset + len(v.DiagnosticRows)
	v.More = v.NextOffset < v.Total
	return v, equipment.OK
}
