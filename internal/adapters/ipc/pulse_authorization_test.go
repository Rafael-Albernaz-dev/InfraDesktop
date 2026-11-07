package ipc

import (
	"bytes"
	"context"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/pulse"
	"infradesktop/internal/modules/radio"
	"os"
	"testing"
	"time"
)

type ipcPulse struct{ id string }

func (p ipcPulse) Snapshot(string, time.Time) (pulse.Snapshot, bool) {
	return pulse.Snapshot{Network: pulse.NetworkOK, Devices: map[string]pulse.DeviceStatus{
		p.id: {State: pulse.Online, Address: "127.0.0.1", CheckedAt: time.Now()},
	}}, true
}
func TestInventoryPulseNeverPublishedWithoutSession(t *testing.T) {
	j := newPPJourney(t)
	provider := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{8}, 32), nil })
	inv := inventory.New(j.auth, j.store, provider, nil, testEquipmentIDs{}, time.Now)
	inv.Pulse = ipcPulse{j.device}
	AttachEquipment(j.server, &equipment.Service{Handler: radio.NewComposite(inv, nil, nil, nil)})
	r := j.client.Call(context.Background(), Request{Op: "inventory.list", ExpectedProfile: j.client.profile,
		ExpectedActivation: j.client.activation, Equipment: &EquipmentInput{State: "ONLINE"}})
	if r.Code != foundation.OK || r.Data == nil || len(r.Data.Equipment.Devices) != 1 ||
		r.Data.Equipment.Devices[0].Pulse == nil || r.Data.Equipment.Devices[0].Pulse.State != "ONLINE" {
		t.Fatal("authorized pulse missing", r.Code)
	}
	q := Request{Op: "inventory.list", Capability: j.server.capability, Profile: j.client.profile, Activation: j.client.activation,
		ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, Equipment: &EquipmentInput{}}
	for _, session := range []string{"", "expired-session"} {
		q.Session = session
		denied := j.server.dispatchEquipment(context.Background(), uint32(os.Getuid()), q, "synthetic-owner")
		if denied.Code != foundation.Unauthenticated || denied.Data != nil {
			t.Fatal("pulse exposed without session", denied.Code)
		}
	}
	if j.peer.Auth.Load() != 0 || j.peer.Handshakes.Load() != 0 {
		t.Fatal("pulse list contacted SSH")
	}
}
