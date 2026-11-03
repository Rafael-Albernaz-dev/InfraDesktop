package radio

import (
	"context"
	"testing"
)

type radioProfileStore struct{ radioLiveStore }

func (s radioProfileStore) RadioCurrent(_ context.Context, _ string, id string) (*RadioReading, *RadioProfile, Code) {
	if id == s.link.CPE {
		d := s.devices[id]
		return &RadioReading{Family: RadioUnsupported,
				Model:              "synthetic AC",
				Version:            "AC.v8",
				DeviceName:         "synthetic AC",
				Compatible:         true,
				DeviceID:           d.ID,
				Revision:           d.Revision,
				CredentialRevision: 1,
				Connection:         PPConnection(d)},
			&RadioProfile{Family: RadioUnsupported},
			OK
	}
	return nil, &RadioProfile{Confirmed: true, Family: RadioRouterOSWirelessV6}, OK
}
func TestRadioCostCannotCrossPerspective(t *testing.T) {
	s, q, in, store := radioHarness(t, &radioLiveRemote{})
	s.Store = radioProfileStore{store}
	if _, c := s.HandleRadio(context.Background(), q, in); c != RadioCodeLiveCostRequired {
		t.Fatal("two-sided proof reused for AP-only live", c)
	}
	for k, cost := range s.radioCosts {
		cost.APOnly = true
		s.radioCosts[k] = cost
	}
	s.Store = store
	if _, c := s.HandleRadio(context.Background(), q, in); c != RadioCodeLiveCostRequired {
		t.Fatal("AP-only proof reused for two-sided live", c)
	}
}
