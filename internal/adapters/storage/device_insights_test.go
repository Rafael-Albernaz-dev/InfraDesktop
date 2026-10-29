package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func TestL06T3SavedPortsRestartReplayAndReopen(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Minute)
	up, full, half := true, true, false
	speed := uint64(100000000)
	downs, more := "33", "34"
	fast := uint64(1)
	save := func(stamp time.Time, uptime int64, port *equipment.DevicePort) {
		t.Helper()
		h := &equipment.DeviceHealth{Started: equipment.PPTime(stamp), At: equipment.PPTime(stamp.Add(time.Millisecond)), Uptime: equipment.PPInterval{Known: true, Min: uptime * int64(time.Second), Max: (uptime + 1) * int64(time.Second)}, FastTrack: &fast}
		r := &equipment.PPReadResult{Started: h.Started, Ended: h.At, Scope: "operations", Availability: map[string]string{"packages": "unavailable", "ports": "complete"}, Rows: []equipment.PPReadRow{{Kind: "resource", Health: h}, {Kind: "port", Name: "X", Port: port}}}
		if c := j.st.SaveDeviceMonitoring(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r); c != equipment.OK {
			t.Fatal(c)
		}
	}
	save(at, 100, &equipment.DevicePort{Running: &up, SpeedBPS: &speed, FullDuplex: &full, LinkDowns: &downs})
	save(at.Add(10*time.Second), 5, &equipment.DevicePort{Running: &up, SpeedBPS: &speed, FullDuplex: &half, LinkDowns: &more})
	save(at.Add(10*time.Second), 5, &equipment.DevicePort{Running: &up, SpeedBPS: &speed, FullDuplex: &half, LinkDowns: &more})
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyY, access.OK)
	j.selectCompany(companyX, access.OK)
	v, c := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "equipment"})
	if c != equipment.OK {
		t.Fatalf("saved equipment not available: %s", c)
	}
	var restarts, ports, resource bool
	for _, r := range v.Read.Rows {
		switch r.Kind {
		case "restarts":
			restarts = r.Used != nil && *r.Used == 1 && r.Size != nil && *r.Size == 1
		case "port":
			ports = r.Port != nil && r.Port.FullDuplex != nil && !*r.Port.FullDuplex
		case "resource":
			resource = r.Health != nil && r.Health.FastTrack != nil && *r.Health.FastTrack == 1
		}
	}
	if !restarts || !ports || !resource {
		t.Fatal("snapshot projection lost facts", v.Read.Rows)
	}
	v, c = j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "port-history"})
	if c != equipment.OK || len(v.Read.Rows) != 1 || v.Read.Rows[0].BeforePort == nil || v.Read.Rows[0].Port == nil || *v.Read.Rows[0].Port.LinkDowns != "34" {
		t.Fatal("port delta/replay", c, v.Read)
	}
	var version int
	if e := j.st.unit.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); e != nil || version != 12 {
		t.Fatal("additive migration", version, e)
	}
	if e := j.st.validateUnit(j.st.unit, companyX); e != nil {
		t.Fatal("reopen checksum", e)
	}
	// A stale replay must not replace the new snapshot or create another event.
	save(at, 100, &equipment.DevicePort{Running: &up, SpeedBPS: &speed, FullDuplex: &full, LinkDowns: &downs})
	var n int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM device_equipment_events").Scan(&n); e != nil || n != 2 {
		t.Fatal(fmt.Sprint("replay events ", n, e))
	}
}

func dropLaterMonitoringFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	dropRadioFixtureSchema(t, j)
	for _, table := range []string{"device_context_samples", "device_equipment_snapshot", "device_equipment_events", "device_monitoring_points", "device_session_transitions", "device_monitoring_checkpoint", "device_monitoring_limits", "device_monitoring_generation", "device_monitoring_packages", "device_monitoring_package_status"} {
		if _, e := j.st.unit.Exec("DROP TABLE " + table); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version>=9"); e != nil {
		t.Fatal(e)
	}
}
func TestL06T3MigrationTenAtomicPreservesAppliedNine(t *testing.T) {
	j, _ := ppStoreSetup(t)
	dropRadioFixtureSchema(t, j)
	var before, after string
	if e := j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations WHERE version<=9 ORDER BY version)").Scan(&before); e != nil {
		t.Fatal(e)
	}
	if _, e := j.st.unit.Exec("DROP TABLE device_context_samples;DROP TABLE device_equipment_snapshot;DROP TABLE device_equipment_events;DELETE FROM schema_migrations WHERE version>=10"); e != nil {
		t.Fatal(e)
	}
	original := companyEquipmentInsightsSchema
	defer func() { companyEquipmentInsightsSchema = original }()
	companyEquipmentInsightsSchema = original + ";invalid SQL;"
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e == nil {
		t.Fatal("bad010 accepted")
	}
	var n int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name IN ('device_equipment_snapshot','device_equipment_events')").Scan(&n); e != nil || n != 0 {
		t.Fatal("partial010 DDL escaped", n, e)
	}
	companyEquipmentInsightsSchema = original
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	if e := j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations WHERE version<=9 ORDER BY version)").Scan(&after); e != nil || before != after {
		t.Fatal("applied001..009 changed", e)
	}
}
func TestL06T3CatalogAbsenceSavedAndReopened(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Minute)
	dynamic, up := true, true
	save := func(stamp time.Time, rows []equipment.PDRow) *equipment.PDDiagnostic {
		t.Helper()
		job := uuid.NewString()
		in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString(), Scope: equipment.PDCatalog, SavedAt: equipment.PPTime(stamp)}
		if _, _, c := j.st.ReservePPDiagnostic(ctx, companyX, in, "catalog:"+in.Operation, job); c != equipment.OK {
			t.Fatal(c)
		}
		p := monitoringObservation(target, stamp, uuid.NewString(), 0).Provenance
		p.Projection = "catalog"
		o := &equipment.PDDiagnostic{ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PDCatalog, Provenance: p, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Rows: rows}
		if c := j.st.FinishPPDiagnostic(ctx, companyX, target, in, job, equipment.PPAttempt{JobID: job, Code: equipment.OK, State: "SUCCEEDED"}, o); c != equipment.OK {
			t.Fatal(c)
		}
		return o
	}
	save(at, []equipment.PDRow{{ID: "present", Name: "X", Kind: "interface", Dynamic: &dynamic, Running: &up}})
	next := save(at.Add(time.Minute), nil)
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyY, access.OK)
	j.selectCompany(companyX, access.OK)
	// Read the typed stored row: absence is preserved but current UP is not.
	v, c := j.st.PPDiagnosticPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: equipment.PDCatalog, DiagnosticID: next.ID})
	if c != equipment.OK || len(v.DiagnosticRows) != 1 || v.DiagnosticRows[0].Running != nil || v.DiagnosticRows[0].NotSeenSince == "" {
		t.Fatal("saved absence lost", c, v)
	}
	if e := j.st.validateUnit(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	var raw string
	if e := j.st.unit.QueryRow("SELECT row_json FROM pppoe_diagnostic_rows WHERE diagnostic_id=?", next.ID).Scan(&raw); e != nil {
		t.Fatal(e)
	}
	var r equipment.PDRow
	if e := json.Unmarshal([]byte(raw), &r); e != nil || r.Running != nil {
		t.Fatal("serialization fabricated UP", e)
	}
}
