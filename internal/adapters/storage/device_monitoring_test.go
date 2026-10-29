package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
	"time"
)

func monitoringObservation(target equipment.PPTarget, at time.Time, id string, n int) *equipment.PPObservation {
	elapsed := at.Sub(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)).Nanoseconds()
	p := equipment.PPProvenance{Installation: target.Installation, Company: companyX, DeviceID: target.Device.ID, Revision: target.Device.Revision, CredentialRevision: target.Credential.Metadata.Revision, Connection: equipment.PPConnection(target.Device), Epoch: "monitoring-test", Projection: "pppoe-active-v1", Codec: "1", Capabilities: "1", Family: "v7", Version: "7.14.3", Started: equipment.PPTime(at), Ended: equipment.PPTime(at.Add(time.Millisecond)), Sample: equipment.PPInterval{Known: true, Min: elapsed, Max: elapsed + int64(time.Millisecond)}, RouterUptime: equipment.PPInterval{Known: true, Min: elapsed + int64(time.Hour), Max: elapsed + int64(time.Hour+time.Millisecond)}}
	o := &equipment.PPObservation{ID: id, DeviceID: target.Device.ID, Scope: equipment.PPScope, Provenance: p, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Online: n}
	return o
}
func TestL06T2SeriesTransitionsFailureReplayAndReopen(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Minute)
	previous := monitoringObservation(target, at, uuid.NewString(), 10)
	previous.Rows = []equipment.PPRow{{Login: "synthetic", State: "ONLINE"}}
	next := monitoringObservation(target, at.Add(2*time.Second), uuid.NewString(), 4)
	next.Rows = []equipment.PPRow{{Login: "synthetic", State: "OFFLINE"}}
	publish := func(old, o *equipment.PPObservation) {
		t.Helper()
		if c := j.st.ppWrite(ctx, companyX, func(tx *sql.Tx) equipment.Code {
			if e := recordSessionMonitoring(ctx, tx, old, o); e != nil {
				t.Error(e)
				return equipment.Unavailable
			}
			return equipment.OK
		}); c != equipment.OK {
			t.Fatal(c)
		}
	}
	publish(nil, previous)
	publish(previous, next)
	publish(previous, next)
	var transitions, points int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_session_transitions").Scan(&transitions); e != nil || transitions != 1 {
		t.Fatal("replay duplicated or lost transition", transitions, e)
	}
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_monitoring_points").Scan(&points); e != nil || points != 1 {
		t.Fatal("more than one point/min", points, e)
	}
	var min, max, last int
	var drop bool
	if e := j.st.unit.QueryRow("SELECT min_online,max_online,last_online,drop_seen FROM device_monitoring_points").Scan(&min, &max, &last, &drop); e != nil || min != 4 || max != 10 || last != 4 || !drop {
		t.Fatal("bucket lost min/max/drop", min, max, last, drop, e)
	}
	if _, e := j.st.unit.Exec("UPDATE device_monitoring_checkpoint SET continuous=0"); e != nil {
		t.Fatal(e)
	}
	after := monitoringObservation(target, at.Add(4*time.Second), uuid.NewString(), 10)
	after.Rows = []equipment.PPRow{{Login: "synthetic", State: "ONLINE"}}
	publish(next, after)
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_session_transitions").Scan(&transitions); e != nil || transitions != 1 {
		t.Fatal("failed interval counted as flap", transitions, e)
	}
	reboot := monitoringObservation(target, at.Add(6*time.Second), uuid.NewString(), 1)
	reboot.Provenance.RouterUptime = equipment.PPInterval{Known: true, Min: 1, Max: 2}
	reboot.Rows = []equipment.PPRow{{Login: "synthetic", State: "OFFLINE"}}
	publish(after, reboot)
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_session_transitions").Scan(&transitions); e != nil || transitions != 1 {
		t.Fatal("reboot counted as flap", transitions, e)
	}
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyY, access.OK)
	j.selectCompany(companyX, access.OK)
	v, c := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "series"})
	if c != equipment.OK || len(v.Read.Rows) != 1 || v.Read.Rows[0].Point.Online == nil || *v.Read.Rows[0].Point.Online != 1 {
		t.Fatal("reopen lost series", v, c)
	}
}
func TestL06T2RetentionAndBaseQuotaPriority(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	c := j.st.ppWrite(ctx, companyX, func(tx *sql.Tx) equipment.Code {
		for _, age := range []time.Duration{8 * 24 * time.Hour, 25 * time.Hour, 25*time.Hour - time.Minute, time.Minute} {
			at := now.Add(-age).Unix()
			n := 10
			if age == 25*time.Hour-time.Minute {
				n = 3
			}
			if _, e := tx.Exec("INSERT INTO device_monitoring_points ("+monitoringColumns+") VALUES(?,?,60,?,?,?,?,?,?,'',NULL,0,0,1)", target.Device.ID, at/60*60, at, at, at, n, n, n); e != nil {
				t.Error(e)
				return equipment.Unavailable
			}
		}
		if e := monitoringGC(ctx, tx, now); e != nil {
			t.Error(e)
			return equipment.Unavailable
		}
		return equipment.OK
	})
	if c != equipment.OK {
		t.Fatal(c)
	}
	var count, min, max int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_monitoring_points").Scan(&count); e != nil || count != 2 {
		t.Fatal("retention/downsample count", count, e)
	}
	if e := j.st.unit.QueryRow("SELECT min_online,max_online FROM device_monitoring_points WHERE resolution=3600").Scan(&min, &max); e != nil || min != 3 || max != 10 {
		t.Fatal("hour range lost", min, max, e)
	}
	// Fill the logical A budget without allocating160MiB or changing protected
	// observations. Only disposable synthetic accounting is used for this oracle.
	base := ppSynthetic(t, j, target, 1)
	c = j.st.ppWrite(ctx, companyX, func(tx *sql.Tx) equipment.Code {
		if _, e := tx.Exec("UPDATE pppoe_observations SET payload_bytes=? WHERE id=?", equipment.PPObservationBytes, base.ID); e != nil {
			t.Error(e)
			return equipment.Unavailable
		}
		for i := 1; i < equipment.PPCompanyBytes/equipment.PPObservationBytes; i++ {
			if _, e := tx.Exec("INSERT INTO pppoe_observations SELECT ?,device_id,scope,meta_json,meta_bytes,row_count,? FROM pppoe_observations WHERE id=?", uuid.NewString(), equipment.PPObservationBytes, base.ID); e != nil {
				t.Error(e)
				return equipment.Unavailable
			}
		}
		if e := trimMonitoring(ctx, tx, equipment.PPCompanyBytes, now.Unix()); e != nil {
			t.Error(e)
			return equipment.Unavailable
		}
		total, e := ppTotalBytes(ctx, tx)
		if e != nil || total != equipment.PPCompanyBytes {
			t.Error("base priority or checkpoint overflow", total, e)
			return equipment.Unavailable
		}
		if e := trimMonitoring(ctx, tx, equipment.PPCompanyBytes-1, now.Unix()); !errors.Is(e, errMonitoringQuota) {
			t.Error("unreclaimable quota silently succeeded", e)
			return equipment.Unavailable
		}
		return equipment.OK
	})
	if c != equipment.OK {
		t.Fatal(c)
	}
	var metadata int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_monitoring_checkpoint").Scan(&metadata); e != nil || metadata != 0 {
		t.Fatal("auxiliary checkpoint prevented exact base budget", metadata, e)
	}
}
func TestL06T2MigrationNineAtomicAndFirstEightUnchanged(t *testing.T) {
	j, _ := ppStoreSetup(t)
	original := companyDeviceMonitoringSchema
	defer func() { companyDeviceMonitoringSchema = original }()
	var before string
	if e := j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM schema_migrations WHERE version<=8 ORDER BY version").Scan(&before); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"device_context_samples", "device_equipment_snapshot", "device_equipment_events", "device_monitoring_points", "device_session_transitions", "device_monitoring_checkpoint", "device_monitoring_limits", "device_monitoring_generation", "device_monitoring_packages", "device_monitoring_package_status"} {
		if _, e := j.st.unit.Exec("DROP TABLE " + table); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version>=9"); e != nil {
		t.Fatal(e)
	}
	companyDeviceMonitoringSchema = original + "; invalid SQL;"
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e == nil {
		t.Fatal("bad migration accepted")
	}
	var n int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='device_monitoring_points'").Scan(&n); e != nil || n != 0 {
		t.Fatal("partial migration escaped transaction", n, e)
	}
	companyDeviceMonitoringSchema = original
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	var after string
	if e := j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM schema_migrations WHERE version<=8 ORDER BY version").Scan(&after); e != nil || before != after {
		t.Fatal("applied migration changed", e)
	}
}
func TestL06T2HealthSourceAndPackageProjectionPersist(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	at := time.Now().UTC().Add(-10 * time.Second)
	disabled := false
	result := &equipment.PPReadResult{Scope: "operations", Ended: equipment.PPTime(at.Add(5 * time.Second)), Availability: map[string]string{"packages": "complete"}, Rows: []equipment.PPReadRow{{Kind: "resource", Health: &equipment.DeviceHealth{At: equipment.PPTime(at), Version: "7.14.3"}}, {Kind: "package", Name: "fixture-package", Value: "7.14.3", Disabled: &disabled}}}
	if c := j.st.SaveDeviceMonitoring(ctx, companyX, target.Device, target.Credential.Metadata.Revision, result); c != equipment.OK {
		t.Fatal(c)
	}
	v, c := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "series"})
	if c != equipment.OK || len(v.Read.Rows) != 1 || v.Read.Rows[0].Point.HealthAt != equipment.PPTime(time.Unix(at.Unix(), 0)) {
		b, _ := json.Marshal(v)
		t.Fatal("health relabeled with later auth timestamp", string(b), c)
	}
	v, c = j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "versions"})
	if c != equipment.OK || len(v.Read.Rows) != 1 || v.Read.Rows[0].Used == nil || *v.Read.Rows[0].Used != 1 || v.Read.Rows[0].Event != "complete" {
		t.Fatal("package projection missing", v, c)
	}
}

func TestL06T2GlobalPackageDivergenceIncludesOtherPage(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	at := time.Now().UTC()
	disabled := false
	for i := 0; i < 17; i++ {
		d := target.Device
		if i > 0 {
			d.ID = uuid.NewString()
			if _, e := j.st.unit.Exec("INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation) SELECT ?,?,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation FROM devices WHERE id=?", d.ID, fmt.Sprintf("router-%02d", i), target.Device.ID); e != nil {
				t.Fatal(e)
			}
		}
		version := "1.0"
		if i == 16 {
			version = "2.0"
		}
		result := &equipment.PPReadResult{Scope: "operations", Ended: equipment.PPTime(at), Availability: map[string]string{"packages": "complete"}, Rows: []equipment.PPReadRow{{Kind: "resource", Health: &equipment.DeviceHealth{At: equipment.PPTime(at), Version: "7.14.3"}}, {Kind: "package", Name: "fixture-package", Value: version, Disabled: &disabled}}}
		if c := j.st.SaveDeviceMonitoring(ctx, companyX, d, target.Credential.Metadata.Revision, result); c != equipment.OK {
			t.Fatal(c)
		}
	}
	v, c := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "versions"})
	if c != equipment.OK || len(v.Read.Rows) != 16 || !v.More || v.Read.ComparedDevices != 17 || v.Read.VersionDivergence == nil || *v.Read.VersionDivergence || v.Read.PackageDivergence == nil || !*v.Read.PackageDivergence {
		t.Fatal("comparison only covered visible page", v, c)
	}
	for _, r := range v.Read.Rows {
		if r.Name == "router-16" {
			t.Fatal("fixture did not put different package on another page")
		}
	}
	snapshot := equipment.PPInput{DeviceID: target.Device.ID, Scope: "versions", Offset: 16, MonitoringGeneration: v.MonitoringGeneration}
	if _, c := j.st.DeviceMonitoring(ctx, companyX, snapshot); c != equipment.OK {
		t.Fatal("legitimate next version page rejected", c)
	}
	if _, e := j.st.unit.Exec("UPDATE devices SET name='first-renamed' WHERE name='router-16'"); e != nil {
		t.Fatal(e)
	}
	if _, c := j.st.DeviceMonitoring(ctx, companyX, snapshot); c != equipment.CursorInvalid {
		t.Fatal("inventory rename shifted cursor silently", c)
	}
	if _, e := j.st.unit.Exec("UPDATE devices SET name='router-16' WHERE name='first-renamed'"); e != nil {
		t.Fatal(e)
	}
	if _, e := j.st.unit.Exec("UPDATE devices SET active=0 WHERE name='router-16'"); e != nil {
		t.Fatal(e)
	}
	if _, c := j.st.DeviceMonitoring(ctx, companyX, snapshot); c != equipment.CursorInvalid {
		t.Fatal("inactive inventory shifted cursor silently", c)
	}

}
func TestL06T2CursorCoversInventoryAndTemporalWindow(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	original := monitoringNow
	defer func() { monitoringNow = original }()
	now := time.Now().UTC().Truncate(time.Second)
	monitoringNow = func() time.Time { return now }
	for i := 0; i < 17; i++ {
		if _, e := j.st.unit.Exec("INSERT INTO device_session_transitions VALUES(?,?,?,?,?,?)", target.Device.ID, uuid.NewString(), fmt.Sprintf("expiring-%02d", i), now.Add(-24*time.Hour+time.Second).Unix(), "ONLINE", "OFFLINE"); e != nil {
			t.Fatal(e)
		}
	}
	input := equipment.PPInput{DeviceID: target.Device.ID, Scope: "flapping"}
	first, c := j.st.DeviceMonitoring(ctx, companyX, input)
	if c != equipment.OK || !first.More {
		t.Fatal("fixture missing temporal next page", first, c)
	}
	input.Offset = 16
	input.MonitoringGeneration = first.MonitoringGeneration
	now = now.Add(2 * time.Second)
	next, c := j.st.DeviceMonitoring(ctx, companyX, input)
	if c != equipment.OK || len(next.Read.Rows) != 1 {
		t.Fatal("cutoff moved between pages", next, c)
	}
	now = now.Add(120 * time.Second)
	if _, c = j.st.DeviceMonitoring(ctx, companyX, input); c != equipment.CursorInvalid {
		t.Fatal("old cursor outlived bounded snapshot", c)
	}
}

func TestL06T2FlappingOverflowIsNotACompletePrefix(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	tx, e := j.st.unit.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	for i := 0; i < 2049; i++ {
		if _, e = tx.Exec("INSERT INTO device_session_transitions VALUES(?,?,?,?,?,?)", target.Device.ID, uuid.NewString(), fmt.Sprintf("synthetic-%04d", i), time.Now().Unix(), "ONLINE", "OFFLINE"); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	v, c := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "flapping"})
	if c != equipment.Limit || v.Read != nil {
		t.Fatal("2049 identities became complete2048 prefix", v, c)
	}
}

func TestL06T2OperationalTwentyMiBOnlyReclaimsOwnOldestData(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Minute)
	payload := `{"padding":"` + strings.Repeat("x", 1900) + `"}`
	charge := int64(len(payload) + len(target.Device.ID) + 128)
	count := int(monitoringBytesLimit/charge) + 1
	tx, e := j.st.unit.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	statement, e := tx.Prepare("INSERT INTO device_monitoring_points (" + monitoringColumns + ") VALUES(?,?,60,?,?,NULL,NULL,NULL,NULL,?,NULL,0,0,0)")
	if e != nil {
		t.Fatal(e)
	}
	defer statement.Close()
	device := target.Device.ID
	for i := 0; i < count; i++ {
		if i%1300 == 0 && i > 0 {
			device = uuid.NewString()
			if _, e = tx.Exec("INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation) SELECT ?,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation FROM devices WHERE id=?", device, target.Device.ID); e != nil {
				t.Fatal(e)
			}
		}
		at := now.Add(-time.Duration(i%1300) * time.Minute).Unix()
		if _, e = statement.Exec(device, at, at, at, payload); e != nil {
			t.Fatal(e)
		}
	}
	before, e := monitoringBytes(ctx, tx)
	if e != nil || before <= monitoringBytesLimit {
		t.Fatal("fixture did not cross20MiB", before, e)
	}
	if e = monitoringGC(ctx, tx, now); e != nil {
		t.Fatal(e)
	}
	after, e := monitoringBytes(ctx, tx)
	if e != nil || after > monitoringBytesLimit || after != before-charge {
		t.Fatal("20MiB cleanup did not preserve maximum valid prefix", before, after, e)
	}
	var limits int
	if e = tx.QueryRow("SELECT count(*) FROM device_monitoring_limits").Scan(&limits); e != nil || limits != 1 {
		t.Fatal("capacity pruning not disclosed", limits, e)
	}
	t.Logf("synthetic logical series bytes: before=%d after=%d cap=%d", before, after, monitoringBytesLimit)
}

// Exercise the real A publication with a material auxiliary reclamation. B
// fixtures reserve logical bytes (not physical disk space), stay current, and
// cannot be reclaimed by A's GC. Setup time is outside the unchanged 3s budget.
func TestL06T2BasePublicationReclaimsMaterialSeriesWithinBudget(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	protected := ppSynthetic(t, j, target, 1)
	ref := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: protected.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}
	if _, c := j.st.SavePPReference(ctx, companyX, ref, "ppref:material-gc"); c != equipment.OK {
		t.Fatal(c)
	}
	snapshot := func(query string, args ...any) string {
		t.Helper()
		rows, e := j.st.unit.Query(query, args...)
		if e != nil {
			t.Fatal(e)
		}
		defer rows.Close()
		var out strings.Builder
		columns, e := rows.Columns()
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
			values := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range values {
				pointers[i] = &values[i]
			}
			if e := rows.Scan(pointers...); e != nil {
				t.Fatal(e)
			}
			fmt.Fprintf(&out, "%#v\n", values)
		}
		if e := rows.Err(); e != nil {
			t.Fatal(e)
		}
		return out.String()
	}
	protectedSQL := "SELECT meta_json,payload_bytes FROM pppoe_observations WHERE id=?"
	rowsSQL := "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal"
	protectedBefore, rowsBefore := snapshot(protectedSQL, protected.ID), snapshot(rowsSQL, protected.ID)
	tx, e := j.st.unit.Begin()
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	now := time.Now().UTC().Truncate(time.Minute)
	health := `{"padding":"` + strings.Repeat("x", 1900) + `"}`
	charge := int64(len(health) + len(target.Device.ID) + 128)
	count := int((monitoringBytesLimit - 4096) / charge)
	statement, e := tx.Prepare("INSERT INTO device_monitoring_points (" + monitoringColumns + ") VALUES(?,?,60,?,?,NULL,NULL,NULL,NULL,?,NULL,0,0,0)")
	if e != nil {
		t.Fatal(e)
	}
	defer statement.Close()
	device := target.Device.ID
	for i := 0; i < count; i++ {
		if i%1300 == 0 {
			device = uuid.NewString()
			if _, e = tx.Exec("INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation) SELECT ?,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation FROM devices WHERE id=?", device, target.Device.ID); e != nil {
				t.Fatal(e)
			}
		}
		at := now.Add(-time.Duration(i%1300) * time.Minute).Unix()
		if _, e = statement.Exec(device, at, at, at, health); e != nil {
			t.Fatal(e)
		}
	}
	opsBefore, e := monitoringBytes(ctx, tx)
	if e != nil {
		t.Fatal(e)
	}
	total, e := ppTotalBytes(ctx, tx)
	if e != nil {
		t.Fatal(e)
	}
	remaining := int64(equipment.PPCompanyBytes) - 64*1024 - total
	scopes := []string{equipment.PDCatalog, equipment.PDRadius, equipment.PDPing}
	for i := 0; remaining > 0; i++ {
		if i%3 == 0 {
			device = uuid.NewString()
			if _, e = tx.Exec("INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation) SELECT ?,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation FROM devices WHERE id=?", device, target.Device.ID); e != nil {
				t.Fatal(e)
			}
		}
		id, scope := uuid.NewString(), scopes[i%3]
		n := min(remaining, int64(equipment.PPObservationBytes))
		meta, _ := json.Marshal(equipment.PDDiagnostic{ID: id, DeviceID: device, Scope: scope})
		if _, e = tx.Exec("INSERT INTO pppoe_diagnostic_observations VALUES(?,?,?,?,?,?)", id, device, scope, string(meta), 0, n); e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec("INSERT INTO pppoe_diagnostic_current VALUES(?,?,?)", device, scope, id); e != nil {
			t.Fatal(e)
		}
		remaining -= n
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}
	bSQL := "SELECT id,device_id,scope,meta_json,payload_bytes FROM pppoe_diagnostic_observations ORDER BY id"
	bBefore := snapshot(bSQL)
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	job := uuid.NewString()
	if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "material-gc", job); c != equipment.OK {
		t.Fatal(c)
	}
	ledgerSQL := "SELECT id,stamp,entity FROM equipment_operations ORDER BY id"
	ledgerBefore := snapshot(ledgerSQL)
	o := monitoringObservation(target, now.Add(time.Second), uuid.NewString(), 1400)
	for i := 0; i < 1400; i++ {
		o.Rows = append(o.Rows, equipment.PPRow{ID: uuid.NewString(), RemoteID: fmt.Sprintf("*%X", i+1), Login: fmt.Sprintf("synthetic-%04d", i), Address: "198.51.100.1", State: "ONLINE"})
	}
	if !ppWithinBudget(o) || o.Bytes < 320*1024 {
		t.Fatal("candidate must require material reclamation", o.Bytes)
	}
	a := equipment.PPAttempt{JobID: job, Code: equipment.OK, State: "SUCCEEDED", ObservationID: o.ID}
	publication, cancel := context.WithTimeout(ctx, equipment.PPPersistenceBudget)
	defer cancel()
	start := time.Now()
	code := j.st.FinishPPCollection(publication, companyX, target, job, a, o)
	elapsed := time.Since(start)
	t.Logf("real synthetic publication: code=%s duration=%s budget=%s candidate=%d series_before=%d points_before=%d", code, elapsed, equipment.PPPersistenceBudget, o.Bytes, opsBefore, count)
	if code != equipment.OK || elapsed >= equipment.PPPersistenceBudget {
		t.Fatalf("base publication did not commit under unchanged budget: %s (%s)", code, elapsed)
	}
	tx2, e := j.st.unit.Begin()
	if e != nil {
		t.Fatal(e)
	}
	opsAfter, e := monitoringBytes(ctx, tx2)
	if e != nil {
		t.Fatal(e)
	}
	total, e = ppTotalBytes(ctx, tx2)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx2.Rollback(); e != nil {
		t.Fatal(e)
	}
	if opsBefore-opsAfter < 256*1024 || opsAfter > monitoringBytesLimit || total > equipment.PPCompanyBytes {
		t.Fatal("insufficient material reclaim or quota breach", opsBefore, opsAfter, total)
	}
	if snapshot(protectedSQL, protected.ID) != protectedBefore || snapshot(rowsSQL, protected.ID) != rowsBefore || snapshot(bSQL) != bBefore || snapshot(ledgerSQL) != ledgerBefore {
		t.Fatal("GC changed protected observation, B currents, rows or immutable ledger")
	}
	refs, c := j.st.PPReferences(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID})
	if c != equipment.OK || len(refs.References) != 1 || refs.References[0].ID != ref.ReferenceID {
		t.Fatal("protected reference lost", c)
	}
	current, c := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || current.Current == nil || current.Current.ID != o.ID || current.Attempt == nil || current.Attempt.State != "SUCCEEDED" {
		t.Fatal("new current/receipt missing", c, current)
	}
	t.Logf("series reclaimed=%d total_after=%d B preserved; protected reference/rows and operation ledger unchanged", opsBefore-opsAfter, total)
}
