package inventory_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
)

type runningBlockStore struct {
	mu    sync.Mutex
	state equipment.State
}

func (s *runningBlockStore) ReadEquipment(context.Context, string) (equipment.State, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, equipment.OK
}
func (s *runningBlockStore) UpdateEquipment(_ context.Context, _ string, f func(*equipment.State) equipment.Code) equipment.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	return f(&s.state)
}

type runningBlockVault struct{ renewalVault }

func (runningBlockVault) Available() bool { return true }

type runningBlockRemote struct {
	equipment.Remote
	entered chan struct{}
	release chan struct{}
}

func (r runningBlockRemote) Query(
	ctx context.Context, _ equipment.Device, _, _ string, check func() equipment.Code,
) (*equipment.Observation, equipment.Code) {
	close(r.entered)
	select {
	case <-r.release:
	case <-ctx.Done():
		return nil, equipment.Cancelled
	}
	if code := check(); code != equipment.OK {
		return nil, code
	}
	return &equipment.Observation{Source: "synthetic"}, equipment.OK
}

func TestBlockOnlyLeavesRunningQueryAbleToFinish(t *testing.T) {
	for _, mode := range []string{"edit", "bulk"} {
		t.Run(mode, func(t *testing.T) {
			id, credential := fixtureUUID(), fixtureUUID()
			device := equipment.Device{ID: id, Name: "Sintetico", Address: "192.0.2.1", Port: 22,
				Transport: equipment.TransportSSH, Role: equipment.RoleOther, Revision: 1, Active: true,
				Fingerprint: "synthetic", CredentialID: credential}
			store := &runningBlockStore{state: equipment.State{Company: "company", WrappedKey: []byte{1},
				Devices: []equipment.Device{device}, Credentials: []equipment.SealedCredential{
					{Metadata: equipment.Credential{ID: credential, Revision: 1}},
				}}}
			remote := runningBlockRemote{entered: make(chan struct{}), release: make(chan struct{})}
			service := inventory.New(renewalAuth{}, store, runningBlockVault{}, remote, renewalIDs{}, time.Now)
			defer service.Close()
			scope := access.Command{Profile: "company", ExpectedProfile: "company", Activation: "active", ExpectedActivation: "active"}
			start := equipment.Command{Op: "device.query-start", Scope: scope, ID: id, Revision: 1, Operation: fixtureUUID()}
			view, code := service.Handle(context.Background(), start)
			if code != equipment.OK || view.Job == nil {
				t.Fatal("start", code)
			}
			<-remote.entered
			update := equipment.Command{Scope: scope, ID: id, Revision: 1, Operation: fixtureUUID()}
			if mode == "edit" {
				update.Op = "inventory.update"
				update.Name = device.Name
				update.Address = device.Address
				update.Port = device.Port
				update.Transport = device.Transport
				update.Role = device.Role
				update.CredentialID = credential
				update.Block = "Torre sintetica"
			} else {
				block := "Torre sintetica"
				update.Op = "inventory.bulk-update"
				update.Bulk = &equipment.BulkUpdate{Block: &block, Items: []equipment.BulkItem{{ID: id, Revision: 1}}}
			}
			_, code = service.Handle(context.Background(), update)
			close(remote.release)
			if code != equipment.OK {
				t.Fatal("update", code)
			}
			service.WG.Wait()
			service.Mu.Lock()
			job := service.Jobs[view.Job.ID]
			state, result := job.View.State, job.View.Code
			service.Mu.Unlock()
			if state != "SUCCEEDED" || result != equipment.OK {
				t.Fatalf("query cancelled or failed: %s %s", state, result)
			}
			saved, _ := store.ReadEquipment(context.Background(), "company")
			if saved.Devices[0].Revision != 1 || saved.Devices[0].Block != "Torre sintetica" || saved.Devices[0].Result == nil {
				t.Fatal("block or result lost", saved.Devices[0])
			}
		})
	}
}
