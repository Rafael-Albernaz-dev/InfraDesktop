package storage

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"reflect"
	"testing"
	"time"
)

// Only synthetic migration fixtures use this to reconstruct a pre-v5 schema.
func dropPPPoEFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	dropPPDiagnosticFixtureSchema(t, j)
	for _, table := range []string{"pppoe_attempts", "pppoe_references", "pppoe_current", "pppoe_rows", "pppoe_observations"} {
		if _, e := j.st.unit.Exec("DROP TABLE " + table); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version=5"); e != nil {
		t.Fatal(e)
	}
}

func ppStoreSetup(t *testing.T) (*journey, equipment.PPTarget) {
	t.Helper()
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	_, d := setupEquipment(t, j, s, 2222)
	target, c := j.st.ReadPPTarget(context.Background(), companyX, d.ID, true)
	if c != equipment.OK {
		t.Fatal(c)
	}
	return j, target
}
func ppSynthetic(t *testing.T, j *journey, target equipment.PPTarget, n int, logins ...string) *equipment.PPObservation {
	t.Helper()
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	job := uuid.NewString()
	if _, _, c := j.st.ReservePPCollection(context.Background(), companyX, in, "synthetic:"+job, job); c != equipment.OK {
		t.Fatal(c)
	}
	o := &equipment.PPObservation{ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Provenance: equipment.PPProvenance{Company: companyX, Installation: target.Installation, Ended: time.Now().UTC().Format(time.RFC3339Nano)}}
	login := "duplicate"
	if len(logins) > 0 {
		login = logins[0]
	}
	for i := 0; i < n; i++ {
		o.Rows = append(o.Rows, equipment.PPRow{ID: uuid.NewString(), RemoteID: "*" + uuid.NewString(), Login: login, Address: "198.51.100.1"})
	}
	a := equipment.PPAttempt{JobID: job, Code: equipment.OK, State: "SUCCEEDED", ObservationID: o.ID}
	if c := j.st.FinishPPCollection(context.Background(), companyX, target, job, a, o); c != equipment.OK {
		t.Fatal(c)
	}
	return o
}
func TestPPPoEStoreReferencesGCPageAndReopen(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	a := ppSynthetic(t, j, target, 17)
	ref := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: a.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}
	if _, c := j.st.SavePPReference(ctx, companyX, ref, "ppref:save-a"); c != equipment.OK {
		t.Fatal(c)
	}
	b := ppSynthetic(t, j, target, 2)
	v, c := j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: a.ID, Query: &equipment.PPQuery{Duplicates: true}})
	if c != equipment.OK || len(v.Rows) != 16 || !v.More || v.Total != 17 {
		t.Fatal(v, c)
	}
	v, c = j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: a.ID, Offset: 16})
	if c != equipment.OK || len(v.Rows) != 1 || v.More {
		t.Fatal(v, c)
	}
	// Removing a reference cannot remove current or a subsequently saved association.
	del := equipment.PPInput{DeviceID: target.Device.ID, ReferenceID: ref.ReferenceID, Operation: uuid.NewString()}
	if _, c = j.st.DeletePPReference(ctx, companyX, del, "ppref:delete-a"); c != equipment.OK {
		t.Fatal(c)
	}
	if _, c = j.st.PPObservation(ctx, companyX, target.Device.ID, a.ID); c != equipment.ObservationMissing {
		t.Fatal("unowned A was not collected", c)
	}
	if _, c = j.st.PPObservation(ctx, companyX, target.Device.ID, b.ID); c != equipment.OK {
		t.Fatal("current removed", c)
	}
	fresh := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: b.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}
	if _, c = j.st.SavePPReference(ctx, companyX, fresh, "ppref:save-b"); c != equipment.OK {
		t.Fatal(c)
	}
	if _, c = j.st.DeletePPReference(ctx, companyX, del, "ppref:delete-a"); c != equipment.OK {
		t.Fatal(c)
	}
	refs, c := j.st.PPReferences(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID})
	if c != equipment.OK || len(refs.References) != 1 || refs.References[0].ID != fresh.ReferenceID {
		t.Fatal(refs, c)
	}
	// Reopen actual company DB through selection; old connection is closed.
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyY, access.OK)
	j.selectCompany(companyX, access.OK)
	cur, c := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || cur.Current.ID != b.ID {
		t.Fatal(cur, c)
	}
	// At most current + explicit references, even after ten refreshes.
	for i := 0; i < 10; i++ {
		ppSynthetic(t, j, target, 1)
	}
	var n int
	if j.st.unit.QueryRow("SELECT count(*) FROM pppoe_observations").Scan(&n) != nil || n != 2 {
		t.Fatal("hidden history", n)
	}
}
func TestPPPoEMigrationRollbackAndLegacyPreservation(t *testing.T) {
	j, target := ppStoreSetup(t)
	before, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	dropPPPoEFixtureSchema(t, j)
	var checksums string
	if j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM schema_migrations ORDER BY version").Scan(&checksums) != nil {
		t.Fatal("checksums")
	}
	original := companyPPPoESchema
	companyPPPoESchema = original + "; invalid syntax;"
	defer func() { companyPPPoESchema = original }()
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e == nil {
		t.Fatal("invalid migration passed")
	}
	var n int
	if j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name LIKE 'pppoe_%'").Scan(&n) != nil || n != 0 {
		t.Fatal("partial DDL persisted", n)
	}
	if j.st.unit.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&n) != nil || n != 4 {
		t.Fatal("ledger migration partial", n)
	}
	companyPPPoESchema = original
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	after, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK || !reflect.DeepEqual(before, after) {
		t.Fatal("legacy changed", c)
	}
	var final string
	if j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM schema_migrations WHERE version<=4 ORDER BY version").Scan(&final) != nil || final != checksums {
		t.Fatal("checksums1-4 changed")
	}
	a := ppSynthetic(t, j, target, 0)
	if a.Count != 0 {
		t.Fatal("zero not complete")
	}
	if _, e := j.st.unit.Exec("INSERT INTO pppoe_current VALUES(?,?,?)", uuid.NewString(), equipment.PPScope, a.ID); e == nil {
		t.Fatal("composite/device FK not enforced")
	}
}
func TestPPPoECommitFailurePreservesGood(t *testing.T) {
	j, target := ppStoreSetup(t)
	a := ppSynthetic(t, j, target, 2)
	ctx := context.Background()
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	job := uuid.NewString()
	j.st.ReservePPCollection(ctx, companyX, in, "next", job)
	// Real SQL trigger fails after the candidate was inserted; entire publish must roll back.
	if _, e := j.st.unit.Exec("CREATE TRIGGER pp_fail BEFORE UPDATE ON pppoe_current BEGIN SELECT RAISE(ABORT,'synthetic commit failure'); END"); e != nil {
		t.Fatal(e)
	}
	b := *a
	b.ID = uuid.NewString()
	b.Rows = []equipment.PPRow{{ID: uuid.NewString(), Login: "new"}}
	if c := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job}, &b); c != equipment.Unavailable {
		t.Fatal(c)
	}
	cur, c := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || cur.Current.ID != a.ID {
		t.Fatal("last good lost", c)
	}
	raw, _ := json.Marshal(cur.Current)
	if len(raw) == 0 {
		t.Fatal("metadata")
	}
	var n int
	j.st.unit.QueryRow("SELECT count(*) FROM pppoe_observations").Scan(&n)
	if n != 1 {
		t.Fatal("candidate survived rollback", n)
	}
}

func TestPPPoEReviewFinalBudgetBytes(t *testing.T) {
	o := &equipment.PPObservation{ID: uuid.NewString(), Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Rows: []equipment.PPRow{{ID: uuid.NewString(), Login: "ação"}}}
	if !ppWithinBudget(o) {
		t.Fatal("positive rejected")
	}
	b, _ := json.Marshal(o)
	expected := len(b)
	for _, r := range o.Rows {
		b, _ = json.Marshal(r)
		expected += len(b)
	}
	if o.Bytes != expected {
		t.Fatalf("stored %d differs from final actual %d", o.Bytes, expected)
	}
}

func TestPPPoEUnicodeSQLAndReferenceBounds(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	var last *equipment.PPObservation
	for i := 0; i < 9; i++ {
		last = ppSynthetic(t, j, target, 1, "Áção")
		v, c := j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: last.ID, Query: &equipment.PPQuery{Search: "ÁÇÃO"}})
		if c != equipment.OK || v.Total != 1 || len(v.Rows) != 1 {
			t.Fatal("SQL Unicode folding failed", c, v)
		}
		in := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: last.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString(), SavedAt: equipment.PPTime(time.Now())}
		_, c = j.st.SavePPReference(ctx, companyX, in, "ppref:"+in.Operation)
		if i < 8 && c != equipment.OK {
			t.Fatal(c)
		}
		if i == 8 && c != equipment.Limit {
			t.Fatal("reference cap", c)
		}
	}
	refs, c := j.st.PPReferences(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID})
	if c != equipment.OK || len(refs.References) != 8 {
		t.Fatal(c, refs)
	}
}

func TestPPPoECompanyReferenceBoundary(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	var last equipment.PPInput
	for deviceIndex := 0; deviceIndex < 9; deviceIndex++ {
		if deviceIndex > 0 {
			target.Device.ID = uuid.NewString()
			if c := j.st.UpdateEquipment(ctx, companyX, func(s *equipment.State) equipment.Code {
				s.Devices = append(s.Devices, target.Device)
				return equipment.OK
			}); c != equipment.OK {
				t.Fatal(c)
			}
		}
		for refIndex := 0; refIndex < 8; refIndex++ {
			observation := ppSynthetic(t, j, target, 0)
			in := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: observation.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString(), SavedAt: equipment.PPTime(time.Now())}
			v, c := j.st.SavePPReference(ctx, companyX, in, "boundary:"+in.Operation)
			if deviceIndex == 8 {
				if c != equipment.Limit || v.Reason != "references_company" {
					t.Fatal("65th reference cause", c, v.Reason)
				}
				var total int
				if e := j.st.unit.QueryRow("SELECT count(*) FROM pppoe_references").Scan(&total); e != nil || total != 64 {
					t.Fatal("company quota changed", total, e)
				}
				// The last known successful receipt remains replayable at the ceiling.
				if _, c := j.st.SavePPReference(ctx, companyX, last, "boundary:"+last.Operation); c != equipment.OK {
					t.Fatal("replay at64", c)
				}
				return
			}
			if c != equipment.OK {
				t.Fatal(deviceIndex, refIndex, c)
			}
			last = in
		}
	}
	t.Fatal("boundary not exercised")
}

func TestPPPoEAH03CanonicalReferenceAcrossCreateReuseReplayAndReopen(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	o := ppSynthetic(t, j, target, 3)
	in := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString(), SavedAt: "2026-10-25T00:00:00Z"}
	first, c := j.st.SavePPReference(ctx, companyX, in, "ppref:canonical-first")
	if c != equipment.OK || first.Reference == nil || first.Reference.Count != 3 {
		t.Fatal(first, c)
	}
	want := *first.Reference
	// Both retries and a distinct save of the same observation project persisted values.
	in.SavedAt = "2026-10-26T01:00:00Z"
	replay, c := j.st.SavePPReference(ctx, companyX, in, "ppref:canonical-first")
	if c != equipment.OK || replay.Reference == nil || *replay.Reference != want {
		t.Fatalf("replay lost persisted DTO: %#v, want %#v (%s)", replay.Reference, want, c)
	}
	in.Operation, in.ReferenceID = uuid.NewString(), uuid.NewString()
	reused, c := j.st.SavePPReference(ctx, companyX, in, "ppref:canonical-second")
	if c != equipment.OK || reused.Reference == nil || *reused.Reference != want {
		t.Fatalf("reuse changed persisted DTO: %#v, want %#v (%s)", reused.Reference, want, c)
	}
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyY, access.OK)
	j.selectCompany(companyX, access.OK)
	listed, c := j.st.PPReferences(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID})
	if c != equipment.OK || len(listed.References) != 1 || listed.References[0] != want {
		t.Fatal("reopen list differs", listed, c)
	}
	deleted := equipment.PPInput{DeviceID: target.Device.ID, ReferenceID: want.ID, Operation: uuid.NewString()}
	if _, c = j.st.DeletePPReference(ctx, companyX, deleted, "ppref:canonical-delete"); c != equipment.OK {
		t.Fatal(c)
	}
	removed, c := j.st.SavePPReference(ctx, companyX, in, "ppref:canonical-second")
	if c != equipment.OK || removed.Effect != "removed" || removed.Reference != nil {
		t.Fatal("deleted receipt recreated reference", removed, c)
	}
}

func TestPPPoEAH03ReplayDatabaseFailureIsNotRemovedReference(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	o := ppSynthetic(t, j, target, 2)
	in := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString(), SavedAt: "2026-10-25T00:00:00Z"}
	if _, c := j.st.SavePPReference(ctx, companyX, in, "ppref:database-failure"); c != equipment.OK {
		t.Fatal(c)
	}
	// An actual SQL failure after receipt lookup must not become a successful removal.
	if _, e := j.st.unit.Exec("ALTER TABLE pppoe_references RENAME TO synthetic_references_unavailable"); e != nil {
		t.Fatal(e)
	}
	v, c := j.st.SavePPReference(ctx, companyX, in, "ppref:database-failure")
	if c != equipment.Unavailable || v.Effect == "removed" {
		t.Fatal("database failure was reported as removal", v, c)
	}
}
