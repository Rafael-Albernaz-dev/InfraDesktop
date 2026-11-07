package pulse

import (
	"context"
	"sync"
	"testing"
	"time"

	"infradesktop/internal/modules/equipment"
)

var r0 = time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)

type fakeSession struct {
	d      *fakeDialer
	id     string
	closed bool
}

func (s *fakeSession) Read(context.Context, ReadTarget) (Reading, ReadFault) {
	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	s.d.reads[s.id]++
	return Reading{CPU: 7, Clients: 10, Uptime: time.Hour}, s.d.readFault[s.id]
}
func (s *fakeSession) Close() {
	s.d.mu.Lock()
	defer s.d.mu.Unlock()
	s.closed = true
}

type fakeDialer struct {
	mu        sync.Mutex
	dials     map[string]int
	reads     map[string]int
	fault     map[string]ReadFault
	readFault map[string]ReadFault
	sessions  []*fakeSession
	gate      chan struct{}
	started   chan struct{}
	now, max  int
}

func newDialer() *fakeDialer {
	return &fakeDialer{
		dials: map[string]int{}, reads: map[string]int{},
		fault: map[string]ReadFault{}, readFault: map[string]ReadFault{},
	}
}
func (d *fakeDialer) Dial(ctx context.Context, t ReadTarget) (ReadSession, ReadFault) {
	d.mu.Lock()
	d.dials[t.ID]++
	d.now++
	d.max = max(d.max, d.now)
	gate, started := d.gate, d.started
	d.mu.Unlock()
	if started != nil {
		started <- struct{}{}
	}
	if gate != nil {
		select {
		case <-gate:
		case <-ctx.Done():
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.now--
	if f := d.fault[t.ID]; f != ReadOK {
		return nil, f
	}
	s := &fakeSession{d: d, id: t.ID}
	d.sessions = append(d.sessions, s)
	return s, ReadOK
}
func (d *fakeDialer) count(id string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.dials[id]
}
func (d *fakeDialer) openSessions() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := 0
	for _, s := range d.sessions {
		if !s.closed {
			n++
		}
	}
	return n
}

type fakeReadInventory struct {
	mu      sync.Mutex
	profile string
	targets []ReadTarget
	ok      bool
}

func (i *fakeReadInventory) ReadTargets(context.Context) (string, []ReadTarget, bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.profile, append([]ReadTarget(nil), i.targets...), i.ok
}
func (i *fakeReadInventory) set(profile string, targets ...ReadTarget) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.profile, i.targets, i.ok = profile, targets, true
}

var alvo = ReadTarget{ID: "conc", Role: "concentrator", Stamp: "v1"}

func always(string, string, time.Time) bool { return true }

func montar(limit int, targets ...ReadTarget) (*ReadCollector, *fakeDialer, *fakeReadInventory) {
	d, inv := newDialer(), &fakeReadInventory{}
	inv.set("empresa", targets...)
	return NewReadCollector(inv, d, always, limit), d, inv
}

// ciclo roda os 60 segundos do minuto informado.
func ciclo(c *ReadCollector, minuto int) {
	for s := 0; s < ReadSlots; s++ {
		c.Step(context.Background(), s, r0.Add(time.Duration(minuto)*time.Minute+time.Duration(s)*time.Second))
	}
}

func TestConnectionStaysOpenAcrossCycles(t *testing.T) {
	c, d, _ := montar(4, alvo)
	for m := 0; m < 3; m++ {
		ciclo(c, m)
	}
	if d.count("conc") != 1 || d.reads["conc"] != 3 {
		t.Fatalf("dials=%d reads=%d", d.count("conc"), d.reads["conc"])
	}
	got, ok := c.Snapshot("empresa", r0.Add(3*time.Minute))
	if !ok || !got["conc"].HasReading || got["conc"].CPU != 7 || got["conc"].Clients != 10 {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestDeviceIsReadOnlyInItsOwnSecond(t *testing.T) {
	alvoFora := ReadTarget{ID: "a", Role: equipment.RoleAP, Stamp: "v1"}
	for i := 0; ReadSlot(alvoFora.ID) == 0; i++ {
		alvoFora.ID += "a"
	}
	c, d, _ := montar(4, alvoFora)
	slot := ReadSlot(alvoFora.ID)
	for s := 0; s < ReadSlots; s++ {
		if s != slot {
			c.Step(context.Background(), s, r0.Add(time.Duration(s)*time.Second))
		}
	}
	if d.count(alvoFora.ID) != 0 {
		t.Fatal("read outside its second")
	}
	c.Step(context.Background(), slot, r0.Add(time.Duration(slot)*time.Second))
	if d.count(alvoFora.ID) != 1 {
		t.Fatalf("dials = %d", d.count(alvoFora.ID))
	}
}

func TestRefusedPasswordIsNeverRetriedUntilCredentialChanges(t *testing.T) {
	c, d, inv := montar(4, alvo)
	d.fault["conc"] = ReadRefused
	for m := 0; m < 30; m++ {
		ciclo(c, m)
	}
	if d.count("conc") != 1 {
		t.Fatalf("password tried %d times", d.count("conc"))
	}
	got, _ := c.Snapshot("empresa", r0.Add(30*time.Minute))
	if got["conc"].Fault != ReadRefused {
		t.Fatalf("fault = %q", got["conc"].Fault)
	}
	novo := alvo
	novo.Stamp = "v2"
	inv.set("empresa", novo)
	d.fault["conc"] = ReadOK
	ciclo(c, 31)
	got, _ = c.Snapshot("empresa", r0.Add(32*time.Minute))
	if d.count("conc") != 2 || got["conc"].Fault != ReadOK || !got["conc"].HasReading {
		t.Fatalf("dials=%d snapshot=%+v", d.count("conc"), got["conc"])
	}
}

func TestChangedHostKeyIsNeverRetriedUntilTargetChanges(t *testing.T) {
	c, d, _ := montar(4, alvo)
	d.fault["conc"] = ReadKeyChanged
	for m := 0; m < 30; m++ {
		ciclo(c, m)
	}
	got, _ := c.Snapshot("empresa", r0.Add(30*time.Minute))
	if d.count("conc") != 1 || got["conc"].Fault != ReadKeyChanged {
		t.Fatalf("dials=%d fault=%q", d.count("conc"), got["conc"].Fault)
	}
}

func TestLostConnectionBacksOffUpToTenCycles(t *testing.T) {
	c, d, _ := montar(4, alvo)
	d.fault["conc"] = ReadLost
	tentativas := []int{}
	antes := 0
	for m := 0; m < 40; m++ {
		ciclo(c, m)
		if n := d.count("conc"); n != antes {
			tentativas, antes = append(tentativas, m), n
		}
	}
	want := []int{0, 1, 3, 7, 15, 25, 35}
	if len(tentativas) != len(want) {
		t.Fatalf("attempts at %v", tentativas)
	}
	for i := range want {
		if tentativas[i] != want[i] {
			t.Fatalf("attempts at %v, want %v", tentativas, want)
		}
	}
}

func TestDroppedSessionIsClosedAndDialedAgain(t *testing.T) {
	c, d, _ := montar(4, alvo)
	ciclo(c, 0)
	d.readFault["conc"] = ReadLost
	ciclo(c, 1)
	if d.openSessions() != 0 {
		t.Fatal("broken session left open")
	}
	d.readFault["conc"] = ReadOK
	ciclo(c, 2)
	got, _ := c.Snapshot("empresa", r0.Add(3*time.Minute))
	if d.count("conc") != 2 || got["conc"].Fault != ReadOK {
		t.Fatalf("dials=%d fault=%q", d.count("conc"), got["conc"].Fault)
	}
}

func TestUnreachableDeviceIsNotDialed(t *testing.T) {
	d, inv := newDialer(), &fakeReadInventory{}
	inv.set("empresa", alvo)
	c := NewReadCollector(inv, d, func(string, string, time.Time) bool { return false }, 4)
	ciclo(c, 0)
	if d.count("conc") != 0 {
		t.Fatal("dialed a device that does not answer ping")
	}
}

func TestOldReadingIsDropped(t *testing.T) {
	c, _, _ := montar(4, alvo)
	ciclo(c, 0)
	got, _ := c.Snapshot("empresa", r0.Add(10*time.Minute))
	if got["conc"].HasReading {
		t.Fatalf("stale reading shown: %+v", got["conc"])
	}
}

func TestCompanySwitchClosesEverything(t *testing.T) {
	c, d, inv := montar(4, alvo)
	ciclo(c, 0)
	inv.set("outra", ReadTarget{ID: "b", Role: equipment.RoleAP, Stamp: "v1"})
	ciclo(c, 1)
	if _, ok := c.Snapshot("empresa", r0.Add(2*time.Minute)); ok {
		t.Fatal("old company still readable")
	}
	if d.openSessions() != 1 || d.count("b") != 1 {
		t.Fatalf("open=%d", d.openSessions())
	}
	got, ok := c.Snapshot("outra", r0.Add(2*time.Minute))
	if !ok || len(got) != 1 || got["conc"].HasReading {
		t.Fatalf("snapshot = %+v", got)
	}
}

func TestRemovedOrChangedTargetClosesItsSession(t *testing.T) {
	c, d, inv := montar(4, alvo)
	ciclo(c, 0)
	inv.set("empresa")
	ciclo(c, 1)
	if d.openSessions() != 0 {
		t.Fatal("removed device kept its session")
	}
	inv.set("empresa", alvo)
	ciclo(c, 2)
	novo := alvo
	novo.Stamp = "v2"
	inv.set("empresa", novo)
	ciclo(c, 3)
	if d.openSessions() != 1 || d.count("conc") != 3 {
		t.Fatalf("open=%d dials=%d", d.openSessions(), d.count("conc"))
	}
}

func TestInventoryUnavailableClosesAndDialsNothing(t *testing.T) {
	c, d, inv := montar(4, alvo)
	ciclo(c, 0)
	inv.mu.Lock()
	inv.ok = false
	inv.mu.Unlock()
	ciclo(c, 1)
	if d.openSessions() != 0 || d.count("conc") != 1 {
		t.Fatalf("open=%d dials=%d", d.openSessions(), d.count("conc"))
	}
}

func TestDialLimitHolds(t *testing.T) {
	targets := []ReadTarget{}
	for i := 0; ; i++ {
		id := "d" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		if ReadSlot(id) == 0 {
			targets = append(targets, ReadTarget{ID: id, Role: equipment.RoleAP, Stamp: "v1"})
		}
		if len(targets) == 8 {
			break
		}
	}
	c, d, _ := montar(2, targets...)
	d.gate, d.started = make(chan struct{}), make(chan struct{}, 8)
	done := make(chan struct{})
	go func() { c.Step(context.Background(), 0, r0); close(done) }()
	<-d.started
	<-d.started
	select {
	case <-d.started:
		t.Fatal("third simultaneous dial")
	case <-time.After(50 * time.Millisecond):
	}
	close(d.gate)
	<-done
	if d.max > 2 {
		t.Fatalf("max = %d", d.max)
	}
}

func TestLateDialAfterCloseIsDiscarded(t *testing.T) {
	c, d, _ := montar(4, alvo)
	d.gate, d.started = make(chan struct{}), make(chan struct{}, 1)
	done := make(chan struct{})
	go func() { ciclo(c, 0); close(done) }()
	<-d.started
	c.Close()
	close(d.gate)
	<-done
	if d.openSessions() != 0 {
		t.Fatal("session opened after Close was left open")
	}
	if _, ok := c.Snapshot("empresa", r0); ok {
		t.Fatal("closed collector still serves readings")
	}
}

func TestCloseStopsDialing(t *testing.T) {
	c, d, _ := montar(4, alvo)
	ciclo(c, 0)
	c.Close()
	ciclo(c, 1)
	if d.openSessions() != 0 || d.count("conc") != 1 {
		t.Fatalf("open=%d dials=%d", d.openSessions(), d.count("conc"))
	}
}

func TestCancelledStepDialsNothing(t *testing.T) {
	for range 64 {
		c, d, _ := montar(4, alvo)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		for s := 0; s < ReadSlots; s++ {
			c.Step(ctx, s, r0)
		}
		if d.count("conc") != 0 {
			t.Fatal("dialed after cancellation")
		}
	}
}
