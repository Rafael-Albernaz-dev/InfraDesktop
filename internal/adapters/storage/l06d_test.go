package storage

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func TestL06DContextMigrationReopenAndBudget(t *testing.T) {
	j, target := ppStoreSetup(t)
	dropLaterMonitoringFixtureSchema(t, j)
	ctx := context.Background()
	var before int
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&before)
	if _, e := j.st.unit.Exec("DROP INDEX pppoe_rows_service_vlan; DELETE FROM schema_migrations WHERE version=8"); e != nil {
		t.Fatal(e)
	}
	original := companySessionContextSchema
	companySessionContextSchema = original + "; invalid syntax;"
	err := j.st.validateUnitIdentity(j.st.unit, companyX)
	companySessionContextSchema = original
	if err == nil {
		t.Fatal("invalid008 accepted")
	}
	var partial int
	j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='pppoe_rows_service_vlan'").Scan(&partial)
	if partial != 0 {
		t.Fatal("failed008 left DDL")
	}
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	job := uuid.NewString()
	if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "read", job); c != equipment.OK {
		t.Fatal(c)
	}
	o := &equipment.PPObservation{ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Provenance: equipment.PPProvenance{Ended: equipment.PPTime(time.Now())}}
	vlan := 100
	for i := 0; i < 2; i++ {
		o.Rows = append(o.Rows, equipment.PPRow{ID: uuid.NewString(), Login: "synthetic", Context: &equipment.PPContext{Association: "associated", VLAN: &vlan, Service: "service", Profile: "profile"}})
	}
	if c := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job}, o); c != equipment.OK {
		t.Fatal(c)
	}
	if e := j.st.unit.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := j.st.openUnit(companyX)
	if e != nil {
		t.Fatal(e)
	}
	j.st.unit = db
	page, c := j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID})
	if c != equipment.OK || len(page.Rows) != 2 || page.Rows[0].Context == nil || page.Rows[0].Context.Profile != "profile" {
		t.Fatal("stable context lost on reopen", c)
	}
	groups, c := j.st.PPContextCounts(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID})
	if c != equipment.OK || len(groups.Groups) != 1 || groups.Groups[0].Count != 2 || *groups.Groups[0].VLAN != 100 {
		t.Fatal("groups not same immutable reading", c, groups.Groups)
	}
	var after int
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&after)
	if after != before {
		t.Fatal("D read mutated ledger")
	}
	measured := *o
	measured.ID = uuid.NewString()
	measured.Rows = nil
	for i := 0; i < 2048; i++ {
		row := o.Rows[0]
		row.ID = uuid.NewString()
		measured.Rows = append(measured.Rows, row)
	}
	raw, _ := json.Marshal(measured.Rows[0])
	within := ppWithinBudget(&measured)
	t.Logf("S1 budget synthetic2048: row_bytes=%d observation_bytes=%d cap=%d fits=%v (no limit changed)", len(raw), measured.Bytes, equipment.PPObservationBytes, within)
	if len(raw) > equipment.PPRowBytes {
		t.Fatal("representative stable row overflows row cap")
	}
}
