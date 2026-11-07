package composition

import (
	"context"
	"time"

	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
)

type liveStore interface {
	equipment.Store
	ReadPPTarget(context.Context, string, string, bool) (equipment.PPTarget, equipment.Code)
	RadioCurrent(context.Context, string, string) (*equipment.RadioReading, *equipment.RadioProfile, equipment.Code)
}
type liveInventory struct {
	access *access.Service
	store  liveStore
}

func (i liveInventory) ReadTargets(ctx context.Context) (string, []pulse.ReadTarget, bool) {
	profile, ok := i.access.ActiveProfile(ctx)
	if !ok {
		return "", nil, false
	}
	state, code := i.store.ReadEquipment(ctx, profile)
	if code != equipment.OK {
		return "", nil, false
	}
	credentials := map[string]uint64{}
	for _, c := range state.Credentials {
		credentials[c.Metadata.ID] = c.Metadata.Revision
	}
	targets := []pulse.ReadTarget{}
	for _, d := range state.Devices {
		revision, exists := credentials[d.CredentialID]
		if !exists || !i.eligible(ctx, profile, d) {
			continue
		}
		targets = append(targets, pulse.ReadTarget{ID: d.ID, Role: d.Role,
			Stamp: pulse.ReadStamp(profile, equipment.PPConnection(d), d.Revision, revision)})
	}
	return profile, targets, true
}
func (i liveInventory) eligible(ctx context.Context, profile string, d equipment.Device) bool {
	if !d.Active || d.Transport != equipment.TransportSSH || d.CredentialID == "" || d.Fingerprint == "" {
		return false
	}
	switch d.Role {
	case equipment.RoleConcentrator:
		return true
	case equipment.RoleAP:
		// ignored: Eligibility uses the confirmed profile, not the saved reading.
		_, p, code := i.store.RadioCurrent(ctx, profile, d.ID)
		return code == equipment.OK && p != nil && p.Confirmed && p.Family == equipment.RadioRouterOSWirelessV6
	default:
		return false
	}
}

type liveDialer struct {
	inventory liveInventory
	vault     equipment.Vault
}

func (d liveDialer) Dial(ctx context.Context, want pulse.ReadTarget) (pulse.ReadSession, pulse.ReadFault) {
	profile, ok := d.inventory.access.ActiveProfile(ctx)
	if !ok {
		return nil, pulse.ReadLost
	}
	target, code := d.inventory.store.ReadPPTarget(ctx, profile, want.ID, true)
	if code != equipment.OK || target.Company != profile {
		return nil, pulse.ReadLost
	}
	actual := pulse.ReadStamp(profile, equipment.PPConnection(target.Device), target.Device.Revision, target.Credential.Metadata.Revision)
	if actual != want.Stamp || target.Device.Role != want.Role || !d.inventory.eligible(ctx, profile, target.Device) {
		return nil, pulse.ReadLost
	}
	if !d.vault.Ready(target.Installation, target.Company, target.Wrapped) {
		return nil, pulse.ReadLost
	}
	secret, code := d.vault.Open(target.Installation, target.Company, target.Credential.Metadata.ID,
		target.Credential.Metadata.Revision, target.Credential.Envelope, target.Wrapped)
	if code != equipment.OK {
		return nil, pulse.ReadLost
	}
	session, fault := routerssh.OpenLive(ctx, target.Device, target.Credential.Metadata.User, secret)
	secret = ""
	return session, fault
}
func composeLiveReader(server *ipc.Server, inv liveInventory, vault equipment.Vault, ping *pulse.Collector) *pulse.ReadCollector {
	reachable := func(profile, id string, now time.Time) bool {
		snapshot, ok := ping.Snapshot(profile, now)
		return ok && snapshot.Devices[id].State == pulse.Online
	}
	collector := pulse.NewReadCollector(inv, liveDialer{inv, vault}, reachable, 4)
	server.StartBackground(collector.Run)
	return collector
}
