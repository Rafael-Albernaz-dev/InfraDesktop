package ipc

import (
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"sync/atomic"
	"testing"
)

func stateComparisonFixture(t *testing.T) (*ppJourney, *equipment.PPObservation, *equipment.PPObservation) {
	var count, tick atomic.Int32
	count.Store(34)
	j := newPPJourney(t, sshfixture.Config{PPPoE: func() string {
		return sshfixture.PPPoETranscript(int(count.Load()), "7.26", int(tick.Add(1)))
	}})
	base := j.collect(uuid.NewString())
	j.call("pppoe.reference-save", equipment.PPInput{DeviceID: j.device, ObservationID: base.ID,
		ReferenceID: uuid.NewString(), Operation: uuid.NewString()}, foundation.OK)
	count.Store(17)
	target := j.collect(uuid.NewString())
	return j, base, target
}

func TestPPPoEComparisonFiltersStateBeforePagination(t *testing.T) {
	j, base, target := stateComparisonFixture(t)
	for _, state := range []string{"ONLINE", "OFFLINE"} {
		in := equipment.PPInput{DeviceID: j.device, BaseID: base.ID, TargetID: target.ID, Query: &equipment.PPQuery{State: state}}
		first := j.call("pppoe.compare", in, foundation.OK).Data.PPPoE
		if first.Total != 17 || len(first.Deltas) != 16 || first.Next == "" {
			t.Fatalf("%s filter not before pagination: %+v", state, first)
		}
		assertDeltaState(t, first.Deltas, state)
		in.Cursor = first.Next
		last := j.call("pppoe.compare", in, foundation.OK).Data.PPPoE
		if last.Total != 17 || len(last.Deltas) != 1 || last.Next != "" {
			t.Fatal("filtered comparison cursor", last)
		}
		assertDeltaState(t, last.Deltas, state)
	}
	changed := j.call("pppoe.compare", equipment.PPInput{DeviceID: j.device, BaseID: base.ID, TargetID: target.ID,
		Query: &equipment.PPQuery{State: "OFFLINE", Category: "changed"}}, foundation.OK).Data.PPPoE
	if changed.Total != 17 {
		t.Fatal("source filtering changed state-transition classification", changed.Total)
	}
	all := j.call("pppoe.compare", equipment.PPInput{DeviceID: j.device, BaseID: base.ID, TargetID: target.ID}, foundation.OK).Data.PPPoE
	if all.Total != 34 {
		t.Fatal("unfiltered comparison changed", all.Total)
	}
}
func assertDeltaState(t *testing.T, deltas []equipment.PPDelta, state string) {
	t.Helper()
	for _, delta := range deltas {
		if delta.Target == nil || delta.Target.State != state {
			t.Fatalf("other state escaped comparison: %+v", delta)
		}
	}
}
