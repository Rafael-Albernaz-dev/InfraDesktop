package inventory

import (
	"time"

	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
)

type LiveReader interface {
	Snapshot(string, time.Time) (map[string]pulse.DeviceReading, bool)
}

func (s *Service) withLive(devices []Device, credentials []equipment.SealedCredential, profile string) []Device {
	if s.Live == nil {
		return devices
	}
	snapshot, ok := s.Live.Snapshot(profile, s.Now())
	if !ok {
		return devices
	}
	revisions := map[string]uint64{}
	for _, c := range credentials {
		revisions[c.Metadata.ID] = c.Metadata.Revision
	}
	for n := range devices {
		d := &devices[n]
		value, exists := snapshot[d.ID]
		stamp := pulse.ReadStamp(profile, equipment.PPConnection(*d), d.Revision, revisions[d.CredentialID])
		if !d.Active || !exists || value.Stamp != stamp {
			continue
		}
		live := &equipment.LiveReading{Fault: string(value.Fault)}
		if value.HasReading {
			cpu, clients, uptime := value.CPU, value.Clients, int64(value.Uptime/time.Second)
			live.CPU, live.Clients, live.UptimeSeconds, live.At = &cpu, &clients, &uptime, pulseTime(value.At)
		}
		d.Live = live
	}
	return devices
}
