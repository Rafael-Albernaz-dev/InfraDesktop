package pulse

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)

func at(seconds int) time.Time { return t0.Add(time.Duration(seconds) * time.Second) }

var (
	a = Target{ID: "a", Address: "192.0.2.1"}
	b = Target{ID: "b", Address: "192.0.2.2"}
)

func stateOf(t *testing.T, tr *Tracker, id string, now time.Time) State {
	t.Helper()
	d, ok := tr.Snapshot(now).Devices[id]
	if !ok {
		t.Fatalf("device %s missing from snapshot", id)
	}
	return d.State
}

func TestNewDeviceIsUnknown(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	if s := stateOf(t, tr, "a", at(0)); s != Unknown {
		t.Fatalf("state = %s", s)
	}
}

func TestOfflineOnlyAfterThreeConsecutiveFailures(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	for i, want := range []State{Online, Online, Offline} {
		s := at(10 * (i + 1))
		tr.Record(b, Reply, s)
		tr.Record(a, NoReply, s)
		tr.Record(b, Reply, s.Add(time.Second))
		if got := stateOf(t, tr, "a", s.Add(time.Second)); got != want {
			t.Fatalf("failure %d: state = %s, want %s", i+1, got, want)
		}
	}
	if d := tr.Snapshot(at(31)).Devices["a"]; !d.Since.Equal(at(30)) {
		t.Fatalf("since = %s", d.Since)
	}
}

func TestSingleReplyResetsTheStreak(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	for i := 0; i < 20; i++ {
		s := at(i * 10)
		tr.Record(b, Reply, s)
		if i%3 == 2 {
			tr.Record(a, Reply, s)
		} else {
			tr.Record(a, NoReply, s)
		}
		if i >= 2 {
			if got := stateOf(t, tr, "a", s); got != Online {
				t.Fatalf("round %d: state = %s", i, got)
			}
		}
	}
}

func TestRecoveryIsImmediate(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	for i := 0; i < 3; i++ {
		tr.Record(a, NoReply, at(i*10))
		tr.Record(b, Reply, at(i*10+1))
	}
	if got := stateOf(t, tr, "a", at(21)); got != Offline {
		t.Fatalf("state = %s", got)
	}
	tr.Record(a, Reply, at(30))
	d := tr.Snapshot(at(30)).Devices["a"]
	if d.State != Online || !d.Since.Equal(at(30)) || !d.LastReply.Equal(at(30)) {
		t.Fatalf("device = %+v", d)
	}
}

// Critical: losing this PC's network must never become mass OFFLINE,
// not even for the seconds between one device's third failure and the next.
func TestEverythingSilentIsNeverMassOffline(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	tr.Record(b, Reply, at(1))
	for i := 1; i <= 3; i++ {
		tr.Record(a, NoReply, at(i*10))
		snap := tr.Snapshot(at(i * 10))
		if snap.Devices["a"].State == Offline || snap.Devices["b"].State == Offline {
			t.Fatalf("round %d A: false OFFLINE: %+v", i, snap)
		}
		tr.Record(b, NoReply, at(i*10+8))
		snap = tr.Snapshot(at(i*10 + 8))
		if snap.Devices["a"].State == Offline || snap.Devices["b"].State == Offline {
			t.Fatalf("round %d B: false OFFLINE: %+v", i, snap)
		}
	}
	snap := tr.Snapshot(at(39))
	if snap.Network != NoResponse {
		t.Fatalf("network = %s", snap.Network)
	}
	if snap.Devices["a"].State != Unknown || snap.Devices["b"].State != Unknown {
		t.Fatalf("devices = %+v", snap.Devices)
	}
	tr.Record(a, Reply, at(40))
	if snap = tr.Snapshot(at(40)); snap.Network != NetworkOK {
		t.Fatalf("network after reply = %s", snap.Network)
	}
}

func TestDeadDeviceFoundWhenNetworkReturns(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	for i := 0; i < 3; i++ {
		tr.Record(a, NoReply, at(i*10))
		tr.Record(b, NoReply, at(i*10+1))
	}
	tr.Record(b, Reply, at(31))
	if got := stateOf(t, tr, "a", at(31)); got != Unknown {
		t.Fatalf("no new probe yet, state = %s", got)
	}
	tr.Record(a, NoReply, at(40))
	d := tr.Snapshot(at(40)).Devices["a"]
	if d.State != Offline || !d.Since.Equal(at(40)) {
		t.Fatalf("dead device not confirmed by the next probe: %+v", d)
	}
}

func TestNetworkReturnNeverShowsFalseOffline(t *testing.T) {
	tr := NewTracker()
	c := Target{ID: "c", Address: "192.0.2.3"}
	all := []Target{a, b, c}
	tr.Sync(all)
	for i, x := range all {
		tr.Record(x, Reply, at(i))
	}
	for round := 1; round <= 4; round++ {
		for i, x := range all {
			tr.Record(x, NoReply, at(round*10+i))
		}
	}
	tr.Record(a, Reply, at(50))
	snap := tr.Snapshot(at(51))
	if snap.Network != NetworkOK {
		t.Fatalf("network = %s", snap.Network)
	}
	for _, id := range []string{"b", "c"} {
		if got := snap.Devices[id].State; got != Unknown {
			t.Fatalf("%s before a new probe = %q", id, got)
		}
	}
	tr.Record(b, Reply, at(52))
	if got := stateOf(t, tr, "b", at(52)); got != Online {
		t.Fatalf("b = %s", got)
	}
	if got := stateOf(t, tr, "c", at(52)); got != Unknown {
		t.Fatalf("c = %s", got)
	}
}

func TestDeviceAddedDuringOutageDoesNotHideIt(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	tr.Record(b, Reply, at(1))
	for round := 1; round <= 3; round++ {
		tr.Record(a, NoReply, at(round*10))
		tr.Record(b, NoReply, at(round*10+1))
	}
	c := Target{ID: "c", Address: "192.0.2.3"}
	tr.Sync([]Target{a, b, c})
	snap := tr.Snapshot(at(32))
	if snap.Network != NoResponse {
		t.Fatalf("network = %s", snap.Network)
	}
	tr.Record(b, Reply, at(33))
	if got := stateOf(t, tr, "a", at(33)); got != Unknown {
		t.Fatalf("a = %s", got)
	}
}

func TestOneDeviceDownWhileOthersAnswerKeepsNetworkOK(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	for i := 0; i < 4; i++ {
		tr.Record(a, NoReply, at(i*10))
		tr.Record(b, Reply, at(i*10+1))
	}
	if snap := tr.Snapshot(at(31)); snap.Network != NetworkOK {
		t.Fatalf("network = %s", snap.Network)
	}
}

func TestSingleDeviceInventoryCanGoOffline(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a})
	for i := 0; i < 3; i++ {
		tr.Record(a, NoReply, at(i*10))
	}
	snap := tr.Snapshot(at(20))
	if snap.Devices["a"].State != Offline || snap.Network != NetworkOK {
		t.Fatalf("snapshot = %+v", snap)
	}
}

// Critical: a probe that could not run says nothing about the equipment.
func TestProbeUnavailableIsNotEvidenceOfOutage(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	tr.Record(b, Reply, at(0))
	for i := 1; i <= 5; i++ {
		tr.Record(a, ProbeUnavailable, at(i*10))
		tr.Record(b, ProbeUnavailable, at(i*10))
	}
	snap := tr.Snapshot(at(50))
	if snap.Network != ProbeBlocked {
		t.Fatalf("network = %s", snap.Network)
	}
	if snap.Devices["a"].State != Unknown {
		t.Fatalf("state = %s", snap.Devices["a"].State)
	}
	tr.Record(a, Reply, at(60))
	if snap = tr.Snapshot(at(60)); snap.Network != NetworkOK {
		t.Fatalf("network after reply = %s", snap.Network)
	}
}

func TestProbeUnavailableKeepsNoFailureStreak(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	tr.Record(a, NoReply, at(10))
	tr.Record(a, NoReply, at(20))
	tr.Record(a, ProbeUnavailable, at(30))
	tr.Record(b, Reply, at(31))
	tr.Record(a, NoReply, at(40))
	tr.Record(b, Reply, at(41))
	if got := stateOf(t, tr, "a", at(41)); got != Offline {
		t.Fatalf("streak lost: state = %s", got)
	}
}

// Old data never passes for current: a stopped collector shows no state.
func TestStaleCheckBecomesUnknown(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	if got := stateOf(t, tr, "a", at(89)); got != Online {
		t.Fatalf("state at 89 s = %s", got)
	}
	if got := stateOf(t, tr, "a", at(91)); got != Unknown {
		t.Fatalf("state at 91 s = %s", got)
	}
}

func TestSyncDropsRemovedAndResetsChangedAddress(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a, b})
	tr.Record(a, Reply, at(0))
	tr.Record(b, Reply, at(0))
	moved := Target{ID: "a", Address: "192.0.2.9"}
	tr.Sync([]Target{moved})
	snap := tr.Snapshot(at(1))
	if _, ok := snap.Devices["b"]; ok {
		t.Fatal("removed device kept")
	}
	if snap.Devices["a"].State != Unknown {
		t.Fatalf("changed address kept state: %s", snap.Devices["a"].State)
	}
	tr.Record(a, Reply, at(2)) // late result for the old address
	if got := tr.Snapshot(at(2)).Devices["a"].State; got != Unknown {
		t.Fatalf("stale result applied: %s", got)
	}
	tr.Record(b, Reply, at(2)) // late result for a removed device
	if _, ok := tr.Snapshot(at(2)).Devices["b"]; ok {
		t.Fatal("removed device came back")
	}
}

func TestSyncKeepsStateWhenAddressIsTheSame(t *testing.T) {
	tr := NewTracker()
	tr.Sync([]Target{a})
	tr.Record(a, Reply, at(0))
	tr.Sync([]Target{a, b})
	if got := stateOf(t, tr, "a", at(1)); got != Online {
		t.Fatalf("state = %s", got)
	}
}

type fakeInventory struct {
	mu      sync.Mutex
	profile string
	targets []Target
	ok      bool
}

func (f *fakeInventory) Targets(context.Context) (string, []Target, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.profile, append([]Target(nil), f.targets...), f.ok
}

func (f *fakeInventory) set(profile string, targets []Target, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.profile, f.targets, f.ok = profile, targets, ok
}

type fakeProber struct {
	mu                  sync.Mutex
	calls               map[string]int
	running, maxRunning atomic.Int32
	outcome             Outcome
	hold                time.Duration
}

func (f *fakeProber) Probe(ctx context.Context, address string) Outcome {
	now := f.running.Add(1)
	defer f.running.Add(-1)
	for {
		seen := f.maxRunning.Load()
		if now <= seen || f.maxRunning.CompareAndSwap(seen, now) {
			break
		}
	}
	time.Sleep(f.hold)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	f.calls[address]++
	return f.outcome
}

func manyTargets(n int) []Target {
	out := make([]Target, n)
	for i := range out {
		id := string(rune('A'+i/26)) + string(rune('a'+i%26))
		out[i] = Target{ID: id, Address: "198.51.100." + id}
	}
	return out
}

func fullCycle(c *Collector) {
	for second := 0; second < Slots; second++ {
		c.Step(context.Background(), second, at(second))
	}
}

func TestSlotIsStableAndSpread(t *testing.T) {
	seen := map[int]int{}
	for _, target := range manyTargets(200) {
		slot := Slot(target.ID)
		if slot != Slot(target.ID) || slot < 0 || slot >= Slots {
			t.Fatalf("slot = %d", slot)
		}
		seen[slot]++
	}
	if len(seen) < Slots/2 {
		t.Fatalf("targets not spread: %v", seen)
	}
}

func TestEveryTargetProbedExactlyOncePerCycle(t *testing.T) {
	targets := manyTargets(60)
	inv := &fakeInventory{profile: "p1", targets: targets, ok: true}
	p := &fakeProber{outcome: Reply}
	c := NewCollector(inv, p, 8)
	fullCycle(c)
	if len(p.calls) != len(targets) {
		t.Fatalf("probed %d of %d", len(p.calls), len(targets))
	}
	for address, n := range p.calls {
		if n != 1 {
			t.Fatalf("%s probed %d times", address, n)
		}
	}
}

// Critical: a large inventory must not become a burst.
func TestConcurrencyLimitHolds(t *testing.T) {
	inv := &fakeInventory{profile: "p1", targets: manyTargets(120), ok: true}
	p := &fakeProber{outcome: Reply, hold: time.Millisecond}
	c := NewCollector(inv, p, 4)
	fullCycle(c)
	if got := p.maxRunning.Load(); got > 4 || got == 0 {
		t.Fatalf("max concurrency = %d", got)
	}
}

// Critical: state of one company never appears for another.
func TestCompanySwitchIsolatesState(t *testing.T) {
	inv := &fakeInventory{profile: "p1", targets: []Target{a, b}, ok: true}
	p := &fakeProber{outcome: Reply}
	c := NewCollector(inv, p, 4)
	fullCycle(c)
	if _, ok := c.Snapshot("p1", at(10)); !ok {
		t.Fatal("no snapshot for active company")
	}
	p.outcome = NoReply // same ID and address in another company proves nothing
	inv.set("p2", []Target{{ID: "a", Address: "192.0.2.1"}}, true)
	c.Step(context.Background(), 0, at(11))
	if _, ok := c.Snapshot("p1", at(11)); ok {
		t.Fatal("previous company still visible")
	}
	snap, ok := c.Snapshot("p2", at(11))
	if !ok {
		t.Fatal("no snapshot for new company")
	}
	if d := snap.Devices["a"]; d.State == Online {
		t.Fatalf("state crossed companies: %+v", d)
	}
}

func TestInventoryUnavailableStopsProbing(t *testing.T) {
	inv := &fakeInventory{profile: "p1", targets: []Target{a}, ok: true}
	p := &fakeProber{outcome: Reply}
	c := NewCollector(inv, p, 4)
	fullCycle(c)
	p.mu.Lock()
	before := p.calls[a.Address]
	p.mu.Unlock()
	inv.set("", nil, false)
	fullCycle(c)
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls[a.Address] != before {
		t.Fatal("probed without an inventory")
	}
	if _, ok := c.Snapshot("p1", at(99)); ok {
		t.Fatal("snapshot kept without an inventory")
	}
}

func TestStepHonoursCancellation(t *testing.T) {
	inv := &fakeInventory{profile: "p1", targets: manyTargets(40), ok: true}
	p := &fakeProber{outcome: Reply}
	c := NewCollector(inv, p, 4)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for second := 0; second < Slots; second++ {
		c.Step(ctx, second, at(second))
	}
	if len(p.calls) != 0 {
		t.Fatalf("probed after cancel: %d", len(p.calls))
	}
}
