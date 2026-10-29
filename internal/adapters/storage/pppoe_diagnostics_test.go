package storage

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func dropPPDiagnosticFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	for _, table := range []string{"device_context_samples", "device_equipment_snapshot", "device_equipment_events", "device_monitoring_points", "device_session_transitions", "device_monitoring_checkpoint", "device_monitoring_limits", "device_monitoring_generation", "device_monitoring_packages", "device_monitoring_package_status"} {
		if _, err := j.st.unit.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, e := j.st.unit.Exec("DROP TABLE equipment_read_receipt_migration; ALTER TABLE devices DROP COLUMN role; DROP INDEX pppoe_rows_service_vlan; DELETE FROM schema_migrations WHERE version>=7"); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"pppoe_diagnostic_attempts", "pppoe_diagnostic_current", "pppoe_diagnostic_rows", "pppoe_diagnostic_observations"} {
		if _, err := j.st.unit.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version=6"); err != nil {
		t.Fatal(err)
	}
}
func pdSynthetic(t *testing.T, j *journey, target equipment.PPTarget, scope string, n int) (equipment.PPInput, *equipment.PDDiagnostic) {
	t.Helper()
	ctx := context.Background()
	in := equipment.PPInput{DeviceID: target.Device.ID, Scope: scope, Revision: target.Device.Revision, Operation: uuid.NewString(), WorkerEpoch: "synthetic", SavedAt: "2026-10-26T00:00:00Z"}
	job := uuid.NewString()
	if _, _, c := j.st.ReservePPDiagnostic(ctx, companyX, in, "diagnostic:"+in.Operation, job); c != equipment.OK {
		t.Fatal(c)
	}
	o := &equipment.PDDiagnostic{ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: scope, Quality: equipment.PPQuality{Status: "complete", Code: equipment.OK, Terminated: true, MembershipChecked: true}, Provenance: equipment.PPProvenance{Ended: in.SavedAt}}
	for i := 0; i < n; i++ {
		o.Rows = append(o.Rows, equipment.PDRow{ID: uuid.NewString(), Kind: "interface", Name: "synthetic"})
	}
	attempt := equipment.PPAttempt{Epoch: "synthetic", JobID: job, State: "SUCCEEDED", Code: equipment.OK, At: in.SavedAt}
	if c := j.st.FinishPPDiagnostic(ctx, companyX, target, in, job, attempt, o); c != equipment.OK {
		t.Fatal(c)
	}
	return in, o
}
func TestPPDiagnosticPersistencePageReopenGCAndFailure(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	in, first := pdSynthetic(t, j, target, equipment.PDCatalog, 17)
	var persisted, metadata, lines int
	if err := j.st.unit.QueryRow("SELECT payload_bytes,length(CAST(meta_json AS BLOB)),(SELECT sum(length(CAST(row_json AS BLOB))) FROM pppoe_diagnostic_rows WHERE diagnostic_id=?) FROM pppoe_diagnostic_observations WHERE id=?", first.ID, first.ID).Scan(&persisted, &metadata, &lines); err != nil || persisted != metadata+lines || persisted != first.Bytes {
		t.Fatal("final metadata/row bytes not exact", persisted, metadata, lines, first.Bytes, err)
	}
	in.DiagnosticID = first.ID
	page, c := j.st.PPDiagnosticPage(ctx, companyX, in)
	if c != equipment.OK || len(page.DiagnosticRows) != 16 || !page.More || page.Total != 17 {
		t.Fatal(page, c)
	}
	in.Offset = 16
	page, c = j.st.PPDiagnosticPage(ctx, companyX, in)
	if c != equipment.OK || len(page.DiagnosticRows) != 1 || page.More {
		t.Fatal(page, c)
	}
	_, second := pdSynthetic(t, j, target, equipment.PDCatalog, 2)
	if _, c = j.st.PPDiagnosticPage(ctx, companyX, in); c != equipment.ObservationMissing {
		t.Fatal("stale diagnostic ID accepted", c)
	}
	var n int
	if err := j.st.unit.QueryRow("SELECT count(*) FROM pppoe_diagnostic_observations").Scan(&n); err != nil || n != 1 {
		t.Fatal("hidden diagnostic history", n, err)
	}
	failed := in
	failed.Operation = uuid.NewString()
	job := uuid.NewString()
	if _, _, c = j.st.ReservePPDiagnostic(ctx, companyX, failed, "diagnostic:failure", job); c != equipment.OK {
		t.Fatal(c)
	}
	if c = j.st.FinishPPDiagnostic(ctx, companyX, target, failed, job, equipment.PPAttempt{JobID: job, State: "FAILED", Code: equipment.Partial, At: "2026-10-26T01:00:00Z"}, nil); c != equipment.OK {
		t.Fatal(c)
	}
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyY, access.OK)
	j.selectCompany(companyX, access.OK)
	current, c := j.st.PPDiagnosticCurrent(ctx, companyX, in)
	if c != equipment.OK || current.Diagnostic == nil || current.Diagnostic.ID != second.ID || current.Diagnostic.Provenance.Ended != second.Provenance.Ended || current.Attempt == nil || current.Attempt.Code != equipment.Partial {
		t.Fatal("failure/reopen lost last good/date", current, c)
	}
}

func TestPPDiagnosticFinalByteBudget(t *testing.T) {
	o := &equipment.PDDiagnostic{ID: "synthetic", Scope: equipment.PDCatalog, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}}
	for i := 0; i < 260; i++ {
		o.Rows = append(o.Rows, equipment.PDRow{ID: uuid.NewString(), Kind: "interface", Name: strings.Repeat("x", 1900)})
	}
	if !pdBudget(o) {
		t.Fatal("below-cap synthetic serialized payload rejected")
	}
	meta, _ := json.Marshal(o)
	actual := len(meta)
	for _, row := range o.Rows {
		body, _ := json.Marshal(row)
		actual += len(body)
	}
	if o.Bytes != actual || actual > equipment.PPObservationBytes {
		t.Fatal("metadata point fixed incorrectly", o.Bytes, actual)
	}
	for i := 0; i < 10; i++ {
		o.Rows = append(o.Rows, equipment.PDRow{ID: uuid.NewString(), Kind: "interface", Name: strings.Repeat("x", 1900)})
	}
	if pdBudget(o) {
		t.Fatal("above-cap serialized payload accepted")
	}
}
func TestPPDiagnosticMigrationSixIsAdditiveAtomic(t *testing.T) {
	j, target := ppStoreSetup(t)
	original := companyPPDiagnosticSchema
	defer func() { companyPPDiagnosticSchema = original }()
	session := ppSynthetic(t, j, target, 2)
	dropPPDiagnosticFixtureSchema(t, j)
	var before string
	if err := j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM schema_migrations ORDER BY version").Scan(&before); err != nil {
		t.Fatal(err)
	}
	companyPPDiagnosticSchema = original + "; invalid syntax;"
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err == nil {
		t.Fatal("invalid migration accepted")
	}
	var n int
	if err := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name LIKE 'pppoe_diagnostic_%'").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial DDL", n, err)
	}
	companyPPDiagnosticSchema = original
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err != nil {
		t.Fatal(err)
	}
	var after string
	if err := j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM schema_migrations WHERE version<=5 ORDER BY version").Scan(&after); err != nil || after != before {
		t.Fatal("migration1–5 changed", before, after, err)
	}
	view, c := j.st.PPCurrent(context.Background(), companyX, target.Device.ID)
	if c != equipment.OK || view.Current == nil || view.Current.ID != session.ID {
		t.Fatal("existing sessions changed", view, c)
	}
}

func TestPPDiagnosticSharedCompanyBudgetBothPublicationDirections(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	good := ppSynthetic(t, j, target, 16)
	if _, code := j.st.SavePPReference(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: good.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString(), SavedAt: "2026-10-26T00:00:00Z"}, "ppref:protected-for-quota"); code != equipment.OK {
		t.Fatal(code)
	}
	var sessionBytes int64
	if err := j.st.unit.QueryRow("SELECT sum(payload_bytes) FROM pppoe_observations").Scan(&sessionBytes); err != nil {
		t.Fatal(err)
	}
	// Synthetic LOGICAL accounting boundary, not a disk/RSS/firmware benchmark.
	// Small valid records deliberately reserve their maximum declared bytes.
	tx, err := j.st.unit.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// Include T2 disposable series already created by the initial base reading
	// when constructing the same exact160MiB logical starting state.
	monitoringFootprint, err := monitoringBytes(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	devices := []string{}
	for i := 0; i < 107; i++ {
		id := uuid.NewString()
		devices = append(devices, id)
		if _, err = tx.Exec("INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation) SELECT ?,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,observation FROM devices WHERE id=?", id, target.Device.ID); err != nil {
			t.Fatal(err)
		}
	}
	scopes := []string{equipment.PDCatalog, equipment.PDRadius, equipment.PDPing}
	for i := 0; i < 320; i++ {
		id, device, scope := uuid.NewString(), devices[i/3], scopes[i%3]
		bytes := int64(equipment.PPObservationBytes)
		if i == 319 {
			bytes -= sessionBytes + monitoringFootprint
		}
		meta, _ := json.Marshal(equipment.PDDiagnostic{ID: id, DeviceID: device, Scope: scope})
		if _, err = tx.Exec("INSERT INTO pppoe_diagnostic_observations VALUES(?,?,?,?,?,?)", id, device, scope, string(meta), 0, bytes); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec("INSERT INTO pppoe_diagnostic_current VALUES(?,?,?)", device, scope, id); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	// An A publication must count B, preserve the protected reading and roll back.
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	job := uuid.NewString()
	if _, _, code := j.st.ReservePPCollection(ctx, companyX, in, "synthetic:quota-a", job); code != equipment.OK {
		t.Fatal(code)
	}
	candidate := *good
	candidate.ID = uuid.NewString()
	candidate.Rows = append([]equipment.PPRow{}, good.Rows...)
	if code := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job, State: "SUCCEEDED", Code: equipment.OK}, &candidate); code != equipment.Limit {
		t.Fatal("A ignored B footprint", code)
	}
	current, code := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	if code != equipment.OK || current.Current.ID != good.ID {
		t.Fatal("failed A publication replaced last good", current, code)
	}
	// Conversely a B publication must count A, even though its candidate is small.
	in.Scope = equipment.PDCatalog
	in.Operation = uuid.NewString()
	job = uuid.NewString()
	if _, _, code = j.st.ReservePPDiagnostic(ctx, companyX, in, "synthetic:quota-b", job); code != equipment.OK {
		t.Fatal(code)
	}
	diagnostic := &equipment.PDDiagnostic{ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PDCatalog, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}}
	if code = j.st.FinishPPDiagnostic(ctx, companyX, target, in, job, equipment.PPAttempt{JobID: job, State: "SUCCEEDED", Code: equipment.OK}, diagnostic); code != equipment.Limit {
		t.Fatal("B ignored A footprint", code)
	}
	view, code := j.st.PPDiagnosticCurrent(ctx, companyX, in)
	if code != equipment.OK || view.Diagnostic != nil {
		t.Fatal("failed B publication became visible", view, code)
	}
	check, err := j.st.unit.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer check.Rollback()
	bytes, err := ppTotalBytes(ctx, check)
	if err != nil || bytes != equipment.PPCompanyBytes {
		t.Fatal("quota changed after rollback", bytes, err)
	}
	if err := check.Rollback(); err != nil {
		t.Fatal(err)
	}
	// Free only enough logical A+B budget for a new base reading, while the
	// previous current remains protected by its reference. Context must yield.
	next := capacityObservation(10)
	next.DeviceID = target.Device.ID
	next.Provenance = good.Provenance
	base := *next
	base.Rows = append([]equipment.PPRow(nil), next.Rows...)
	for i := range base.Rows {
		base.Rows[i].Context = nil
		base.Rows[i].Duplicates = 10
	}
	space := capacityBytes(&base) + 64 // includes the omission marker itself
	if _, err := j.st.unit.Exec("UPDATE pppoe_diagnostic_observations SET payload_bytes=payload_bytes-? WHERE id=(SELECT id FROM pppoe_diagnostic_observations LIMIT 1)", space); err != nil {
		t.Fatal(err)
	}
	job = uuid.NewString()
	in = equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	if _, _, code := j.st.ReservePPCollection(ctx, companyX, in, "synthetic:context-quota", job); code != equipment.OK {
		t.Fatal(code)
	}
	if code := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job, State: "SUCCEEDED", Code: equipment.OK}, next); code != equipment.OK {
		t.Fatal("optional context displaced base at shared company quota", code)
	}
	if !next.ContextOmitted || next.Rows[0].Context != nil || next.Bytes > space {
		t.Fatal("company context omission not explicit/bounded", next.Bytes, space)
	}
	retained, code := j.st.PPObservation(ctx, companyX, target.Device.ID, good.ID)
	if code != equipment.OK || retained.Count != good.Count || retained.ContextOmitted {
		t.Fatal("protected reading changed at company quota", code)
	}
}
