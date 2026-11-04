package storage

import (
	"context"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"strings"
)

const ppOnlineDuplicateCount = "(SELECT count(*) FROM pppoe_rows d WHERE d.observation_id=r.observation_id " +
	"AND d.login=r.login AND COALESCE(json_extract(d.row_json,'$.state'),'ONLINE')='ONLINE')"

const ppSessionStateSQL = "COALESCE(json_extract(r.row_json,'$.state'),'ONLINE')"

func ppSessionWhere(observation string, q *equipment.PPQuery) (string, []any) {
	where := "r.observation_id=? AND instr(r.search_text,?)>0"
	args := []any{observation, strings.ToLower(q.Search)}
	if q.Duplicates {
		where += " AND " + ppSessionStateSQL + "='ONLINE' AND r.login<>'' AND " + ppOnlineDuplicateCount + ">1"
	}

	if q.State != "" {
		where += " AND " + ppSessionStateSQL + "=?"
		args = append(args, q.State)
	}
	return where, args
}

func ppSessionOrder(q *equipment.PPQuery) string {
	column := map[string]string{"": "r.login", "login": "r.login", "address": "r.address",
		"uptime": "r.uptime_ns", "row": "r.row_id", "state": ppSessionStateSQL}[q.Sort]
	direction := " ASC"
	if q.Desc {
		direction = " DESC"
	}
	return column + " IS NULL," + column + direction + ",r.row_id" + direction
}

func scanPPSessionRow(rows interface{ Scan(...any) error }) (equipment.PPRow, equipment.Code) {
	var b string
	var count int
	if rows.Scan(&b, &count) != nil {
		return equipment.PPRow{}, equipment.Unavailable
	}
	var r equipment.PPRow
	if json.Unmarshal([]byte(b), &r) != nil {
		return equipment.PPRow{}, equipment.Unavailable
	}
	if r.Login != "" && r.State != "OFFLINE" {
		r.Duplicates = count
	}
	if r.State == "" {
		r.State = "ONLINE"
	}
	return r, equipment.OK
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
	where, args := ppSessionWhere(in.ObservationID, q)
	if s.unit.QueryRowContext(ctx, "SELECT count(*) FROM pppoe_rows r WHERE "+where, args...).Scan(&v.Total) != nil {
		return v, equipment.Unavailable
	}
	if in.Offset > v.Total {
		return v, equipment.CursorInvalid
	}
	order := ppSessionOrder(q)
	args = append(args, equipment.PageSize, in.Offset)
	rows, e := s.unit.QueryContext(ctx, "SELECT r.row_json,"+ppOnlineDuplicateCount+
		" FROM pppoe_rows r WHERE "+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", args...)
	if e != nil {
		return v, equipment.Unavailable
	}
	defer rows.Close()
	for rows.Next() {
		r, code := scanPPSessionRow(rows)
		if code != equipment.OK {
			return v, code
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
