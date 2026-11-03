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

func pdBudget(o *equipment.PDDiagnostic) bool {
	if o == nil || !equipment.PDValidScope(o.Scope) || o.Quality.Status != "complete" ||
		!o.Quality.Terminated || !o.Quality.MembershipChecked || len(o.Rows) > equipment.PPMaxRows {
		return false
	}
	o.Count = len(o.Rows)
	rowsBytes, ok := measurePDDiagnosticRows(o.Scope, o.Rows)
	if !ok {
		return false
	}
	o.Bytes = rowsBytes
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

func measurePDDiagnosticRows(scope string, rows []equipment.PDRow) (int, bool) {
	seen := map[string]bool{}
	radius := 0
	total := 0
	for _, row := range rows {
		if row.ID == "" || seen[row.ID] {
			return 0, false
		}
		seen[row.ID] = true
		if !validPDRowForScope(scope, row, len(rows), &radius) {
			return 0, false
		}
		body, err := json.Marshal(row)
		if err != nil || len(body) > equipment.PPRowBytes {
			return 0, false
		}
		total += len(body)
	}
	return total, radius <= equipment.PDMaxRadius
}

func validPDRowForScope(scope string, row equipment.PDRow, count int, radius *int) bool {
	switch scope {
	case equipment.PDCatalog:
		return row.Kind == "interface" || row.Kind == "vlan" || row.Kind == "service"
	case equipment.PDRadius:
		if row.Kind == "radius" {
			*radius++
			return row.Counters != nil
		}
		return row.Kind == "aaa"
	case equipment.PDPing:
		return row.Kind == "ping" && row.Ping != nil &&
			row.Ping.Sent == 3 && len(row.Ping.Probes) == 3 && count == 1
	default:
		return false
	}
}

func (s *Store) FinishPPDiagnostic(
	ctx context.Context,
	company string,
	target equipment.PPTarget,
	in equipment.PPInput,
	job string,
	attempt equipment.PPAttempt,
	o *equipment.PDDiagnostic,
) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		s.observe("pp-phase-read-start")
		if code := s.verifyDiagnosticTarget(ctx, tx, company, target, in.Scope, job); code != equipment.OK {
			return code
		}
		if o != nil {
			if code := s.persistDiagnosticObservation(ctx, tx, target.Device.ID, in.Scope, o); code != equipment.OK {
				return code
			}
		}
		body, _ := json.Marshal(attempt)
		query := "UPDATE pppoe_diagnostic_attempts SET meta_json=? " +
			"WHERE device_id=? AND scope=? AND job_id=?"
		_, err := tx.ExecContext(ctx, query, string(body), target.Device.ID, in.Scope, job)
		if err != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}

func (s *Store) verifyDiagnosticTarget(
	ctx context.Context, tx *sql.Tx, company string, target equipment.PPTarget, scope, job string,
) equipment.Code {
	current, code := ppTarget(ctx, tx, s.installation, company, target.Device.ID, false)
	if code != equipment.OK {
		return code
	}
	sameConn := equipment.PPConnection(current.Device) == equipment.PPConnection(target.Device)
	sameCred := current.Credential.Metadata.Revision == target.Credential.Metadata.Revision
	if !current.Device.Active || current.Device.Revision != target.Device.Revision ||
		!sameConn || !sameCred {
		return equipment.Revision
	}
	var active string
	err := tx.QueryRowContext(
		ctx, "SELECT job_id FROM pppoe_diagnostic_attempts WHERE device_id=? AND scope=?",
		target.Device.ID, scope,
	).Scan(&active)
	if err != nil || active != job {
		return equipment.Revision
	}
	return equipment.OK
}

func (s *Store) persistDiagnosticObservation(
	ctx context.Context, tx *sql.Tx, deviceID, scope string, o *equipment.PDDiagnostic,
) equipment.Code {
	s.observe("pp-phase-serialize-start")
	if o.Scope != scope || o.DeviceID != deviceID || !pdBudget(o) {
		return equipment.Limit
	}
	if o.Scope == equipment.PDCatalog {
		if code := reconcileCatalogDiagnostic(ctx, tx, o); code != equipment.OK {
			return code
		}
	}
	body, _ := json.Marshal(o)
	s.observe("pp-phase-inserts-start")
	_, err := tx.ExecContext(
		ctx, "INSERT INTO pppoe_diagnostic_observations VALUES(?,?,?,?,?,?)",
		o.ID, o.DeviceID, o.Scope, string(body), o.Count, o.Bytes,
	)
	if err != nil {
		return equipment.Unavailable
	}
	if code := s.insertPDDiagnosticRows(ctx, tx, o); code != equipment.OK {
		return code
	}
	query := "INSERT INTO pppoe_diagnostic_current VALUES(?,?,?) " +
		"ON CONFLICT(device_id,scope) DO UPDATE SET diagnostic_id=excluded.diagnostic_id"
	if _, err = tx.ExecContext(ctx, query, o.DeviceID, o.Scope, o.ID); err != nil {
		return equipment.Unavailable
	}
	s.observe("pp-phase-cascade-start")
	if code := finalizePDDiagnosticCascade(ctx, tx); code != equipment.OK {
		return code
	}
	s.observe("pp-phase-cascade-end")
	return equipment.OK
}

func reconcileCatalogDiagnostic(
	ctx context.Context, tx *sql.Tx, o *equipment.PDDiagnostic,
) equipment.Code {
	previous, c := pdMeta(ctx, tx, o.DeviceID, o.Scope, "")
	if c != equipment.OK && c != equipment.ObservationMissing {
		return c
	}
	if previous != nil && !loadPDDiagnosticRows(ctx, tx, previous) {
		return equipment.Unavailable
	}
	if c = equipment.ReconcileCatalogAbsences(previous, o); c != equipment.OK {
		return c
	}
	if !pdBudget(o) {
		current := o.Rows[:0]
		for _, row := range o.Rows {
			if row.NotSeenSince == "" {
				current = append(current, row)
			}
		}
		o.Rows = current
		o.AbsentInterfaces = 0
		o.AbsencesLimited = true
		if !pdBudget(o) {
			return equipment.Limit
		}
	}
	return equipment.OK
}

func loadPDDiagnosticRows(
	ctx context.Context, tx *sql.Tx, previous *equipment.PDDiagnostic,
) bool {
	q, e := tx.QueryContext(
		ctx, "SELECT row_json FROM pppoe_diagnostic_rows WHERE diagnostic_id=? ORDER BY ordinal",
		previous.ID,
	)
	if e != nil {
		return false
	}
	defer q.Close()
	for q.Next() {
		var raw string
		var row equipment.PDRow
		if q.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &row) != nil {
			return false
		}
		previous.Rows = append(previous.Rows, row)
	}
	return q.Err() == nil
}

func (s *Store) insertPDDiagnosticRows(
	ctx context.Context, tx *sql.Tx, o *equipment.PDDiagnostic,
) equipment.Code {
	valuesFor := func(n int) string {
		return strings.TrimSuffix(strings.Repeat("(?,?,?,?),", n), ",")
	}
	full, e := tx.PrepareContext(
		ctx, "INSERT INTO pppoe_diagnostic_rows VALUES"+valuesFor(equipment.PageSize),
	)
	if e != nil {
		return equipment.Unavailable
	}
	defer full.Close()
	for start := 0; start < len(o.Rows); start += equipment.PageSize {
		end := min(start+equipment.PageSize, len(o.Rows))
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
			_, err = tx.ExecContext(
				ctx, "INSERT INTO pppoe_diagnostic_rows VALUES"+valuesFor(end-start), args...,
			)
		}
		if err != nil {
			s.observe("pp-phase-inserts-error")
			return equipment.Unavailable
		}
	}
	return equipment.OK
}

func finalizePDDiagnosticCascade(ctx context.Context, tx *sql.Tx) equipment.Code {
	delQuery := "DELETE FROM pppoe_diagnostic_observations WHERE NOT EXISTS(" +
		"SELECT 1 FROM pppoe_diagnostic_current WHERE diagnostic_id=pppoe_diagnostic_observations.id)"
	if _, err := tx.ExecContext(ctx, delQuery); err != nil {
		return equipment.Unavailable
	}
	bytes, err := ppTotalBytes(ctx, tx)
	if err != nil {
		return equipment.Unavailable
	}
	if bytes > equipment.PPCompanyBytes {
		err = trimMonitoring(ctx, tx, equipment.PPCompanyBytes, time.Now().Unix())
		if err != nil && !errors.Is(err, errMonitoringQuota) {
			return equipment.Unavailable
		}
		bytes, err = ppTotalBytes(ctx, tx)
		if err != nil {
			return equipment.Unavailable
		}
		if bytes > equipment.PPCompanyBytes {
			return equipment.Limit
		}
	}
	return equipment.OK
}
