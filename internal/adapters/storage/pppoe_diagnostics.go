package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
)

func ppTotalBytes(ctx context.Context, tx *sql.Tx) (int64, error) {
	var bytes int64
	err := tx.QueryRowContext(ctx, "SELECT (SELECT COALESCE(sum(payload_bytes),0) FROM pppoe_observations)+(SELECT COALESCE(sum(payload_bytes),0) FROM pppoe_diagnostic_observations)+(SELECT COALESCE(sum(payload_bytes),0) FROM radio_current)+"+monitoringBytesExpression).Scan(&bytes)
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
