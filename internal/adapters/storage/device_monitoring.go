package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"infradesktop/internal/modules/equipment"
	"sort"
	"strconv"
	"strings"
	"time"
)

var monitoringNow = time.Now

const monitoringBytesLimit int64 = 20 * 1024 * 1024
const monitoringColumns = "device_id,bucket,resolution,first_at,last_at,session_at,min_online,max_online,last_online,health_json,health_at,gap,drop_seen,samples"
const monitoringMerge = ` ON CONFLICT(device_id,resolution,bucket) DO UPDATE SET
 first_at=min(first_at,excluded.first_at), last_at=max(last_at,excluded.last_at),
 min_online=CASE WHEN excluded.min_online IS NULL THEN min_online WHEN min_online IS NULL THEN excluded.min_online ELSE min(min_online,excluded.min_online) END,
 max_online=CASE WHEN excluded.max_online IS NULL THEN max_online WHEN max_online IS NULL THEN excluded.max_online ELSE max(max_online,excluded.max_online) END,
 last_online=CASE WHEN excluded.session_at IS NOT NULL AND (session_at IS NULL OR excluded.session_at>=session_at) THEN excluded.last_online ELSE last_online END,
 session_at=CASE WHEN excluded.session_at IS NULL THEN session_at WHEN session_at IS NULL THEN excluded.session_at ELSE max(session_at,excluded.session_at) END,
 health_json=CASE WHEN excluded.health_at IS NOT NULL AND (health_at IS NULL OR excluded.health_at>=health_at) THEN excluded.health_json ELSE health_json END,
 health_at=CASE WHEN excluded.health_at IS NULL THEN health_at WHEN health_at IS NULL THEN excluded.health_at ELSE max(health_at,excluded.health_at) END,
 gap=max(gap,excluded.gap),drop_seen=max(drop_seen,excluded.drop_seen),samples=samples+excluded.samples`

var errMonitoringQuota = errors.New("monitoring logical quota unavailable")

const monitoringBytesExpression = `(SELECT COALESCE(sum(length(CAST(health_json AS BLOB))+length(device_id)+128),0) FROM device_monitoring_points)+
 (SELECT COALESCE(sum(length(CAST(login AS BLOB))+length(device_id)+length(observation_id)+32),0) FROM device_session_transitions)+
 (SELECT COALESCE(sum(length(device_id)+length(observation_id)+8),0) FROM device_monitoring_checkpoint)+
 (SELECT COALESCE(sum(length(device_id)+length(CAST(name AS BLOB))+length(CAST(version AS BLOB))+16),0) FROM device_monitoring_packages)+
 (SELECT COALESCE(sum(length(device_id)+24),0) FROM device_monitoring_package_status)+
 (SELECT COALESCE(sum(length(CAST(health_json AS BLOB))+length(CAST(ports_json AS BLOB))+length(connection)+length(device_id)+length(stamp)+64),0) FROM device_equipment_snapshot)+
 (SELECT COALESCE(sum(length(CAST(row_json AS BLOB))+length(device_id)+length(name)+length(stamp)+32),0) FROM device_equipment_events)+
 (SELECT COALESCE(sum(length(device_id)+length(observation_id)+length(CAST(service AS BLOB))+length(context)+32),0) FROM device_context_samples)`

func monitoringBytes(ctx context.Context, tx *sql.Tx) (int64, error) {
	var n int64
	e := tx.QueryRowContext(ctx, "SELECT "+monitoringBytesExpression).Scan(&n)
	return n, e
}
func bumpMonitoring(ctx context.Context, tx *sql.Tx) error {
	_, e := tx.ExecContext(ctx, "UPDATE device_monitoring_generation SET generation=generation+1 WHERE singleton=1")
	return e
}
func trimMonitoring(ctx context.Context, tx *sql.Tx, totalLimit int64, now int64) error {
	ops, e := monitoringBytes(ctx, tx)
	if e != nil {
		return e
	}
	total, e := ppTotalBytes(ctx, tx)
	if e != nil {
		return e
	}
	need := max(ops-monitoringBytesLimit, total-totalLimit)
	if need <= 0 {
		return nil
	}
	// Account and order once per transaction. Re-scanning the entire series for
	// each removed point would let auxiliary history exhaust the base write's3s.
	rows, e := tx.QueryContext(ctx, `SELECT kind,device_id,stamp,resolution,observation_id,login,bytes FROM (
 SELECT 'point' AS kind,device_id,bucket AS stamp,resolution,'' AS observation_id,'' AS login,
 length(CAST(health_json AS BLOB))+length(device_id)+128 AS bytes FROM device_monitoring_points
 UNION ALL SELECT 'edge',device_id,at,0,observation_id,login,
 length(CAST(login AS BLOB))+length(device_id)+length(observation_id)+32 FROM device_session_transitions
 UNION ALL SELECT 'packages',s.device_id,s.at,0,'','',length(s.device_id)+24+
 COALESCE((SELECT sum(length(p.device_id)+length(CAST(p.name AS BLOB))+length(CAST(p.version AS BLOB))+16)
 FROM device_monitoring_packages p WHERE p.device_id=s.device_id),0) FROM device_monitoring_package_status s
 UNION ALL SELECT 'checkpoint',device_id,9223372036854775807,0,observation_id,'',length(device_id)+length(observation_id)+8 FROM device_monitoring_checkpoint
 UNION ALL SELECT 'equipment_snapshot',device_id,at,0,'','',length(CAST(health_json AS BLOB))+length(CAST(ports_json AS BLOB))+length(connection)+length(device_id)+length(stamp)+64 FROM device_equipment_snapshot
 UNION ALL SELECT 'equipment_event',device_id,at,0,stamp,name,length(CAST(row_json AS BLOB))+length(device_id)+length(name)+length(stamp)+32 FROM device_equipment_events
 UNION ALL SELECT 'context_sample',device_id,at,vlan,observation_id,service,length(device_id)+length(observation_id)+length(CAST(service AS BLOB))+length(context)+32 FROM device_context_samples
 ) ORDER BY stamp,kind,device_id,resolution,observation_id,login`)
	if e != nil {
		return e
	}
	type candidate struct {
		kind, device, observation, login string
		bucket, resolution, bytes        int64
	}
	var selected []candidate
	var reclaimed int64
	for rows.Next() {
		var c candidate
		if e = rows.Scan(&c.kind, &c.device, &c.bucket, &c.resolution, &c.observation, &c.login, &c.bytes); e != nil {
			rows.Close()
			return e
		}
		selected = append(selected, c)
		reclaimed += c.bytes
		if reclaimed >= need {
			break
		}
	}
	e = rows.Err()
	closeErr := rows.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	for _, c := range selected {
		switch c.kind {
		case "context_sample":
			_, e = tx.ExecContext(ctx, "DELETE FROM device_context_samples WHERE device_id=? AND observation_id=? AND service=? AND vlan=?", c.device, c.observation, c.login, c.resolution)
		case "equipment_snapshot":
			_, e = tx.ExecContext(ctx, "DELETE FROM device_equipment_snapshot WHERE device_id=?", c.device)
		case "equipment_event":
			_, e = tx.ExecContext(ctx, "DELETE FROM device_equipment_events WHERE device_id=? AND stamp=? AND name=?", c.device, c.observation, c.login)
		case "point":
			_, e = tx.ExecContext(ctx, "DELETE FROM device_monitoring_points WHERE device_id=? AND bucket=? AND resolution=?", c.device, c.bucket, c.resolution)
		case "edge":
			_, e = tx.ExecContext(ctx, "DELETE FROM device_session_transitions WHERE device_id=? AND observation_id=? AND login=?", c.device, c.observation, c.login)
		case "packages":
			if _, e = tx.ExecContext(ctx, "DELETE FROM device_monitoring_packages WHERE device_id=?", c.device); e == nil {
				_, e = tx.ExecContext(ctx, "DELETE FROM device_monitoring_package_status WHERE device_id=?", c.device)
			}
		case "checkpoint":
			_, e = tx.ExecContext(ctx, "DELETE FROM device_monitoring_checkpoint WHERE device_id=?", c.device)
		}
		if e != nil {
			return e
		}
	}
	if len(selected) > 0 {
		if e = bumpMonitoring(ctx, tx); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO device_monitoring_limits VALUES(1,?) ON CONFLICT(singleton) DO UPDATE SET pruned_at=excluded.pruned_at", now); e != nil {
			return e
		}
	}
	// Reconcile actual accounting once, including package groups and metadata.
	ops, e = monitoringBytes(ctx, tx)
	if e != nil {
		return e
	}
	total, e = ppTotalBytes(ctx, tx)
	if e != nil {
		return e
	}
	if ops > monitoringBytesLimit || total > totalLimit {
		return errMonitoringQuota
	}
	return nil
}
func monitoringGC(ctx context.Context, tx *sql.Tx, at time.Time) error {
	minuteCut, hourCut, edgeCut := equipment.MonitoringRetention(at)
	// Select the last session and health by their own timestamps, never relabel
	// an old online count with a newer health-only read.
	_, e := tx.ExecContext(ctx, `INSERT INTO device_monitoring_points (`+monitoringColumns+`)
 SELECT p.device_id,(p.bucket/3600)*3600,3600,min(p.first_at),max(p.last_at),max(p.session_at),min(p.min_online),max(p.max_online),
 (SELECT q.last_online FROM device_monitoring_points q WHERE q.device_id=p.device_id AND q.resolution=60 AND q.bucket<? AND q.bucket/3600=p.bucket/3600 AND q.session_at IS NOT NULL ORDER BY q.session_at DESC,q.bucket DESC LIMIT 1),
 COALESCE((SELECT q.health_json FROM device_monitoring_points q WHERE q.device_id=p.device_id AND q.resolution=60 AND q.bucket<? AND q.bucket/3600=p.bucket/3600 AND q.health_at IS NOT NULL ORDER BY q.health_at DESC,q.bucket DESC LIMIT 1),''),
 max(p.health_at),max(p.gap),max(p.drop_seen),sum(p.samples)
 FROM device_monitoring_points p WHERE p.resolution=60 AND p.bucket<? GROUP BY p.device_id,p.bucket/3600 `+monitoringMerge, minuteCut, minuteCut, minuteCut)
	if e != nil {
		return e
	}
	for _, q := range []struct {
		sql string
		cut int64
	}{
		{"DELETE FROM device_monitoring_points WHERE resolution=60 AND bucket<?", minuteCut},
		{"DELETE FROM device_monitoring_points WHERE bucket<?", hourCut},
		{"DELETE FROM device_session_transitions WHERE at<?", edgeCut},
		{"DELETE FROM device_equipment_events WHERE at<?", at.Add(-30 * 24 * time.Hour).Unix()},
		{"DELETE FROM device_equipment_snapshot WHERE at<?", at.Add(-30 * 24 * time.Hour).Unix()},
		{"DELETE FROM device_context_samples WHERE at<?", at.Add(-7 * 24 * time.Hour).Unix()},
	} {
		if _, e = tx.ExecContext(ctx, q.sql, q.cut); e != nil {
			return e
		}
	}
	return trimMonitoring(ctx, tx, equipment.PPCompanyBytes, at.Unix())
}
func recordSessionMonitoring(ctx context.Context, tx *sql.Tx, previous, next *equipment.PPObservation) error {
	var known string
	var valid bool
	e := tx.QueryRowContext(ctx, "SELECT observation_id,continuous FROM device_monitoring_checkpoint WHERE device_id=?", next.DeviceID).Scan(&known, &valid)
	if e != nil && e != sql.ErrNoRows {
		return e
	}
	if known == next.ID {
		return nil
	}
	if e := recordContextGroups(ctx, tx, next); e != nil {
		return e
	}
	at, e := time.Parse(time.RFC3339Nano, next.Provenance.Ended)
	if e != nil {
		return e
	}
	continuous := valid && previous != nil && known == previous.ID && equipment.PPMonitoringContinuous(previous, next)
	drop := continuous && equipment.PPMonitoringDrop(previous.Online, next.Online)
	states := func(rows []equipment.PPRow) map[string]string {
		counts := map[string]int{}
		out := map[string]string{}
		for _, r := range rows {
			if r.Login != "" {
				counts[r.Login]++
				out[r.Login] = r.State
			}
		}
		for login, n := range counts {
			if n != 1 {
				delete(out, login)
			}
		}
		return out
	}
	if continuous {
		before, after := states(previous.Rows), states(next.Rows)
		keys := make([]string, 0, len(after))
		for login := range after {
			keys = append(keys, login)
		}
		sort.Strings(keys)
		for _, login := range keys {
			old, exists := before[login]
			now := after[login]
			if exists && old != now && (old == "ONLINE" || old == "OFFLINE") && (now == "ONLINE" || now == "OFFLINE") {
				if _, e = tx.ExecContext(ctx, "INSERT INTO device_session_transitions VALUES(?,?,?,?,?,?)", next.DeviceID, next.ID, login, at.Unix(), old, now); e != nil {
					return e
				}
			}
		}
	}
	_, e = tx.ExecContext(ctx, "INSERT INTO device_monitoring_points ("+monitoringColumns+") VALUES(?,?,60,?,?,?,?,?,?,'',NULL,?,?,1)"+monitoringMerge, next.DeviceID, at.Unix()/60*60, at.Unix(), at.Unix(), at.Unix(), next.Online, next.Online, next.Online, !continuous, drop)
	if e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, "INSERT INTO device_monitoring_checkpoint VALUES(?,?,1) ON CONFLICT(device_id) DO UPDATE SET observation_id=excluded.observation_id,continuous=1", next.DeviceID, next.ID); e != nil {
		return e
	}
	if e = monitoringGC(ctx, tx, at); e != nil {
		return e
	}
	return bumpMonitoring(ctx, tx)
}
func (s *Store) SaveDeviceMonitoring(ctx context.Context, company string, d equipment.Device, credentialRevision uint64, result *equipment.PPReadResult) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		target, c := ppTarget(ctx, tx, s.installation, company, d.ID, false)
		if c != equipment.OK {
			return c
		}
		if !target.Device.Active || target.Device.Revision != d.Revision || target.Credential.Metadata.Revision != credentialRevision || equipment.PPConnection(target.Device) != equipment.PPConnection(d) {
			return equipment.Revision
		}
		var health *equipment.DeviceHealth
		for _, r := range result.Rows {
			if r.Kind == "resource" {
				health = r.Health
				break
			}
		}
		if health == nil {
			return equipment.Partial
		}
		stamp := health.At
		if stamp == "" {
			stamp = result.Ended
		}
		at, e := time.Parse(time.RFC3339Nano, stamp)
		if e != nil {
			return equipment.Invalid
		}
		insightAt := at
		if result.Ended != "" {
			end, err := time.Parse(time.RFC3339Nano, result.Ended)
			if err != nil || end.Before(at) {
				return equipment.Invalid
			}
			insightAt = end
		}
		if e = saveEquipmentInsights(ctx, tx, d, credentialRevision, result, health, insightAt); e != nil {
			return equipment.Unavailable
		}
		body, e := json.Marshal(health)
		if e != nil || len(body) > 2048 {
			return equipment.Limit
		}
		_, e = tx.ExecContext(ctx, "INSERT INTO device_monitoring_points ("+monitoringColumns+") VALUES(?,?,60,?,?,NULL,NULL,NULL,NULL,?,?,0,0,0)"+monitoringMerge, d.ID, at.Unix()/60*60, at.Unix(), at.Unix(), string(body), at.Unix())
		if e != nil {
			return equipment.Unavailable
		}
		status := result.Availability["packages"]
		if status != "complete" && status != "limited" {
			status = "unavailable"
		}
		if _, e = tx.ExecContext(ctx, "DELETE FROM device_monitoring_packages WHERE device_id=?", d.ID); e != nil {
			return equipment.Unavailable
		}
		if status == "complete" {
			for _, row := range result.Rows {
				if row.Kind != "package" {
					continue
				}
				if _, e = tx.ExecContext(ctx, "INSERT INTO device_monitoring_packages VALUES(?,?,?,?)", d.ID, row.Name, row.Value, row.Disabled); e != nil {
					return equipment.Unavailable
				}
			}
		}
		if _, e = tx.ExecContext(ctx, "INSERT INTO device_monitoring_package_status VALUES(?,?,?) ON CONFLICT(device_id) DO UPDATE SET at=excluded.at,state=excluded.state", d.ID, at.Unix(), status); e != nil {
			return equipment.Unavailable
		}
		if e = monitoringGC(ctx, tx, at); e != nil {
			if errors.Is(e, errMonitoringQuota) {
				return equipment.Limit
			}
			return equipment.Unavailable
		}
		if bumpMonitoring(ctx, tx) != nil {
			return equipment.Unavailable
		}
		return equipment.OK
	})
}
func (s *Store) DeviceMonitoring(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	v := equipment.PPView{}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return v, equipment.Changed
	}
	if s.checkActive() != nil {
		return v, equipment.Unavailable
	}
	var active bool
	if e := s.unit.QueryRowContext(ctx, "SELECT active FROM devices WHERE id=?", in.DeviceID).Scan(&active); e != nil {
		return v, equipment.Invalid
	}
	var generation int64
	if e := s.unit.QueryRowContext(ctx, "SELECT generation FROM device_monitoring_generation WHERE singleton=1").Scan(&generation); e != nil {
		return v, equipment.Unavailable
	}
	now := monitoringNow().UTC()
	asOf := now.Unix()
	expectedHash := ""
	generationText := strconv.FormatInt(generation, 10)
	if in.MonitoringGeneration != "" {
		fields := strings.Split(in.MonitoringGeneration, "~")
		if len(fields) != 3 || fields[0] != generationText {
			return v, equipment.CursorInvalid
		}
		var e error
		asOf, e = strconv.ParseInt(fields[1], 10, 64)
		if e != nil || asOf > now.Unix() || now.Unix()-asOf > int64(equipment.JobRetention/time.Second) {
			return v, equipment.CursorInvalid
		}
		expectedHash = fields[2]
	}
	now = time.Unix(asOf, 0).UTC()
	var versionDivergence, packageDivergence *bool
	compared := 0
	rows := []equipment.PPReadRow{}
	minuteCut, hourCut, edgeCut := equipment.MonitoringRetention(now)
	_ = minuteCut
	switch in.Scope {
	case "duplicates":
		var e error
		rows, e = crossConcentratorRows(ctx, s.unit, in.DeviceID, now)
		if e != nil {
			return v, equipment.Unavailable
		}
		if len(rows) > equipment.PPMaxRows {
			return v, equipment.Limit
		}
	case "equipment", "port-history":
		var e error
		rows, e = readEquipmentInsights(ctx, s.unit, in.DeviceID, in.Scope, now)
		if e != nil {
			return v, equipment.Unavailable
		}
		if len(rows) > equipment.PPMaxRows {
			return v, equipment.Limit
		}
	case "series":
		q, e := s.unit.QueryContext(ctx, `SELECT bucket,resolution,session_at,min_online,max_online,last_online,health_json,health_at,gap,drop_seen,samples FROM device_monitoring_points WHERE device_id=? AND bucket>=? ORDER BY bucket DESC,resolution`, in.DeviceID, hourCut)
		if e != nil {
			return v, equipment.Unavailable
		}
		for q.Next() {
			var bucket int64
			var sessionAt, healthAt sql.NullInt64
			var min, max, last sql.NullInt64
			var raw string
			var point equipment.DeviceMonitoringPoint
			if e = q.Scan(&bucket, &point.Resolution, &sessionAt, &min, &max, &last, &raw, &healthAt, &point.Gap, &point.Drop, &point.Samples); e != nil {
				q.Close()
				return v, equipment.Unavailable
			}
			point.At = equipment.PPTime(time.Unix(bucket, 0))
			if sessionAt.Valid {
				point.SessionAt = equipment.PPTime(time.Unix(sessionAt.Int64, 0))
			}
			if healthAt.Valid {
				point.HealthAt = equipment.PPTime(time.Unix(healthAt.Int64, 0))
			}
			ptr := func(n sql.NullInt64) *int {
				if !n.Valid {
					return nil
				}
				v := int(n.Int64)
				return &v
			}
			point.OnlineMin, point.OnlineMax, point.Online = ptr(min), ptr(max), ptr(last)
			if raw != "" {
				var h equipment.DeviceHealth
				if json.Unmarshal([]byte(raw), &h) != nil {
					q.Close()
					return v, equipment.Unavailable
				}
				point.Health = &h
			}
			rows = append(rows, equipment.PPReadRow{Kind: "point", Point: &point})
		}
		e = q.Err()
		q.Close()
		if e != nil {
			return v, equipment.Unavailable
		}
	case "flapping":
		q, e := s.unit.QueryContext(ctx, "SELECT login,count(*),min(at),max(at) FROM device_session_transitions WHERE device_id=? AND at>=? GROUP BY login ORDER BY count(*) DESC,login LIMIT 2049", in.DeviceID, edgeCut)
		if e != nil {
			return v, equipment.Unavailable
		}
		for q.Next() {
			var login string
			var n uint64
			var first, last int64
			if q.Scan(&login, &n, &first, &last) != nil {
				q.Close()
				return v, equipment.Unavailable
			}
			rows = append(rows, equipment.PPReadRow{Kind: "flapping", Login: login, Used: &n, At: equipment.PPTime(time.Unix(first, 0)), Value: equipment.PPTime(time.Unix(last, 0))})
		}
		e = q.Err()
		q.Close()
		if e != nil {
			return v, equipment.Unavailable
		}
	case "versions":
		q, e := s.unit.QueryContext(ctx, `SELECT d.id,d.name,p.health_json,p.health_at FROM devices d JOIN device_monitoring_points p ON p.device_id=d.id WHERE d.active=1 AND p.health_at>=? AND p.health_at=(SELECT max(q.health_at) FROM device_monitoring_points q WHERE q.device_id=d.id) GROUP BY d.id ORDER BY d.name LIMIT 257`, hourCut)
		if e != nil {
			return v, equipment.Unavailable
		}
		type versionRow struct {
			id, name, version string
			at                int64
		}
		all := []versionRow{}
		for q.Next() {
			var r versionRow
			var raw string
			var h equipment.DeviceHealth
			if q.Scan(&r.id, &r.name, &raw, &r.at) != nil || json.Unmarshal([]byte(raw), &h) != nil {
				q.Close()
				return v, equipment.Unavailable
			}
			r.version = h.Version
			all = append(all, r)
		}
		e = q.Err()
		q.Close()
		if e != nil {
			return v, equipment.Unavailable
		}
		if len(all) > equipment.MaxDevices {
			return v, equipment.Limit
		}
		versions, packages := map[string]bool{}, map[string]bool{}
		for _, r := range all {
			if r.version != "" {
				versions[r.version] = true
			}
			row := equipment.PPReadRow{Kind: "version", Name: r.name, Value: r.version, At: equipment.PPTime(time.Unix(r.at, 0))}
			var at int64
			var state string
			e = s.unit.QueryRowContext(ctx, "SELECT at,state FROM device_monitoring_package_status WHERE device_id=?", r.id).Scan(&at, &state)
			if e == nil {
				row.Event = state
				row.SourceAt = equipment.PPTime(time.Unix(at, 0))
				if state == "complete" {
					pq, e := s.unit.QueryContext(ctx, "SELECT name,version,disabled FROM device_monitoring_packages WHERE device_id=? ORDER BY name", r.id)
					if e != nil {
						return v, equipment.Unavailable
					}
					signature := []string{}
					var n uint64
					for pq.Next() {
						var name, version string
						var disabled sql.NullBool
						if pq.Scan(&name, &version, &disabled) != nil {
							pq.Close()
							return v, equipment.Unavailable
						}
						signature = append(signature, name+"\x00"+version+"\x00"+strconv.FormatBool(disabled.Valid)+strconv.FormatBool(disabled.Bool))
						n++
					}
					e = pq.Err()
					pq.Close()
					if e != nil {
						return v, equipment.Unavailable
					}
					packages[strings.Join(signature, "\n")] = true
					row.Used = &n
				}
			} else if e != sql.ErrNoRows {
				return v, equipment.Unavailable
			}
			rows = append(rows, row)
		}
		compared = len(all)
		if len(all) >= 2 {
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
	case "hygiene":
		var raw string
		e := s.unit.QueryRowContext(ctx, "SELECT o.meta_json FROM pppoe_observations o JOIN pppoe_current c ON c.observation_id=o.id WHERE c.device_id=?", in.DeviceID).Scan(&raw)
		if e == sql.ErrNoRows {
			rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "higiene", Value: "unavailable"})
			break
		}
		if e != nil {
			return v, equipment.Unavailable
		}
		var o equipment.PPObservation
		if json.Unmarshal([]byte(raw), &o) != nil {
			return v, equipment.Unavailable
		}
		q, e := s.unit.QueryContext(ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=?", o.ID)
		if e != nil {
			return v, equipment.Unavailable
		}
		var disabled, unknown uint64
		for q.Next() {
			var body string
			var r equipment.PPRow
			if q.Scan(&body) != nil || json.Unmarshal([]byte(body), &r) != nil {
				q.Close()
				return v, equipment.Unavailable
			}
			if r.State == "OFFLINE" {
				continue
			}
			if r.Context == nil || r.Context.SecretState == "" || r.Context.SecretState == "unavailable" {
				unknown++
			} else if r.Context.SecretState == "disabled" {
				disabled++
			}
		}
		e = q.Err()
		q.Close()
		if e != nil {
			return v, equipment.Unavailable
		}
		rows = append(rows, equipment.PPReadRow{Kind: "hygiene", Name: "ONLINE com secret local desativado", Used: &disabled, Size: &unknown, At: o.Provenance.Ended})
		rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "Secrets sem sessão", Value: "unavailable"})
	default:
		return v, equipment.Invalid
	}
	if in.Scope == "flapping" && len(rows) > equipment.PPMaxRows {
		return v, equipment.Limit
	}
	var pruned int64
	e := s.unit.QueryRowContext(ctx, "SELECT pruned_at FROM device_monitoring_limits WHERE singleton=1").Scan(&pruned)
	if e != nil && e != sql.ErrNoRows {
		return v, equipment.Unavailable
	}
	if pruned > 0 {
		rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "retention", Value: "limited", At: equipment.PPTime(time.Unix(pruned, 0))})
	}
	// Cursor fixes both the temporal cutoff and the complete relevant set.
	// This also detects inventory rename/active changes that reorder versions.
	snapshot, e := json.Marshal(rows)
	if e != nil {
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
	v.Read = &equipment.PPReadResult{Scope: in.Scope, VersionDivergence: versionDivergence, PackageDivergence: packageDivergence, ComparedDevices: compared, Ended: equipment.PPTime(now), Rows: append([]equipment.PPReadRow{}, rows[in.Offset:end]...)}
	v.NextOffset = end
	v.More = end < v.Total
	return v, equipment.OK
}
