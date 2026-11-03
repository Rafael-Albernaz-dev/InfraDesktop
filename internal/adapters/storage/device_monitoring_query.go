package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
	"time"
)

func (s *Store) DeviceMonitoring(
	ctx context.Context, company string, in equipment.PPInput,
) (equipment.PPView, equipment.Code) {
	v := equipment.PPView{}
	s.mu.Lock()
	defer s.mu.Unlock()
	generationText, asOf, expectedHash, now, code := s.prepareMonitoringRead(ctx, company, in)
	if code != equipment.OK {
		return v, code
	}
	_, hourCut, edgeCut := equipment.MonitoringRetention(now)
	rows, versionDiv, packageDiv, compared, code := s.readMonitoringScope(
		ctx, in, now, hourCut, edgeCut,
	)
	if code != equipment.OK {
		return v, code
	}
	rows, code = s.appendRetentionRow(ctx, rows)
	if code != equipment.OK {
		return v, code
	}
	return finalizeMonitoringView(
		in, rows, generationText, asOf, expectedHash, now, versionDiv, packageDiv, compared,
	)
}

func (s *Store) prepareMonitoringRead(
	ctx context.Context, company string, in equipment.PPInput,
) (string, int64, string, time.Time, equipment.Code) {
	if s.unit == nil || s.unitID != company {
		return "", 0, "", time.Time{}, equipment.Changed
	}
	if s.checkActive() != nil {
		return "", 0, "", time.Time{}, equipment.Unavailable
	}
	var active bool
	err := s.unit.QueryRowContext(
		ctx, "SELECT active FROM devices WHERE id=?", in.DeviceID,
	).Scan(&active)
	if err != nil {
		return "", 0, "", time.Time{}, equipment.Invalid
	}
	var generation int64
	err = s.unit.QueryRowContext(
		ctx, "SELECT generation FROM device_monitoring_generation WHERE singleton=1",
	).Scan(&generation)
	if err != nil {
		return "", 0, "", time.Time{}, equipment.Unavailable
	}
	generationText := strconv.FormatInt(generation, 10)
	asOf, expectedHash, now, ok := parseMonitoringGeneration(in.MonitoringGeneration, generationText)
	if !ok {
		return "", 0, "", time.Time{}, equipment.CursorInvalid
	}
	return generationText, asOf, expectedHash, now, equipment.OK
}

func parseMonitoringGeneration(
	cursor, generationText string,
) (int64, string, time.Time, bool) {
	now := monitoringNow().UTC()
	asOf := now.Unix()
	if cursor == "" {
		return asOf, "", time.Unix(asOf, 0).UTC(), true
	}
	fields := strings.Split(cursor, "~")
	if len(fields) != 3 || fields[0] != generationText {
		return 0, "", time.Time{}, false
	}
	parsed, err := strconv.ParseInt(fields[1], 10, 64)
	maxRetention := int64(equipment.JobRetention / time.Second)
	if err != nil || parsed > now.Unix() || now.Unix()-parsed > maxRetention {
		return 0, "", time.Time{}, false
	}
	return parsed, fields[2], time.Unix(parsed, 0).UTC(), true
}

func (s *Store) readMonitoringScope(
	ctx context.Context, in equipment.PPInput, now time.Time, hourCut, edgeCut int64,
) ([]equipment.PPReadRow, *bool, *bool, int, equipment.Code) {
	switch in.Scope {
	case "duplicates":
		rows, code := s.monitoringDuplicates(ctx, in.DeviceID, now)
		return rows, nil, nil, 0, code
	case "equipment", "port-history":
		rows, code := s.monitoringEquipmentInsights(ctx, in.DeviceID, in.Scope, now)
		return rows, nil, nil, 0, code
	case "series":
		rows, code := s.monitoringSeriesRows(ctx, in.DeviceID, hourCut)
		return rows, nil, nil, 0, code
	case "flapping":
		rows, code := s.monitoringFlappingRows(ctx, in.DeviceID, edgeCut)
		return rows, nil, nil, 0, code
	case "versions":
		return s.monitoringVersionRows(ctx, hourCut)
	case "hygiene":
		rows, code := s.monitoringHygieneRows(ctx, in.DeviceID)
		return rows, nil, nil, 0, code
	default:
		return nil, nil, nil, 0, equipment.Invalid
	}
}

func (s *Store) monitoringDuplicates(
	ctx context.Context, deviceID string, now time.Time,
) ([]equipment.PPReadRow, equipment.Code) {
	rows, err := crossConcentratorRows(ctx, s.unit, deviceID, now)
	if err != nil {
		return nil, equipment.Unavailable
	}
	if len(rows) > equipment.PPMaxRows {
		return nil, equipment.Limit
	}
	return rows, equipment.OK
}

func (s *Store) monitoringEquipmentInsights(
	ctx context.Context, deviceID, scope string, now time.Time,
) ([]equipment.PPReadRow, equipment.Code) {
	rows, err := readEquipmentInsights(ctx, s.unit, deviceID, scope, now)
	if err != nil {
		return nil, equipment.Unavailable
	}
	if len(rows) > equipment.PPMaxRows {
		return nil, equipment.Limit
	}
	return rows, equipment.OK
}

func (s *Store) monitoringSeriesRows(
	ctx context.Context, deviceID string, hourCut int64,
) ([]equipment.PPReadRow, equipment.Code) {
	query := `SELECT bucket,resolution,session_at,min_online,max_online,last_online,` +
		`health_json,health_at,gap,drop_seen,samples FROM device_monitoring_points ` +
		`WHERE device_id=? AND bucket>=? ORDER BY bucket DESC,resolution`
	q, err := s.unit.QueryContext(ctx, query, deviceID, hourCut)
	if err != nil {
		return nil, equipment.Unavailable
	}
	defer q.Close()
	rows := []equipment.PPReadRow{}
	for q.Next() {
		point, ok := scanMonitoringPoint(q)
		if !ok {
			return nil, equipment.Unavailable
		}
		rows = append(rows, equipment.PPReadRow{Kind: "point", Point: &point})
	}
	if q.Err() != nil {
		return nil, equipment.Unavailable
	}
	return rows, equipment.OK
}

func scanMonitoringPoint(q *sql.Rows) (equipment.DeviceMonitoringPoint, bool) {
	var bucket int64
	var sessionAt, healthAt, minVal, maxVal, lastVal sql.NullInt64
	var raw string
	var point equipment.DeviceMonitoringPoint
	err := q.Scan(
		&bucket, &point.Resolution, &sessionAt, &minVal, &maxVal, &lastVal,
		&raw, &healthAt, &point.Gap, &point.Drop, &point.Samples,
	)
	if err != nil {
		return point, false
	}
	point.At = equipment.PPTime(time.Unix(bucket, 0))
	if sessionAt.Valid {
		point.SessionAt = equipment.PPTime(time.Unix(sessionAt.Int64, 0))
	}
	if healthAt.Valid {
		point.HealthAt = equipment.PPTime(time.Unix(healthAt.Int64, 0))
	}
	point.OnlineMin = nullIntPtr(minVal)
	point.OnlineMax = nullIntPtr(maxVal)
	point.Online = nullIntPtr(lastVal)
	if raw != "" {
		var h equipment.DeviceHealth
		if json.Unmarshal([]byte(raw), &h) != nil {
			return point, false
		}
		point.Health = &h
	}
	return point, true
}

func nullIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func (s *Store) monitoringFlappingRows(
	ctx context.Context, deviceID string, edgeCut int64,
) ([]equipment.PPReadRow, equipment.Code) {
	query := "SELECT login,count(*),min(at),max(at) FROM device_session_transitions " +
		"WHERE device_id=? AND at>=? GROUP BY login ORDER BY count(*) DESC,login LIMIT 2049"
	q, err := s.unit.QueryContext(ctx, query, deviceID, edgeCut)
	if err != nil {
		return nil, equipment.Unavailable
	}
	defer q.Close()
	rows := []equipment.PPReadRow{}
	for q.Next() {
		var login string
		var n uint64
		var first, last int64
		if q.Scan(&login, &n, &first, &last) != nil {
			return nil, equipment.Unavailable
		}
		rows = append(rows, equipment.PPReadRow{
			Kind:  "flapping",
			Login: login,
			Used:  &n,
			At:    equipment.PPTime(time.Unix(first, 0)),
			Value: equipment.PPTime(time.Unix(last, 0)),
		})
	}
	if q.Err() != nil {
		return nil, equipment.Unavailable
	}
	if len(rows) > equipment.PPMaxRows {
		return nil, equipment.Limit
	}
	return rows, equipment.OK
}

type monitoringVersionEntry struct {
	id, name, version string
	at                int64
}

func (s *Store) monitoringVersionRows(
	ctx context.Context, hourCut int64,
) ([]equipment.PPReadRow, *bool, *bool, int, equipment.Code) {
	all, code := s.queryActiveDeviceVersions(ctx, hourCut)
	if code != equipment.OK {
		return nil, nil, nil, 0, code
	}
	versions, packages := map[string]bool{}, map[string]bool{}
	rows := make([]equipment.PPReadRow, 0, len(all))
	for _, r := range all {
		if r.version != "" {
			versions[r.version] = true
		}
		row, sig, ok := s.buildDeviceVersionRow(ctx, r)
		if !ok {
			return nil, nil, nil, 0, equipment.Unavailable
		}
		if sig != "" {
			packages[sig] = true
		}
		rows = append(rows, row)
	}
	versionDiv, packageDiv := computeVersionDivergence(len(all), versions, packages, rows)
	return rows, versionDiv, packageDiv, len(all), equipment.OK
}

func (s *Store) queryActiveDeviceVersions(
	ctx context.Context, hourCut int64,
) ([]monitoringVersionEntry, equipment.Code) {
	query := `SELECT d.id,d.name,p.health_json,p.health_at FROM devices d ` +
		`JOIN device_monitoring_points p ON p.device_id=d.id WHERE d.active=1 AND p.health_at>=? ` +
		`AND p.health_at=(SELECT max(q.health_at) FROM device_monitoring_points q ` +
		`WHERE q.device_id=d.id) GROUP BY d.id ORDER BY d.name LIMIT 257`
	q, err := s.unit.QueryContext(ctx, query, hourCut)
	if err != nil {
		return nil, equipment.Unavailable
	}
	defer q.Close()
	all := []monitoringVersionEntry{}
	for q.Next() {
		var r monitoringVersionEntry
		var raw string
		var h equipment.DeviceHealth
		if q.Scan(&r.id, &r.name, &raw, &r.at) != nil || json.Unmarshal([]byte(raw), &h) != nil {
			return nil, equipment.Unavailable
		}
		r.version = h.Version
		all = append(all, r)
	}
	if q.Err() != nil {
		return nil, equipment.Unavailable
	}
	if len(all) > equipment.MaxDevices {
		return nil, equipment.Limit
	}
	return all, equipment.OK
}

func (s *Store) buildDeviceVersionRow(
	ctx context.Context, r monitoringVersionEntry,
) (equipment.PPReadRow, string, bool) {
	row := equipment.PPReadRow{
		Kind:  "version",
		Name:  r.name,
		Value: r.version,
		At:    equipment.PPTime(time.Unix(r.at, 0)),
	}
	var at int64
	var state string
	err := s.unit.QueryRowContext(
		ctx, "SELECT at,state FROM device_monitoring_package_status WHERE device_id=?", r.id,
	).Scan(&at, &state)
	if err == sql.ErrNoRows {
		return row, "", true
	}
	if err != nil {
		return row, "", false
	}
	row.Event = state
	row.SourceAt = equipment.PPTime(time.Unix(at, 0))
	if state != "complete" {
		return row, "", true
	}
	sig, count, ok := s.readPackageSignature(ctx, r.id)
	if !ok {
		return row, "", false
	}
	row.Used = &count
	return row, sig, true
}

func (s *Store) readPackageSignature(
	ctx context.Context, deviceID string,
) (string, uint64, bool) {
	query := "SELECT name,version,disabled FROM device_monitoring_packages " +
		"WHERE device_id=? ORDER BY name"
	pq, err := s.unit.QueryContext(ctx, query, deviceID)
	if err != nil {
		return "", 0, false
	}
	defer pq.Close()
	signature := []string{}
	var n uint64
	for pq.Next() {
		var name, version string
		var disabled sql.NullBool
		if pq.Scan(&name, &version, &disabled) != nil {
			return "", 0, false
		}
		part := name + "\x00" + version + "\x00" +
			strconv.FormatBool(disabled.Valid) + strconv.FormatBool(disabled.Bool)
		signature = append(signature, part)
		n++
	}
	if pq.Err() != nil {
		return "", 0, false
	}
	return strings.Join(signature, "\n"), n, true
}

func computeVersionDivergence(
	total int, versions, packages map[string]bool, rows []equipment.PPReadRow,
) (*bool, *bool) {
	var versionDivergence, packageDivergence *bool
	if total >= 2 {
		different := len(versions) > 1
		versionDivergence = &different
	}
	packageSources := 0
	for _, row := range rows {
		if row.Event == "complete" {
			packageSources++
		}
	}
	if packageSources >= 2 {
		different := len(packages) > 1
		packageDivergence = &different
	}
	return versionDivergence, packageDivergence
}

func (s *Store) appendRetentionRow(
	ctx context.Context, rows []equipment.PPReadRow,
) ([]equipment.PPReadRow, equipment.Code) {
	var pruned int64
	err := s.unit.QueryRowContext(
		ctx, "SELECT pruned_at FROM device_monitoring_limits WHERE singleton=1",
	).Scan(&pruned)
	if err != nil && err != sql.ErrNoRows {
		return nil, equipment.Unavailable
	}
	if pruned > 0 {
		rows = append(rows, equipment.PPReadRow{
			Kind:  "availability",
			Name:  "retention",
			Value: "limited",
			At:    equipment.PPTime(time.Unix(pruned, 0)),
		})
	}
	return rows, equipment.OK
}

func finalizeMonitoringView(
	in equipment.PPInput,
	rows []equipment.PPReadRow,
	generationText string,
	asOf int64,
	expectedHash string,
	now time.Time,
	versionDiv, packageDiv *bool,
	compared int,
) (equipment.PPView, equipment.Code) {
	v := equipment.PPView{}
	snapshot, err := json.Marshal(rows)
	if err != nil {
		return v, equipment.Unavailable
	}
	digest := sha256.Sum256(snapshot)
	fingerprint := hex.EncodeToString(digest[:])
	if expectedHash != "" && expectedHash != fingerprint {
		return v, equipment.CursorInvalid
	}
	v.MonitoringGeneration = generationText + "~" + strconv.FormatInt(asOf, 10) + "~" + fingerprint
	v.Total = len(rows)
	if in.Offset > v.Total {
		return v, equipment.CursorInvalid
	}
	end := min(in.Offset+equipment.PageSize, v.Total)
	v.Read = &equipment.PPReadResult{
		Scope:             in.Scope,
		VersionDivergence: versionDiv,
		PackageDivergence: packageDiv,
		ComparedDevices:   compared,
		Ended:             equipment.PPTime(now),
		Rows:              append([]equipment.PPReadRow{}, rows[in.Offset:end]...),
	}
	v.NextOffset = end
	v.More = end < v.Total
	return v, equipment.OK
}
