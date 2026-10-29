package storage

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func TestL06T3ContextPeakSevenDaysZeroUnknownAndQuota(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	vlan := 100
	publish := func(at time.Time, n int, known bool) *equipment.PPObservation {
		t.Helper()
		in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
		job := uuid.NewString()
		if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "peak", job); c != equipment.OK {
			t.Fatal(c)
		}
		o := monitoringObservation(target, at, uuid.NewString(), n)
		for i := 0; i < n; i++ {
			r := equipment.PPRow{ID: uuid.NewString(), RemoteID: fmt.Sprint(i), SessionID: fmt.Sprint(i), Login: fmt.Sprint("X", i)}
			if known {
				r.Context = &equipment.PPContext{Association: "associated", Service: "X-service", VLAN: &vlan}
			}
			o.Rows = append(o.Rows, r)
		}
		if c := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job}, o); c != equipment.OK {
			t.Fatal(c)
		}
		return o
	}
	// The new auxiliary table participates in base-priority quota trimming.
	if c := j.st.ppWrite(ctx, companyX, func(tx *sql.Tx) equipment.Code {
		if _, e := tx.ExecContext(ctx, "INSERT INTO device_context_samples VALUES(?,?,?,?,?,?,?)", target.Device.ID, fmt.Sprintf("%064d", 0), "synthetic", now.Unix(), "X-service", 100, 10); e != nil {
			t.Fatal(e)
		}
		if e := trimMonitoring(ctx, tx, 0, now.Unix()); e != nil {
			t.Fatal(e)
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	var n int
	j.st.unit.QueryRow("SELECT count(*) FROM device_context_samples").Scan(&n)
	if n != 0 {
		t.Fatal("new auxiliary table blocked base quota")
	}
	publish(now.Add(-8*24*time.Hour), 20, true)
	publish(now.Add(-6*24*time.Hour), 10, true)
	o := publish(now.Add(-time.Second), 4, true)
	groups := func(o *equipment.PPObservation) []equipment.PPGroup {
		v, c := j.st.PPContextCounts(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID})
		if c != equipment.OK {
			t.Fatal(c)
		}
		return v.Groups
	}
	g := groups(o)
	if len(g) != 1 || g[0].Count != 4 || g[0].Peak == nil || *g[0].Peak != 10 {
		t.Fatal("peak is eternal or includes OFFLINE", g)
	}
	o = publish(now, 0, true)
	g = groups(o)
	if len(g) != 1 || g[0].Count != 0 || g[0].Peak == nil || *g[0].Peak != 10 {
		t.Fatal("complete empty reading lost local drop", g)
	}
	o = publish(now.Add(time.Second), 3, false)
	g = groups(o)
	if len(g) != 1 || g[0].Count != 3 || g[0].Peak != nil {
		t.Fatal("unknown context invented zero/peak", g)
	}
}
func TestL06T3MigrationElevenAtomicPreservesTen(t *testing.T) {
	j, _ := ppStoreSetup(t)
	var before, after string
	j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations WHERE version<=10 ORDER BY version)").Scan(&before)
	if _, e := j.st.unit.Exec("DROP TABLE device_context_samples;DELETE FROM schema_migrations WHERE version=11"); e != nil {
		t.Fatal(e)
	}
	original := companyContextPeaksSchema
	defer func() { companyContextPeaksSchema = original }()
	companyContextPeaksSchema = original + "; invalid SQL;"
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e == nil {
		t.Fatal("bad011 accepted")
	}
	var n int
	j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='device_context_samples'").Scan(&n)
	if n != 0 {
		t.Fatal("partial011 escaped")
	}
	companyContextPeaksSchema = original
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations WHERE version<=10 ORDER BY version)").Scan(&after)
	if before != after {
		t.Fatal("001..010 changed")
	}
}
