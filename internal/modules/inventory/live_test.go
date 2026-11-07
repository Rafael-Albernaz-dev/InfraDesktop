package inventory

import (
	"testing"
	"time"

	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
)

type fixedLive struct {
	values map[string]pulse.DeviceReading
}

func (f fixedLive) Snapshot(string, time.Time) (map[string]pulse.DeviceReading, bool) {
	return f.values, true
}
func TestLiveProjectionIsEphemeralAndRequiresCurrentTarget(t *testing.T) {
	now := time.Now()
	device := Device{ID: "a", Active: true, CredentialID: "c", Revision: 2}
	credential := equipment.SealedCredential{Metadata: equipment.Credential{ID: "c", Revision: 3}}
	stamp := pulse.ReadStamp("p", equipment.PPConnection(device), 2, 3)
	values := map[string]pulse.DeviceReading{"a": {Stamp: stamp, Reading: pulse.Reading{CPU: 0, Clients: 0, Uptime: time.Hour},
		HasReading: true, At: now}}
	s := Service{Live: fixedLive{values}, Now: func() time.Time { return now }}
	stored := []Device{device}
	for _, change := range []string{"", "credential", "device", "company", "inactive"} {
		t.Run(change, func(t *testing.T) {
			d, c, profile := device, credential, "p"
			d, c, profile = changedLiveProjection(change, d, c)
			out := s.withPulse([]Device{d}, profile, &View{})
			out = s.withLive(out, []equipment.SealedCredential{c}, profile)
			if change != "" {
				if out[0].Live != nil {
					t.Fatal("outdated target reading published")
				}
				return
			}
			assertLiveZeroes(t, out[0].Live)
		})
	}
	values["a"] = pulse.DeviceReading{Stamp: stamp, Fault: pulse.ReadRefused}
	out := s.withLive(s.withPulse(stored, "p", &View{}), []equipment.SealedCredential{credential}, "p")
	if out[0].Live.Fault != string(pulse.ReadRefused) || out[0].Live.CPU != nil || out[0].Live.At != "" {
		t.Fatal("missing measurement projected as zero")
	}
	if stored[0].Live != nil {
		t.Fatal("projection changed stored device")
	}

}

func changedLiveProjection(change string, d Device, c equipment.SealedCredential) (Device, equipment.SealedCredential, string) {
	profile := "p"
	switch change {
	case "credential":
		c.Metadata.Revision++
	case "device":
		d.Revision++
	case "company":
		profile = "q"
	case "inactive":
		d.Active = false
	}
	return d, c, profile
}
func assertLiveZeroes(t *testing.T, got *equipment.LiveReading) {
	t.Helper()

	if got == nil || got.CPU == nil || *got.CPU != 0 || got.Clients == nil || *got.Clients != 0 || *got.UptimeSeconds != 3600 {
		t.Fatal("valid zero measurements lost")
	}
}
