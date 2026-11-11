package inventory_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
)

// slowDiskStore answers Unavailable to the first `failures` writes once armed,
// the way the SQLite adapter does when a write misses its budget.
type slowDiskStore struct {
	runningBlockStore
	armed    atomic.Bool
	failures atomic.Int32
	writes   atomic.Int32
}

func (s *slowDiskStore) UpdateEquipment(ctx context.Context, p string, f func(*equipment.State) equipment.Code) equipment.Code {
	if s.armed.Load() {
		s.writes.Add(1)
		if s.failures.Add(-1) >= 0 {
			return equipment.Unavailable
		}
	}
	return s.runningBlockStore.UpdateEquipment(ctx, p, f)
}

type answeringRemote struct {
	equipment.Remote
	store *slowDiskStore
}

func (r answeringRemote) Query(
	context.Context, equipment.Device, string, string, func() equipment.Code,
) (*equipment.Observation, equipment.Code) {
	r.store.armed.Store(true)
	return &equipment.Observation{Source: "synthetic"}, equipment.OK
}

func runQueryOnSlowDisk(t *testing.T, failures int32) (*slowDiskStore, equipment.Job) {
	t.Helper()
	id, credential := fixtureUUID(), fixtureUUID()
	device := equipment.Device{ID: id, Name: "Sintetico", Address: "192.0.2.1", Port: 22,
		Transport: equipment.TransportSSH, Role: equipment.RoleOther, Revision: 1, Active: true,
		Fingerprint: "synthetic", CredentialID: credential}
	store := &slowDiskStore{runningBlockStore: runningBlockStore{state: equipment.State{Company: "company",
		WrappedKey: []byte{1}, Devices: []equipment.Device{device},
		Credentials: []equipment.SealedCredential{{Metadata: equipment.Credential{ID: credential, Revision: 1}}}}}}
	store.failures.Store(failures)
	service := inventory.New(renewalAuth{}, store, runningBlockVault{}, answeringRemote{store: store}, renewalIDs{}, time.Now)
	service.PersistRetry = inventory.PersistRetryLimits{Window: 300 * time.Millisecond, Pause: 10 * time.Millisecond}
	defer service.Close()
	scope := access.Command{Profile: "company", ExpectedProfile: "company", Activation: "active", ExpectedActivation: "active"}
	view, code := service.Handle(context.Background(),
		equipment.Command{Op: "device.query-start", Scope: scope, ID: id, Revision: 1, Operation: fixtureUUID()})
	if code != equipment.OK || view.Job == nil {
		t.Fatal("start", code)
	}
	service.WG.Wait()
	service.Mu.Lock()
	defer service.Mu.Unlock()
	return store, service.Jobs[view.Job.ID].View
}

func TestQueryResultSurvivesAMomentarilySlowDisk(t *testing.T) {
	store, job := runQueryOnSlowDisk(t, 2)
	if job.State != "SUCCEEDED" || job.Code != equipment.OK {
		t.Fatalf("result lost to a slow disk: %s %s", job.State, job.Code)
	}
	saved, _ := store.ReadEquipment(context.Background(), "company")
	if saved.Devices[0].Result == nil {
		t.Fatal("successful query was not saved")
	}
	if store.writes.Load() < 3 {
		t.Fatal("write was not retried", store.writes.Load())
	}
}

func TestQueryOnABrokenDiskStillFailsHonestly(t *testing.T) {
	store, job := runQueryOnSlowDisk(t, 1<<30)
	if job.State != "FAILED" || job.Code != inventory.CodePersistenceFailed {
		t.Fatalf("broken disk not reported: %s %s", job.State, job.Code)
	}
	saved, _ := store.ReadEquipment(context.Background(), "company")
	if saved.Devices[0].Result != nil {
		t.Fatal("result reported as failed but saved")
	}
}

func TestPersistRetryStopsOnAnythingButUnavailable(t *testing.T) {
	service := inventory.New(renewalAuth{}, &runningBlockStore{}, runningBlockVault{}, nil, renewalIDs{}, time.Now)
	service.PersistRetry = inventory.PersistRetryLimits{Window: time.Second, Pause: time.Millisecond}
	calls := 0
	code := service.PersistWithRetry(time.Second, func(context.Context) equipment.Code {
		calls++
		return equipment.Limit
	})
	if code != equipment.Limit || calls != 1 {
		t.Fatal("a non-transient failure was retried", code, calls)
	}
	service.Close()
	calls = 0
	code = service.PersistWithRetry(time.Second, func(context.Context) equipment.Code {
		calls++
		return equipment.Unavailable
	})
	if code != equipment.Unavailable || calls != 1 {
		t.Fatal("a closing service kept retrying", code, calls)
	}
}
