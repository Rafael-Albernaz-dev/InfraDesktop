package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"testing"
)

type ppReviewAuth struct{}

func (ppReviewAuth) WithAuthorization(_ context.Context, _ uint32, _ access.Command, fn func(uint64) Code) Code {
	return fn(1)
}

type ppReviewStore struct{ PPStore }

func (ppReviewStore) ReadEquipment(context.Context, string) (State, Code) { return State{}, OK }
func (ppReviewStore) UpdateEquipment(_ context.Context, _ string, fn func(*State) Code) Code {
	return fn(&State{})
}

type ppReviewIDs struct{}

func (ppReviewIDs) ID() (string, Code) { return "", Unavailable }
func TestPPPoEReviewCancelKindBeforeEffect(t *testing.T) {
	for _, kind := range []string{"device.query-start", "trust.inspect", "pppoe.collect-start"} {
		t.Run(kind, func(t *testing.T) {
			s := New(ppReviewAuth{}, ppReviewStore{}, nil, nil, ppReviewIDs{}, nil)
			scope := access.Command{Profile: "x", ExpectedProfile: "x", Activation: "a", ExpectedActivation: "a"}
			q := Command{Op: kind, Authority: 1, Scope: scope}
			cancelled := false
			s.jobs["job"] = &runningJob{view: Job{ID: "job", Kind: kind, State: "RUNNING"}, command: q, cancel: func() { cancelled = true }}
			q.Op = "pppoe.collect-cancel"
			_, c := s.HandlePP(context.Background(), q, PPInput{JobID: "job"})
			if kind != "pppoe.collect-start" {
				if c != Invalid || cancelled || s.jobs["job"].view.State != "RUNNING" {
					t.Fatal("foreign kind changed before refusal", c, cancelled)
				}
			} else if c != OK || !cancelled {
				t.Fatal("legitimate cancel rejected", c)
			}
		})
	}
}
func TestPPPoEReviewSameObservationAndVersion(t *testing.T) {
	a := &PPObservation{ID: "a", DeviceID: "d", Scope: PPScope, Quality: PPQuality{Status: "complete"}, Provenance: PPProvenance{Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1", Version: "7.26"}, Rows: []PPRow{{ID: "r1", RemoteID: "*1", Login: "dup"}, {ID: "r2", RemoteID: "*1", Login: "dup"}}}
	ds, c := ComparePP(a, a)
	if c != OK || len(ds) != 2 {
		t.Fatal(ds, c)
	}
	for _, d := range ds {
		if d.Category != "unchanged" || d.BaseRow != d.TargetRow {
			t.Fatal("same observation lost local identity", ds)
		}
	}
	b := *a
	b.ID = "b"
	b.Provenance.Version = "7.27"
	if _, c := ComparePP(a, &b); c != ObservationIncompatible {
		t.Fatal("firmware version mismatch accepted", c)
	}
}

func TestPPPoEReviewTimestampFractionOrdering(t *testing.T) {
	for _, pair := range [][2]string{{"2026-10-25T00:00:00Z", "2026-10-25T00:00:00.001Z"}, {"2026-10-25T00:00:00.1Z", "2026-10-25T00:00:00.11Z"}} {
		a := &PPObservation{ID: "a", DeviceID: "d", Scope: PPScope, Quality: PPQuality{Status: "complete"}, Provenance: PPProvenance{Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1", Started: pair[0]}}
		b := *a
		b.ID = "b"
		b.Provenance.Started = pair[1]
		if _, c := ComparePP(a, &b); c != OK {
			t.Fatal("later fractional timestamp rejected", c)
		}
		if _, c := ComparePP(&b, a); c != Invalid {
			t.Fatal("real inverted chronology accepted", c)
		}
	}
}
