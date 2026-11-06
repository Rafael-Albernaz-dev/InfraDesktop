package storage

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func seedSharedBudget(t *testing.T, j *journey, target equipment.PPTarget) *equipment.PPObservation {
	t.Helper()
	ctx := context.Background()
	good := ppSynthetic(t, j, target, 16)
	reference := equipment.PPInput{
		DeviceID: target.Device.ID, ObservationID: good.ID, ReferenceID: uuid.NewString(),
		Operation: uuid.NewString(), SavedAt: "2026-10-26T00:00:00Z",
	}
	if _, code := j.st.SavePPReference(ctx, companyX, reference, "ppref:protected-for-quota"); code != equipment.OK {
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
	// when constructing the exact logical starting state at the company limit.
	monitoringFootprint, err := monitoringBytes(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	scopes := []string{equipment.PDCatalog, equipment.PDRadius, equipment.PDPing}
	devices := []string{}
	for i := 0; i < (equipment.MaxDevices+equipment.PPReferenceCompany+len(scopes)-1)/len(scopes); i++ {
		id := uuid.NewString()
		devices = append(devices, id)
		if _, err = tx.Exec("INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,"+
			"fingerprint,trust_origin,observation) SELECT ?,name,address,port,transport,"+
			"credential_id,revision,active,fingerprint,trust_origin,observation FROM devices WHERE id=?",
			id, target.Device.ID); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < equipment.MaxDevices+equipment.PPReferenceCompany; i++ {
		id, device, scope := uuid.NewString(), devices[i/3], scopes[i%3]
		bytes := int64(equipment.PPObservationBytes)
		if i == equipment.MaxDevices+equipment.PPReferenceCompany-1 {
			bytes -= sessionBytes + monitoringFootprint
		}
		meta, _ := json.Marshal(equipment.PDDiagnostic{ID: id, DeviceID: device, Scope: scope})
		if _, err = tx.Exec("INSERT INTO pppoe_diagnostic_observations VALUES(?,?,?,?,?,?)",
			id, device, scope, string(meta), 0, bytes); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec("INSERT INTO pppoe_diagnostic_current VALUES(?,?,?)", device, scope, id); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return good
}

func verifySharedBudgetRefusals(t *testing.T, j *journey, target equipment.PPTarget, good *equipment.PPObservation) {
	t.Helper()
	ctx := context.Background()
	// An A publication must count B, preserve the protected reading and roll back.
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	job := uuid.NewString()
	if _, _, code := j.st.ReservePPCollection(ctx, companyX, in, "synthetic:quota-a", job); code != equipment.OK {
		t.Fatal(code)
	}
	candidate := *good
	candidate.ID = uuid.NewString()
	candidate.Rows = append([]equipment.PPRow{}, good.Rows...)
	attempt := equipment.PPAttempt{JobID: job, State: "SUCCEEDED", Code: equipment.OK}
	if code := j.st.FinishPPCollection(ctx, companyX, target, job, attempt, &candidate); code != equipment.Limit {
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
	diagnostic := &equipment.PDDiagnostic{
		ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PDCatalog,
		Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true},
	}
	attempt.JobID = job
	if code = j.st.FinishPPDiagnostic(ctx, companyX, target, in, job, attempt, diagnostic); code != equipment.Limit {
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
}

func verifySharedBudgetContext(t *testing.T, j *journey, target equipment.PPTarget, good *equipment.PPObservation) {
	t.Helper()
	ctx := context.Background()
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
	if _, err := j.st.unit.Exec("UPDATE pppoe_diagnostic_observations SET payload_bytes=payload_bytes-? "+
		"WHERE id=(SELECT id FROM pppoe_diagnostic_observations LIMIT 1)", space); err != nil {
		t.Fatal(err)
	}
	job := uuid.NewString()
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	if _, _, code := j.st.ReservePPCollection(ctx, companyX, in, "synthetic:context-quota", job); code != equipment.OK {
		t.Fatal(code)
	}
	attempt := equipment.PPAttempt{JobID: job, State: "SUCCEEDED", Code: equipment.OK}
	if code := j.st.FinishPPCollection(ctx, companyX, target, job, attempt, next); code != equipment.OK {
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
