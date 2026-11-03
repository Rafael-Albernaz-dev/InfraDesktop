package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"infradesktop/internal/modules/equipment"
	"sort"
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
func (s *Store) SaveDeviceMonitoring(
	ctx context.Context,
	company string,
	d equipment.Device,
	credentialRevision uint64,
	result *equipment.PPReadResult,
) equipment.Code {
	return s.ppWrite(ctx, company, func(tx *sql.Tx) equipment.Code {
		if code := s.verifyMonitoringSaveTarget(ctx, tx, company, d, credentialRevision); code != equipment.OK {
			return code
		}
		health, at, insightAt, code := extractMonitoringHealth(result)
		if code != equipment.OK {
			return code
		}
		if err := saveEquipmentInsights(ctx, tx, d, credentialRevision, result, health, insightAt); err != nil {
			return equipment.Unavailable
		}
		if code = persistMonitoringPointAndPackages(ctx, tx, d.ID, at, health, result); code != equipment.OK {
			return code
		}
		if err := monitoringGC(ctx, tx, at); err != nil {
			if errors.Is(err, errMonitoringQuota) {
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

func (s *Store) verifyMonitoringSaveTarget(
	ctx context.Context, tx *sql.Tx, company string, d equipment.Device, credentialRevision uint64,
) equipment.Code {
	target, c := ppTarget(ctx, tx, s.installation, company, d.ID, false)
	if c != equipment.OK {
		return c
	}
	sameConn := equipment.PPConnection(target.Device) == equipment.PPConnection(d)
	if !target.Device.Active || target.Device.Revision != d.Revision ||
		target.Credential.Metadata.Revision != credentialRevision || !sameConn {
		return equipment.Revision
	}
	return equipment.OK
}

func extractMonitoringHealth(
	result *equipment.PPReadResult,
) (*equipment.DeviceHealth, time.Time, time.Time, equipment.Code) {
	var health *equipment.DeviceHealth
	for _, r := range result.Rows {
		if r.Kind == "resource" {
			health = r.Health
			break
		}
	}
	if health == nil {
		return nil, time.Time{}, time.Time{}, equipment.Partial
	}
	stamp := health.At
	if stamp == "" {
		stamp = result.Ended
	}
	at, e := time.Parse(time.RFC3339Nano, stamp)
	if e != nil {
		return nil, time.Time{}, time.Time{}, equipment.Invalid
	}
	insightAt := at
	if result.Ended != "" {
		end, err := time.Parse(time.RFC3339Nano, result.Ended)
		if err != nil || end.Before(at) {
			return nil, time.Time{}, time.Time{}, equipment.Invalid
		}
		insightAt = end
	}
	return health, at, insightAt, equipment.OK
}

func persistMonitoringPointAndPackages(
	ctx context.Context,
	tx *sql.Tx,
	deviceID string,
	at time.Time,
	health *equipment.DeviceHealth,
	result *equipment.PPReadResult,
) equipment.Code {
	body, e := json.Marshal(health)
	if e != nil || len(body) > 2048 {
		return equipment.Limit
	}
	insQuery := "INSERT INTO device_monitoring_points (" + monitoringColumns +
		") VALUES(?,?,60,?,?,NULL,NULL,NULL,NULL,?,?,0,0,0)" + monitoringMerge
	_, e = tx.ExecContext(
		ctx, insQuery, deviceID, at.Unix()/60*60, at.Unix(), at.Unix(), string(body), at.Unix(),
	)
	if e != nil {
		return equipment.Unavailable
	}
	return persistDevicePackages(ctx, tx, deviceID, at, result)
}

func persistDevicePackages(
	ctx context.Context, tx *sql.Tx, deviceID string, at time.Time, result *equipment.PPReadResult,
) equipment.Code {
	status := result.Availability["packages"]
	if status != "complete" && status != "limited" {
		status = "unavailable"
	}
	_, err := tx.ExecContext(ctx, "DELETE FROM device_monitoring_packages WHERE device_id=?", deviceID)
	if err != nil {
		return equipment.Unavailable
	}
	if status == "complete" {
		for _, row := range result.Rows {
			if row.Kind != "package" {
				continue
			}
			_, err = tx.ExecContext(
				ctx, "INSERT INTO device_monitoring_packages VALUES(?,?,?,?)",
				deviceID, row.Name, row.Value, row.Disabled,
			)
			if err != nil {
				return equipment.Unavailable
			}
		}
	}
	statusQuery := "INSERT INTO device_monitoring_package_status VALUES(?,?,?) " +
		"ON CONFLICT(device_id) DO UPDATE SET at=excluded.at,state=excluded.state"
	if _, err = tx.ExecContext(ctx, statusQuery, deviceID, at.Unix(), status); err != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}

func (s *Store) monitoringHygieneRows(
	ctx context.Context, deviceID string,
) ([]equipment.PPReadRow, equipment.Code) {
	var raw string
	query := "SELECT o.meta_json FROM pppoe_observations o " +
		"JOIN pppoe_current c ON c.observation_id=o.id WHERE c.device_id=?"
	err := s.unit.QueryRowContext(ctx, query, deviceID).Scan(&raw)
	if err == sql.ErrNoRows {
		return []equipment.PPReadRow{
			{Kind: "availability", Name: "higiene", Value: "unavailable"},
		}, equipment.OK
	}
	if err != nil {
		return nil, equipment.Unavailable
	}
	var o equipment.PPObservation
	if json.Unmarshal([]byte(raw), &o) != nil {
		return nil, equipment.Unavailable
	}
	disabled, unknown, ok := s.countSecretHygiene(ctx, o.ID)
	if !ok {
		return nil, equipment.Unavailable
	}
	return []equipment.PPReadRow{
		{
			Kind: "hygiene",
			Name: "ONLINE com secret local desativado",
			Used: &disabled,
			Size: &unknown,
			At:   o.Provenance.Ended,
		},
		{Kind: "availability", Name: "Secrets sem sessão", Value: "unavailable"},
	}, equipment.OK
}

func (s *Store) countSecretHygiene(
	ctx context.Context, observationID string,
) (uint64, uint64, bool) {
	q, err := s.unit.QueryContext(
		ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=?", observationID,
	)
	if err != nil {
		return 0, 0, false
	}
	defer q.Close()
	var disabled, unknown uint64
	for q.Next() {
		var body string
		var r equipment.PPRow
		if q.Scan(&body) != nil || json.Unmarshal([]byte(body), &r) != nil {
			return 0, 0, false
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
	if q.Err() != nil {
		return 0, 0, false
	}
	return disabled, unknown, true
}
