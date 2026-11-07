package composition

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/modules/pulse"
)

type pulseStore struct {
	equipment.Store
	state          access.State
	devices        []equipment.Device
	reads          atomic.Int32
	closed         atomic.Bool
	readAfterClose atomic.Bool
}

func (s *pulseStore) Read(context.Context) (access.State, access.Code) { return s.state, access.OK }
func (s *pulseStore) Update(_ context.Context, f func(*access.State) access.Code) access.Code {
	return f(&s.state)
}
func (s *pulseStore) ReadEquipment(context.Context, string) (equipment.State, equipment.Code) {
	if s.closed.Load() {
		s.readAfterClose.Store(true)
	}
	s.reads.Add(1)
	return equipment.State{Devices: s.devices}, equipment.OK
}
func TestPulseInventoryIncludesOnlyActiveValidIPs(t *testing.T) {
	store := &pulseStore{state: access.State{Account: &access.Account{Profile: "p"}, Companies: []access.Company{{ID: "p"}}},
		devices: []equipment.Device{{ID: "active", Active: true, Address: "127.0.0.1"},
			{ID: "inactive", Address: "127.0.0.2"}, {ID: "invalid", Active: true, Address: "hostname"}}}
	inv := pulseInventory{access: access.New(store, nil, time.Now, 1000, "a"), store: store}
	profile, targets, ok := inv.Targets(context.Background())
	if !ok || profile != "p" || len(targets) != 1 || targets[0].ID != "active" {
		t.Fatal(profile, targets, ok)
	}
	store.state.Account = nil
	if _, targets, ok = inv.Targets(context.Background()); ok || len(targets) > 0 {
		t.Fatal("accountless probing")
	}
}
func TestServerCollectsWithoutWindowAndStopsBeforeStoreClose(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	store := &pulseStore{state: access.State{Account: &access.Account{Profile: "p"}, Companies: []access.Company{{ID: "p"}}}}
	target := ""
	for i := 0; target == ""; i++ {
		id := fmt.Sprint(i)
		if pulse.Slot(id) == 0 {
			target = id
		}
	}
	store.devices = []equipment.Device{{ID: target, Active: true, Address: "127.0.0.1"}}
	auth := access.New(store, nil, time.Now, uint32(os.Getuid()), "a")
	closeStore := func() error { store.closed.Store(true); return nil }
	server, err := ipc.ListenAccess(root, uint32(os.Getuid()), auth, foundation.Status{}, closeStore)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	collector := composePulse(server, auth, store)
	// Loopback ping runs with no client, window, session, credential or SSH adapter.
	done := make(chan struct{})
	server.StartBackground(func(ctx context.Context) { <-ctx.Done(); close(done) })
	until := time.NewTimer(2 * time.Second)
	defer until.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		snapshot, ok := collector.Snapshot("p", time.Now())
		if ok && snapshot.Devices[target].State == pulse.Online {
			break
		}
		select {
		case <-tick.C:
		case <-until.C:
			t.Fatal("collector did not read inventory")
		}
	}
	if _, ok := collector.Snapshot("p", time.Now()); !ok {
		t.Fatal("collector did not publish")
	}
	server.Close()
	select {
	case <-done:
	default:
		t.Fatal("background outlived server")
	}
	if !store.closed.Load() || store.readAfterClose.Load() {
		t.Fatal("store lifecycle order")
	}
}

var _ pulse.Inventory = pulseInventory{}
