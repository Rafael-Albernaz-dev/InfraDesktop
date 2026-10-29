package equipment

import (
	"testing"
	"time"
)

func TestL06T3CrossConcentratorFreshOverlap(t *testing.T) {
	now := time.Now()
	a := &PPObservation{Quality: PPQuality{Status: "complete"}, Provenance: PPProvenance{Started: PPTime(now.Add(-10 * time.Second)), Ended: PPTime(now.Add(-5 * time.Second))}}
	b := *a
	b.Provenance.Started = PPTime(now.Add(-8 * time.Second))
	b.Provenance.Ended = PPTime(now.Add(-3 * time.Second))
	if !ComparableConcentratorReadings(a, &b, now) {
		t.Fatal("fresh overlapping windows refused")
	}
	for _, delta := range []time.Duration{-20 * time.Minute, -20 * time.Second, time.Minute} {
		bad := b
		bad.Provenance.Started = PPTime(now.Add(delta))
		bad.Provenance.Ended = PPTime(now.Add(delta + time.Second))
		if ComparableConcentratorReadings(a, &bad, now) {
			t.Fatal("stale/disjoint/future accepted", delta)
		}
	}
}
