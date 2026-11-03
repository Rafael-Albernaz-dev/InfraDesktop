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

func (s *Store) FinishPPCollection(
	ctx context.Context,
	company string,
	target equipment.PPTarget,
	job string,
	a equipment.PPAttempt,
	o *equipment.PPObservation,
) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		s.observe("pp-phase-read-start")
		t, code := s.verifyCollectionTarget(ctx, tx, company, target, job)
		if code != equipment.OK {
			return code
		}
		if o != nil {
			if code = s.persistPPCollection(ctx, tx, o); code != equipment.OK {
				return code
			}
		} else {
			_, err := tx.ExecContext(
				ctx, "UPDATE device_monitoring_checkpoint SET continuous=0 WHERE device_id=?",
				t.Device.ID,
			)
			if err != nil {
				return equipment.Unavailable
			}
		}
		b, _ := json.Marshal(a)
		_, e := tx.ExecContext(
			ctx,
			"UPDATE pppoe_attempts SET meta_json=? WHERE device_id=? AND scope=? AND job_id=?",
			string(b), t.Device.ID, equipment.PPScope, job,
		)
		if e != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}

func (s *Store) verifyCollectionTarget(
	ctx context.Context, tx *sql.Tx, company string, target equipment.PPTarget, job string,
) (equipment.PPTarget, equipment.Code) {
	t, c := ppTarget(ctx, tx, s.installation, company, target.Device.ID, false)
	if c != equipment.OK {
		return t, c
	}
	sameConn := equipment.PPConnection(t.Device) == equipment.PPConnection(target.Device)
	sameCred := t.Credential.Metadata.Revision == target.Credential.Metadata.Revision
	if !t.Device.Active || t.Device.Revision != target.Device.Revision || !sameConn || !sameCred {
		return t, equipment.Revision
	}
	var currentJob string
	err := tx.QueryRowContext(
		ctx, "SELECT job_id FROM pppoe_attempts WHERE device_id=? AND scope=?",
		t.Device.ID, equipment.PPScope,
	).Scan(&currentJob)
	if err != nil || currentJob != job {
		return t, equipment.Revision
	}
	return t, equipment.OK
}

func (s *Store) persistPPCollection(
	ctx context.Context, tx *sql.Tx, o *equipment.PPObservation,
) equipment.Code {
	previous, previousID, code := loadPreviousPPObservation(ctx, tx, o.DeviceID)
	if code != equipment.OK {
		return code
	}
	if code = equipment.ReconcilePP(previous, o); code != equipment.OK {
		return code
	}
	s.observe("pp-phase-serialize-start")
	if code = enforcePPObservationQuota(ctx, tx, o, previousID); code != equipment.OK {
		return code
	}
	s.observe("pp-phase-inserts-start")
	if code = s.insertPPObservationAndRows(ctx, tx, o); code != equipment.OK {
		return code
	}
	s.observe("pp-phase-cascade-start")
	if code = finalizePPObservationCascade(ctx, tx, previous, o); code != equipment.OK {
		return code
	}
	s.observe("pp-phase-cascade-end")
	return equipment.OK
}

func loadPreviousPPObservation(
	ctx context.Context, tx *sql.Tx, deviceID string,
) (*equipment.PPObservation, string, equipment.Code) {
	var previousID string
	err := tx.QueryRowContext(
		ctx, "SELECT observation_id FROM pppoe_current WHERE device_id=? AND scope=?",
		deviceID, equipment.PPScope,
	).Scan(&previousID)
	if err == sql.ErrNoRows {
		return nil, "", equipment.OK
	}
	if err != nil {
		return nil, "", equipment.Unavailable
	}
	previous, code := ppMeta(ctx, tx, deviceID, previousID)
	if code != equipment.OK {
		return nil, "", code
	}
	rows, e := tx.QueryContext(
		ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal", previousID,
	)
	if e != nil {
		return nil, "", equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		var b string
		var r equipment.PPRow
		if rows.Scan(&b) != nil || json.Unmarshal([]byte(b), &r) != nil {
			return nil, "", equipment.Unavailable
		}
		previous.Rows = append(previous.Rows, r)
	}
	if rows.Err() != nil {
		return nil, "", equipment.Unavailable
	}
	return previous, previousID, equipment.OK
}

func enforcePPObservationQuota(
	ctx context.Context, tx *sql.Tx, o *equipment.PPObservation, previousID string,
) equipment.Code {
	if !ppWithinBudget(o) {
		return equipment.Limit
	}
	existing, err := ppTotalBytes(ctx, tx)
	if err != nil {
		return equipment.Unavailable
	}
	var reclaim int64
	if previousID != "" {
		query := "SELECT CASE WHEN EXISTS(SELECT 1 FROM pppoe_references r " +
			"WHERE r.observation_id=o.id) THEN 0 ELSE o.payload_bytes END " +
			"FROM pppoe_observations o WHERE o.id=?"
		if tx.QueryRowContext(ctx, query, previousID).Scan(&reclaim) != nil {
			return equipment.Unavailable
		}
	}
	if existing-reclaim+int64(o.Bytes) > equipment.PPCompanyBytes {
		target := equipment.PPCompanyBytes + reclaim - int64(o.Bytes)
		err = trimMonitoring(ctx, tx, target, time.Now().Unix())
		if err != nil && !errors.Is(err, errMonitoringQuota) {
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
	return equipment.OK
}

func (s *Store) insertPPObservationAndRows(
	ctx context.Context, tx *sql.Tx, o *equipment.PPObservation,
) equipment.Code {
	b, _ := json.Marshal(o)
	_, err := tx.ExecContext(
		ctx, "INSERT INTO pppoe_observations VALUES(?,?,?,?,?,?,?)",
		o.ID, o.DeviceID, equipment.PPScope, string(b), len(b), o.Count, o.Bytes,
	)
	if err != nil {
		return equipment.Unavailable
	}
	if code := s.insertPPRowBatches(ctx, tx, o); code != equipment.OK {
		return code
	}
	query := "INSERT INTO pppoe_current VALUES(?,?,?) " +
		"ON CONFLICT(device_id,scope) DO UPDATE SET observation_id=excluded.observation_id"
	if _, err = tx.ExecContext(ctx, query, o.DeviceID, equipment.PPScope, o.ID); err != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}

func (s *Store) insertPPRowBatches(
	ctx context.Context, tx *sql.Tx, o *equipment.PPObservation,
) equipment.Code {
	valuesFor := func(n int) string {
		return strings.TrimSuffix(strings.Repeat("(?,?,?,?,?,?,?,?,?,?),", n), ",")
	}
	full, e := tx.PrepareContext(ctx, "INSERT INTO pppoe_rows VALUES"+valuesFor(equipment.PageSize))
	if e != nil {
		return equipment.Unavailable
	}
	defer full.Close()
	for start := 0; start < len(o.Rows); start += equipment.PageSize {
		end := min(start+equipment.PageSize, len(o.Rows))
		args := buildPPRowBatchArgs(o, start, end)
		var err error
		if end-start == equipment.PageSize {
			_, err = full.ExecContext(ctx, args...)
		} else {
			_, err = tx.ExecContext(
				ctx, "INSERT INTO pppoe_rows VALUES"+valuesFor(end-start), args...,
			)
		}
		if err != nil {
			s.observe("pp-phase-inserts-error")
			return equipment.Unavailable
		}
	}
	return equipment.OK
}

func buildPPRowBatchArgs(o *equipment.PPObservation, start, end int) []any {
	args := make([]any, 0, (end-start)*10)
	for i := start; i < end; i++ {
		r := o.Rows[i]
		body, _ := json.Marshal(r)
		var uptime any
		if r.Uptime.Known {
			uptime = r.Uptime.Min
		}
		search := strings.ToLower(r.Login + " " + r.Address + " " + r.Caller)
		args = append(
			args,
			o.ID, r.ID, i, r.Login, nullablePP(r.Address),
			nullablePP(r.Caller), search, uptime, string(body), len(body),
		)
	}
	return args
}

func finalizePPObservationCascade(
	ctx context.Context, tx *sql.Tx, previous, o *equipment.PPObservation,
) equipment.Code {
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
	if err = recordSessionMonitoring(ctx, tx, previous, o); err != nil {
		if errors.Is(err, errMonitoringQuota) {
			return equipment.Limit
		}
		return equipment.Unavailable
	}
	finalBytes, err := ppTotalBytes(ctx, tx)
	if err != nil {
		return equipment.Unavailable
	}
	if finalBytes > equipment.PPCompanyBytes {
		return equipment.Limit
	}
	return equipment.OK
}

func ppWithinBudget(o *equipment.PPObservation) bool {
	if o == nil || len(o.Rows) > equipment.PPMaxRows ||
		o.Quality.Status != "complete" || !o.Quality.Terminated || !o.Quality.MembershipChecked {
		return false
	}
	rowsBytes, oversized, ok := measurePPRowBytes(o.Rows)
	if !ok {
		return false
	}
	if oversized {
		if !ppOmitContext(o) {
			return false
		}
		return ppWithinBudget(o)
	}
	return solvePPObservationSize(o, rowsBytes)
}

func measurePPRowBytes(rows []equipment.PPRow) (int, bool, bool) {
	rowsBytes := 0
	oversized := false
	seen := map[string]bool{}
	for _, r := range rows {
		b, e := json.Marshal(r)
		if e != nil || seen[r.ID] || r.ID == "" {
			return 0, false, false
		}
		oversized = oversized || len(b) > equipment.PPRowBytes
		seen[r.ID] = true
		rowsBytes += len(b)
	}
	return rowsBytes, oversized, true
}

func solvePPObservationSize(o *equipment.PPObservation, rowsBytes int) bool {
	o.Count = len(o.Rows)
	originalRows := len(o.Rows)
	for removed := 0; removed <= originalRows; removed++ {
		if !stabilizePPMetadataBytes(o, rowsBytes) {
			return false
		}
		if o.Bytes <= equipment.PPObservationBytes {
			return true
		}
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

func stabilizePPMetadataBytes(o *equipment.PPObservation, rowsBytes int) bool {
	for iteration := 0; iteration < 16; iteration++ {
		b, e := json.Marshal(o)
		if e != nil || len(b) > 16384 {
			return false
		}
		n := rowsBytes + len(b)
		if n == o.Bytes {
			return true
		}
		o.Bytes = n
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

func (s *Store) PPContextCounts(
	ctx context.Context, company string, in equipment.PPInput,
) (equipment.PPView, equipment.Code) {
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
		return v, equipment.OK
	}
	if !loadPPObservationRows(ctx, s.unit, o) {
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
		return paginatePPGroups(v, all, in.Offset)
	}
	return s.queryLivePPContextGroups(ctx, v, in)
}

func loadPPObservationRows(ctx context.Context, q queryer, o *equipment.PPObservation) bool {
	rows, e := q.QueryContext(
		ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal", o.ID,
	)
	if e != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var row equipment.PPRow
		if rows.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &row) != nil {
			return false
		}
		o.Rows = append(o.Rows, row)
	}
	return rows.Err() == nil
}

func paginatePPGroups(
	v equipment.PPView, all []equipment.PPGroup, offset int,
) (equipment.PPView, equipment.Code) {
	v.Total = len(all)
	if offset > v.Total {
		return v, equipment.CursorInvalid
	}
	end := min(offset+equipment.PageSize, v.Total)
	v.Groups = all[offset:end]
	v.NextOffset = end
	v.More = end < v.Total
	return v, equipment.OK
}

func (s *Store) queryLivePPContextGroups(
	ctx context.Context, v equipment.PPView, in equipment.PPInput,
) (equipment.PPView, equipment.Code) {
	query := `SELECT COALESCE(json_extract(row_json,'$.context.service'),''),` +
		`json_extract(row_json,'$.context.vlan'),count(*) FROM pppoe_rows ` +
		`WHERE observation_id=? AND COALESCE(json_extract(row_json,'$.state'),'ONLINE')='ONLINE' ` +
		`GROUP BY 1,2 ORDER BY 1,2`
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
