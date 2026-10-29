package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
	"time"
)

func TestL06CReadAtMutationQuota(t *testing.T) {
	j, target := ppStoreSetup(t)
	tx, e := j.st.unit.Begin()
	if e != nil {
		t.Fatal(e)
	}
	var count int
	if e = tx.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&count); e != nil {
		t.Fatal(e)
	}
	for i := count; i < equipment.MaxOperations; i++ {
		if _, e = tx.Exec("INSERT INTO equipment_operations VALUES(?,?,?)", fmt.Sprint(i), "mutation", target.Device.ID); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(); e != nil {
		t.Fatal(e)
	}

	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	_, _, c := j.st.ReservePPCollection(context.Background(), companyX, in, "read", uuid.NewString())
	if c != equipment.OK {
		t.Fatal("reading consumes permanent mutation quota", c)
	}
	var after int
	if e = j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&after); e != nil || after != equipment.MaxOperations {
		t.Fatal("read changed mutation ledger", after, e)
	}
	if _, c = j.st.DeletePPReference(context.Background(), companyX, equipment.PPInput{DeviceID: target.Device.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}, "mutation"); c != equipment.Limit {
		t.Fatal("mutation bypassed quota", c)
	}

}

func TestL06CMigrationPreservesMutationsAndAttempts(t *testing.T) {
	j, target := ppStoreSetup(t)
	dropLaterMonitoringFixtureSchema(t, j)
	// Reconstruct only schema6 in this disposable synthetic company.
	if _, e := j.st.unit.Exec("DROP TABLE pppoe_attempts; DROP TABLE pppoe_diagnostic_attempts; DROP TABLE equipment_read_receipt_migration; ALTER TABLE devices DROP COLUMN role; DROP INDEX pppoe_rows_service_vlan; DELETE FROM schema_migrations WHERE version>=7"); e != nil {
		t.Fatal(e)
	}
	for _, src := range []string{companyPPPoESchema, companyPPDiagnosticSchema} {
		at := strings.Index(src, "CREATE TABLE pppoe_attempts")
		if at < 0 {
			at = strings.Index(src, "CREATE TABLE pppoe_diagnostic_attempts")
		}
		if _, e := j.st.unit.Exec(src[at:]); e != nil {
			t.Fatal(e)
		}
	}
	op := uuid.NewString()
	job := uuid.NewString()
	stamp := fmt.Sprintf("ppref:%q", []string{"pppoe.collect-start", target.Device.ID, fmt.Sprint(target.Device.Revision), "", ""})
	if _, e := j.st.unit.Exec("INSERT INTO equipment_operations VALUES(?,?,?)", op, stamp, job); e != nil {
		t.Fatal(e)
	}
	if _, e := j.st.unit.Exec("INSERT INTO equipment_operations VALUES('unknown','contains pppoe.collect-start but is not canonical','keep')"); e != nil {
		t.Fatal(e)
	}
	if _, e := j.st.unit.Exec("INSERT INTO pppoe_attempts VALUES(?,?,?,?,?)", target.Device.ID, equipment.PPScope, op, job, `{"state":"RUNNING"}`); e != nil {
		t.Fatal(e)
	}
	var before int
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&before)
	original := companySessionsOfflineSchema
	defer func() { companySessionsOfflineSchema = original }()
	companySessionsOfflineSchema = original + "; invalid syntax;"
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e == nil {
		t.Fatal("broken007 accepted")
	}
	var n int
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&n)
	if n != before {
		t.Fatal("failed migration removed receipts")
	}
	companySessionsOfflineSchema = original
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&n)
	if n != before-1 {
		t.Fatal("mutation/unknown removed", before, n)
	}
	var removed int
	j.st.unit.QueryRow("SELECT removed_count FROM equipment_read_receipt_migration WHERE class='sessions'").Scan(&removed)
	if removed != 1 {
		t.Fatal("reclassification not registered")
	}
	var keptJob string
	if e := j.st.unit.QueryRow("SELECT job_id FROM pppoe_attempts").Scan(&keptJob); e != nil || keptJob != job {
		t.Fatal("attempt lost", e)
	}
	if _, e := j.st.unit.Exec("UPDATE pppoe_attempts SET operation_id='read-without-ledger'"); e != nil {
		t.Fatal("read still depends on mutation ledger", e)
	}
}

func TestL06COfflinePersistedAcrossReopenAndReference(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	if c := j.st.UpdateEquipment(ctx, companyX, func(st *equipment.State) equipment.Code { st.Devices[0].Role = "concentrator"; return equipment.OK }); c != equipment.OK {
		t.Fatal(c)
	}
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	publish := func(rows []equipment.PPRow) *equipment.PPObservation {
		in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
		job := uuid.NewString()
		if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "fixture", job); c != equipment.OK {
			t.Fatal(c)
		}
		o := &equipment.PPObservation{ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Provenance: equipment.PPProvenance{Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1", Connection: "c", Started: equipment.PPTime(at), Ended: equipment.PPTime(at.Add(time.Second))}, Rows: rows}
		if c := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job}, o); c != equipment.OK {
			t.Fatal(c)
		}
		at = at.Add(time.Minute)
		return o
	}
	row := equipment.PPRow{ID: uuid.NewString(), RemoteID: "*1", SessionID: "1", Login: "reader", Address: "198.51.100.1"}
	publish([]equipment.PPRow{row})
	offline := publish(nil)
	st, code := j.st.ReadEquipment(ctx, companyX)
	if code != equipment.OK || st.Devices[0].PPPoE == nil || st.Devices[0].PPPoE.Online != 0 || st.Devices[0].PPPoE.Offline != 1 || st.Devices[0].PPPoE.At != offline.Provenance.Ended {
		t.Fatal("dashboard summary not wired to persisted reading", code)
	}
	ref := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: offline.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}
	if _, c := j.st.SavePPReference(ctx, companyX, ref, "keep-offline"); c != equipment.OK {
		t.Fatal(c)
	}
	// Close and reopen the actual synthetic active SQLite unit.
	if e := j.st.unit.Close(); e != nil {
		t.Fatal(e)
	}
	db, e := j.st.openUnit(companyX)
	if e != nil {
		t.Fatal(e)
	}
	j.st.unit = db
	page, c := j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: offline.ID})
	if c != equipment.OK || len(page.Rows) != 1 || page.Rows[0].State != "OFFLINE" {
		t.Fatal("offline not durable", page, c)
	}
	row.ID = uuid.NewString()
	row.SessionID = "2"
	row.RemoteID = "*2"
	online := publish([]equipment.PPRow{row})
	if online.Online != 1 || online.Offline != 0 {
		t.Fatal("reconnection failed", online)
	}
	page, c = j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: offline.ID})
	if c != equipment.OK || page.Rows[0].State != "OFFLINE" {
		t.Fatal("reference changed", page, c)
	}
}

func TestL06CEquipmentWritesOnlyChangedRecords(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	if _, e := j.st.unit.Exec("CREATE TRIGGER no_credential_rewrite BEFORE UPDATE ON credentials BEGIN SELECT RAISE(ABORT,'unchanged credential rewritten'); END"); e != nil {
		t.Fatal(e)
	}
	before, c := j.st.ReadEquipment(ctx, companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	if c = j.st.UpdateEquipment(ctx, companyX, func(st *equipment.State) equipment.Code { st.Devices[0].Name = "changed"; return equipment.OK }); c != equipment.OK {
		t.Fatal("rewrote unchanged credential", c)
	}
	after, c := j.st.ReadEquipment(ctx, companyX)
	a, _ := json.Marshal(before.Credentials)
	b, _ := json.Marshal(after.Credentials)
	if c != equipment.OK || string(a) != string(b) || after.Devices[0].ID != target.Device.ID || after.Devices[0].Name != "changed" {
		t.Fatal("unexpected delta", c)
	}
}

func TestL06COfflineByteBudgetNeverDisplacesOnline(t *testing.T) {
	o := &equipment.PPObservation{Online: 1, Offline: equipment.PPMaxRows - 1, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Rows: []equipment.PPRow{{ID: "live", State: "ONLINE", Login: "present"}}}
	for i := 0; i < equipment.PPMaxRows-1; i++ {
		o.Rows = append(o.Rows, equipment.PPRow{ID: fmt.Sprint(i), State: "OFFLINE", Login: strings.Repeat("x", 256), Caller: strings.Repeat("c", 128), RemoteID: strings.Repeat("1", 32), SessionID: strings.Repeat("s", 64), Address: "ffff:ffff:ffff:ffff:ffff:ffff:192.0.2.1", AbsentStart: "2026-10-26T00:00:00Z", AbsentEnd: "2026-10-26T00:01:00Z"})
	}
	if !ppWithinBudget(o) || o.Rows[0].ID != "live" || o.Online != 1 || o.Offline >= equipment.PPMaxRows-1 || o.Bytes > equipment.PPObservationBytes {
		t.Fatal("offline retention blocks online publication", o.Bytes, o.Offline)
	}
}

func TestL06CMeasurePublication2048(t *testing.T) {
	j, target := ppStoreSetup(t)
	for _, kind := range []string{"sessions", "catalog"} {
		for pass := 0; pass < 2; pass++ {
			last := time.Now()
			j.st.checkpoint = func(phase string) {
				if strings.HasPrefix(phase, "pp-phase-") || phase == "before-pppoe-commit" {
					now := time.Now()
					t.Logf("kind=%s pass=%d until=%s elapsed=%s", kind, pass, phase, now.Sub(last))
					last = now
				}
			}
			if kind == "catalog" {
				pdSynthetic(t, j, target, equipment.PDCatalog, 2048)
			} else {
				ctx := context.Background()
				job := uuid.NewString()
				in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
				if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "measure", job); c != equipment.OK {
					t.Fatal(c)
				}
				o := &equipment.PPObservation{Provenance: equipment.PPProvenance{Started: equipment.PPTime(time.Now()), Ended: equipment.PPTime(time.Now())}, ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}}
				for i := 0; i < 2048; i++ {
					o.Rows = append(o.Rows, equipment.PPRow{ID: uuid.NewString(), RemoteID: fmt.Sprintf("*%x", i), Login: "x"})
				}
				pub, cancel := context.WithTimeout(ctx, equipment.PPPersistenceBudget)
				c := j.st.FinishPPCollection(pub, companyX, target, job, equipment.PPAttempt{JobID: job}, o)
				cancel()
				if c != equipment.OK {
					t.Fatalf("publication failed kind=%s pass=%d bytes=%d code=%s", kind, pass, o.Bytes, c)
				}
				t.Logf("kind=%s pass=%d bytes=%d", kind, pass, o.Bytes)
			}
			j.st.checkpoint = nil
		}
	}
}
