package inventory

import (
	"fmt"
	"infradesktop/internal/modules/pulse"
	"testing"
	"time"
)

type fixedPulse struct{ snapshot pulse.Snapshot }

func (p fixedPulse) Snapshot(string, time.Time) (pulse.Snapshot, bool) { return p.snapshot, true }
func TestPulseFilterRunsAcrossWholeInventoryBeforePagination(t *testing.T) {
	now := time.Now()
	devices := make([]Device, 500)
	statuses := map[string]pulse.DeviceStatus{}
	for i := range devices {
		id := fmt.Sprint(i)
		devices[i] = Device{ID: id, Active: true}
		state := pulse.Online
		if i >= 480 {
			state = pulse.Offline
		}
		statuses[id] = pulse.DeviceStatus{State: state, CheckedAt: now}
	}
	devices[499].Active = false
	service := Service{Now: func() time.Time { return now }, Pulse: fixedPulse{pulse.Snapshot{Devices: statuses, Network: pulse.NetworkOK}}}
	for _, tc := range []struct {
		state        string
		cursor, want int
		first        string
	}{
		{"ONLINE", 0, PageSize, "0"}, {"OFFLINE", 0, PageSize, "480"},
		{"OFFLINE", 16, 3, "496"}, {"unknown", 0, 1, "499"}, {"OFFLINE", 32, 0, ""}, {"ONLINE", int(^uint(0) >> 1), 0, ""},
	} {
		var view View
		code := service.listDevices(&State{Devices: devices}, Command{State: tc.state, Cursor: tc.cursor, IncludeInactive: true}, &view)
		if code != OK || len(view.Devices) != tc.want || (tc.want > 0 && view.Devices[0].ID != tc.first) {
			t.Fatalf("%s cursor %d: code %s len %d", tc.state, tc.cursor, code, len(view.Devices))
		}
		if view.PulseNetwork != string(pulse.NetworkOK) {
			t.Fatal("network missing")
		}
	}
	if devices[0].Pulse != nil {
		t.Fatal("ephemeral pulse mutated stored inventory")
	}
	if validateInventoryFilters(Command{State: "bogus"}) != Invalid {
		t.Fatal("open state filter")
	}
	// Inactive equipment cannot retain an ONLINE state even if the source reports one.
	view := View{}
	out := service.withPulse(devices, "p", &view)
	if out[499].Pulse.State != "" {
		t.Fatal("inactive exposed pulse")
	}
}

func TestChangedAddressDoesNotShowPulseFromPreviousAddress(t *testing.T) {
	now := time.Now()
	service := Service{Now: func() time.Time { return now }, Pulse: fixedPulse{pulse.Snapshot{
		Network: pulse.NetworkOK, Devices: map[string]pulse.DeviceStatus{"a": {Address: "192.0.2.1", State: pulse.Online, CheckedAt: now}},
	}}}
	view := View{}
	devices := service.withPulse([]Device{{ID: "a", Address: "192.0.2.2", Active: true}}, "p", &view)
	if devices[0].Pulse.State != "" {
		t.Fatal("old address pulse published", devices[0].Pulse)
	}
}
