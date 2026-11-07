package inventory

import (
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
	"time"
)

// PulseReader supplies ephemeral reachability without granting collection authority.
type PulseReader interface {
	Snapshot(string, time.Time) (pulse.Snapshot, bool)
}

func pulseTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
func (s *Service) withPulse(devices []Device, profile string, v *View) []Device {
	out := append([]Device(nil), devices...)
	var snapshot pulse.Snapshot
	if s.Pulse != nil {
		if snap, ok := s.Pulse.Snapshot(profile, s.Now()); ok {
			snapshot = snap
			v.PulseNetwork = string(snap.Network)
		}
	}
	for i := range out {
		d := &out[i]
		d.Pulse = &equipment.PulseStatus{}
		status, ok := snapshot.Devices[d.ID]
		if !d.Active || !ok || status.Address != d.Address {
			continue
		}
		d.Pulse = &equipment.PulseStatus{
			State: string(status.State), Since: pulseTime(status.Since),
			LastReply: pulseTime(status.LastReply), CheckedAt: pulseTime(status.CheckedAt),
		}
	}
	return out
}
func pulseMatches(d Device, state string) bool {
	if state == "" {
		return true
	}
	current := ""
	if d.Pulse != nil && d.Active {
		current = d.Pulse.State
	}
	if state == "unknown" {
		return current == ""
	}
	return current == state
}

func validPulseFilter(state string) bool {
	return state == "" || state == string(pulse.Online) || state == string(pulse.Offline) || state == "unknown"
}
