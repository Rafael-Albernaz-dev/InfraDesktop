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
	good := seedSharedBudget(t, j, target)
	verifySharedBudgetRefusals(t, j, target, good)
	verifySharedBudgetContext(t, j, target, good)
}
