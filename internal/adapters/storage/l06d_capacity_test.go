package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

// Same stable context as the S1 measurement, with independent base snapshots.
func capacityObservation(n int) *equipment.PPObservation {
	o := &equipment.PPObservation{ID: uuid.NewString(), Scope: equipment.PPScope, Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}}
	vlan := 100
	for i := 0; i < n; i++ {
		o.Rows = append(o.Rows, equipment.PPRow{ID: uuid.NewString(), Login: "synthetic", State: "ONLINE", Context: &equipment.PPContext{Association: "associated", VLAN: &vlan, Service: "service", Profile: "profile", SecretState: "active"}})
	}
	o.Online = n
	return o
}

func TestL06DContextYieldsBeforeOfflineAndRowLimit(t *testing.T) {
	o := capacityObservation(1600)
	o.Rows[1599].State = "OFFLINE"
	o.Rows[1599].AbsentStart = "2026-10-27T00:00:00Z"
	o.Rows[1599].AbsentEnd = "2026-10-27T00:01:00Z"
	o.Online, o.Offline = 1599, 1
	if !ppWithinBudget(o) || len(o.Rows) != 1600 || o.Offline != 1 || o.Rows[1599].AbsentEnd == "" {
		t.Fatal("optional context displaced retained absence")
	}
	o = capacityObservation(1)
	o.Rows[0].Context.Profile = strings.Repeat("c", equipment.PPRowBytes)
	if !ppWithinBudget(o) || o.Rows[0].Context != nil {
		t.Fatal("context-only row excess rejected base")
	}
	o = capacityObservation(1)
	o.Rows[0].Login = strings.Repeat("b", equipment.PPRowBytes)
	if ppWithinBudget(o) {
		t.Fatal("oversized base accepted")
	}
	o = capacityObservation(2)
	o.Rows[1].ID = o.Rows[0].ID
	if ppWithinBudget(o) {
		t.Fatal("duplicate identity accepted")
	}
}

func TestL06DContextCapacityPublicationReopen(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	var ledgerBefore int
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&ledgerBefore)
	for _, n := range []int{1000, 1600, 2048} {
		job := uuid.NewString()
		in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
		if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "capacity", job); c != equipment.OK {
			t.Fatal(c)
		}
		o := capacityObservation(n)
		o.DeviceID = target.Device.ID
		o.Provenance.Started, o.Provenance.Ended = equipment.PPTime(time.Now()), equipment.PPTime(time.Now())
		pub, cancel := context.WithTimeout(ctx, equipment.PPPersistenceBudget)
		start := time.Now()
		c := j.st.FinishPPCollection(pub, companyX, target, job, equipment.PPAttempt{JobID: job, State: "SUCCEEDED", Code: equipment.OK, ObservationID: o.ID}, o)
		cancel()
		t.Logf("atomic rows=%d elapsed=%s code=%s bytes=%d", n, time.Since(start), c, o.Bytes)
		if c != equipment.OK {
			t.Fatal("context rejected base publication", c)
		}
		page, c := j.st.PPPage(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID})
		if c != equipment.OK || page.Total != n || len(page.Rows) != equipment.PageSize || page.Rows[0].Duplicates != n || page.Current.ContextOmitted != (n > 1000) {
			t.Fatal("stored complete base/marker/duplicates lost", c)
		}
		groups, c := j.st.PPContextCounts(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID})
		if c != equipment.OK || groups.Current.ContextOmitted && len(groups.Groups) != 0 {
			t.Fatal("budget omission became known grouping", c)
		}
	}
	if err := j.st.unit.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := j.st.openUnit(companyX)
	if err != nil {
		t.Fatal(err)
	}
	j.st.unit = db
	v, c := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || v.Current.Count != 2048 || !v.Current.ContextOmitted || v.Attempt.State != "SUCCEEDED" {
		t.Fatal("reopen/attempt lost", c)
	}
	var ledgerAfter int
	j.st.unit.QueryRow("SELECT count(*) FROM equipment_operations").Scan(&ledgerAfter)
	if ledgerAfter != ledgerBefore {
		t.Fatal("reading consumed mutation receipt")
	}
}

func capacityBytes(o *equipment.PPObservation) int {
	n := 0
	for _, row := range o.Rows {
		b, _ := json.Marshal(row)
		n += len(b)
	}
	for i := 0; i < 16; i++ {
		b, _ := json.Marshal(o)
		if o.Bytes == n+len(b) {
			return o.Bytes
		}
		o.Bytes = n + len(b)
	}
	panic("metadata size did not converge")
}

func TestL06DOptionalContextCapacity(t *testing.T) {
	for _, n := range []int{1000, 1600, 2048} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			o := capacityObservation(n)
			withState, _ := json.Marshal(o.Rows[0])
			oldRow := o.Rows[0]
			oldContext := *oldRow.Context
			oldContext.SecretState = ""
			oldRow.Context = &oldContext
			withoutState, _ := json.Marshal(oldRow)
			t.Logf("enum bytes/row=%d rows=%d", len(withState)-len(withoutState), n)
			enrichedBytes := capacityBytes(o)
			base := capacityObservation(n)
			for i := range base.Rows {
				base.Rows[i].Context = nil
			}
			baseBytes := capacityBytes(base)
			before := append([]equipment.PPRow(nil), o.Rows...)
			fits := ppWithinBudget(o)
			t.Logf("rows=%d enriched=%d base=%d final=%d cap=%d published=%v", n, enrichedBytes, baseBytes, o.Bytes, equipment.PPObservationBytes, fits)
			if !fits || len(o.Rows) != n || o.Online != n {
				t.Fatal("optional context rejected complete base sessions")
			}
			for i, row := range o.Rows {
				got := row
				got.Context = before[i].Context
				if got != before[i] {
					t.Fatal("base session changed while omitting optional context")
				}
			}
			meta, _ := json.Marshal(o)
			var fields map[string]any
			json.Unmarshal(meta, &fields)
			if enrichedBytes > equipment.PPObservationBytes {
				if fields["context_omitted"] != true || o.Rows[0].Context != nil {
					t.Fatal("budget omission is not explicit")
				}
			} else if o.Rows[0].Context == nil || fields["context_omitted"] == true {
				t.Fatal("legitimate context was discarded")
			}
			if o.Bytes != capacityBytes(o) || o.Bytes > equipment.PPObservationBytes {
				t.Fatal("budget does not match final serialized payload")
			}
		})
	}
}
