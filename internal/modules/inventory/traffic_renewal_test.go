package inventory_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/pppoe"
)

type renewalAuth struct{}

func (renewalAuth) WithAuthorization(_ context.Context, _ uint32, _ access.Command, f func(uint64) equipment.Code) equipment.Code {
	return f(1)
}

type renewalIDs struct{}

func (renewalIDs) ID() (string, equipment.Code) { return uuid.NewString(), equipment.OK }

type renewalVault struct{ equipment.Vault }

func (renewalVault) Ready(string, string, []byte) bool { return true }
func (renewalVault) Open(string, string, string, uint64, []byte, []byte) (string, equipment.Code) {
	return "synthetic", equipment.OK
}

type renewalRemote struct{ equipment.Remote }

func (renewalRemote) ReadPP(
	_ context.Context, _ equipment.Device, _, _, scope string, _ *equipment.PPReadTarget,
	publish func(*equipment.PPReadResult) equipment.Code, _ func() equipment.Code,
) (*equipment.PPReadResult, equipment.Code) {
	result := &equipment.PPReadResult{Scope: scope, Rows: []equipment.PPReadRow{}, PayloadBytes: 100, MetadataBytes: 100}
	if code := publish(result); code != equipment.OK {
		return nil, code
	}
	return result, equipment.OK
}

type renewalStore struct {
	equipment.PPStore
	target      equipment.PPTarget
	observation *equipment.PPObservation
	row         equipment.PPRow
	writes      atomic.Int64
}

func (s *renewalStore) ReadEquipment(context.Context, string) (equipment.State, equipment.Code) {
	return equipment.State{}, equipment.OK
}
func (s *renewalStore) UpdateEquipment(context.Context, string, func(*equipment.State) equipment.Code) equipment.Code {
	s.writes.Add(1)
	return equipment.OK
}
func (s *renewalStore) ReadPPTarget(context.Context, string, string, bool) (equipment.PPTarget, equipment.Code) {
	return s.target, equipment.OK
}
func (s *renewalStore) PPCurrent(context.Context, string, string) (equipment.PPView, equipment.Code) {
	return equipment.PPView{Current: s.observation}, equipment.OK
}
func (s *renewalStore) PPSession(context.Context, string, equipment.PPInput) (equipment.PPView, equipment.Code) {
	return equipment.PPView{Rows: []equipment.PPRow{s.row}}, equipment.OK
}

func TestTrafficWindowsPruneJobsWithoutPersistingReadings(t *testing.T) {
	var seconds atomic.Int64
	seconds.Store(time.Now().Unix())
	store := &renewalStore{}
	clock := func() time.Time { return time.Unix(seconds.Load(), 0) }
	inv := inventory.New(renewalAuth{}, store, renewalVault{}, renewalRemote{}, renewalIDs{}, clock)
	defer inv.Close()
	d := equipment.Device{ID: uuid.NewString(), Revision: 1, Active: true, Fingerprint: "synthetic", CredentialID: uuid.NewString()}
	store.target = equipment.PPTarget{
		Device: d, Credential: equipment.SealedCredential{Metadata: equipment.Credential{ID: d.CredentialID, Revision: 1}},
	}
	store.row = equipment.PPRow{
		ID: uuid.NewString(), RemoteID: "*1", SessionID: "synthetic", State: "ONLINE", Uptime: equipment.PPInterval{Known: true},
	}
	store.observation = &equipment.PPObservation{
		ID: uuid.NewString(), DeviceID: d.ID, Provenance: equipment.PPProvenance{
			Epoch: inv.Epoch, Connection: equipment.PPConnection(d), Revision: 1,
			CredentialRevision: 1, Sample: equipment.PPInterval{Known: true},
		},
	}
	scope := access.Command{Profile: "company", ExpectedProfile: "company", Activation: "active", ExpectedActivation: "active"}
	q := equipment.Command{Op: "pppoe.read-start", Scope: scope}
	service := pppoe.New(inv, nil)
	for window := 0; window < 100; window++ {
		in := equipment.PPInput{
			DeviceID: d.ID, Revision: 1, Operation: uuid.NewString(), Scope: "traffic",
			ObservationID: store.observation.ID, RowIDs: []string{store.row.ID},
		}
		v, code := service.HandlePP(context.Background(), q, in)
		if code != equipment.OK || v.Job == nil {
			t.Fatalf("window %d: %s", window, code)
		}
		inv.WG.Wait()
		inv.Mu.Lock()
		job, count := inv.Jobs[v.Job.ID], len(inv.Jobs)
		if job.View.Code != equipment.OK || !job.Released || count > 7 || count >= equipment.MaxJobs {
			inv.Mu.Unlock()
			t.Fatalf("window %d: state %s jobs %d", window, job.View.Code, count)
		}
		inv.Mu.Unlock()
		seconds.Add(55)
	}
	if store.writes.Load() != 0 {
		t.Fatalf("traffic persisted %d times", store.writes.Load())
	}
}
