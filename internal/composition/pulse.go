package composition

import (
	"context"
	"net/netip"

	"infradesktop/internal/adapters/icmpprobe"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
)

type pulseInventory struct {
	access *access.Service
	store  equipment.Store
}

func (i pulseInventory) Targets(ctx context.Context) (string, []pulse.Target, bool) {
	profile, ok := i.access.ActiveProfile(ctx)
	if !ok {
		return "", nil, false
	}
	state, code := i.store.ReadEquipment(ctx, profile)
	if code != equipment.OK {
		return "", nil, false
	}
	targets := []pulse.Target{}
	for _, d := range state.Devices {
		// ignored: Only validation is needed; parsed address is not retained.
		if _, err := netip.ParseAddr(d.Address); d.Active && err == nil {
			targets = append(targets, pulse.Target{ID: d.ID, Address: d.Address})
		}
	}
	return profile, targets, true
}
func composePulse(server *ipc.Server, service *access.Service, store equipment.Store) *pulse.Collector {
	collector := pulse.NewCollector(pulseInventory{service, store}, icmpprobe.Probe{}, 16)
	server.StartBackground(collector.Run)
	return collector
}
