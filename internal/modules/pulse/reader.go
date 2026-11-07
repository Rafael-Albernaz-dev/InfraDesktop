package pulse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"infradesktop/internal/modules/equipment"
)

const ReadSlots = 60

func ReadStamp(profile, connection string, deviceRevision, credentialRevision uint64) string {
	return fmt.Sprintf("%q/%q/%d/%d", profile, connection, deviceRevision, credentialRevision)
}

func ReadSlot(id string) int { return slotHash(id, ReadSlots) }

type ReadTarget struct {
	ID    string
	Role  equipment.Role
	Stamp string
}
type Reading struct {
	CPU, Clients int
	Uptime       time.Duration
}
type ReadFault string

const (
	ReadOK         ReadFault = ""
	ReadRefused    ReadFault = "refused"
	ReadKeyChanged ReadFault = "key_changed"
	ReadLost       ReadFault = "lost"
)

type ReadSession interface {
	Read(context.Context, ReadTarget) (Reading, ReadFault)
	Close()
}
type ReadDialer interface {
	Dial(context.Context, ReadTarget) (ReadSession, ReadFault)
}
type ReadInventory interface {
	ReadTargets(context.Context) (string, []ReadTarget, bool)
}
type DeviceReading struct {
	Stamp string
	Reading
	HasReading bool
	At         time.Time
	Fault      ReadFault
}
type readEntry struct {
	ctx            context.Context
	cancel         context.CancelFunc
	target         ReadTarget
	session        ReadSession
	value          DeviceReading
	busy           bool
	failures, next int
}
type refusalKey struct{ profile, id, stamp string }
type ReadCollector struct {
	mu        sync.Mutex
	inventory ReadInventory
	dialer    ReadDialer
	reachable func(string, string, time.Time) bool
	slots     chan struct{}
	profile   string
	entries   map[string]*readEntry
	refused   map[refusalKey]ReadFault
	round     int
	closed    bool
}

func NewReadCollector(inv ReadInventory, d ReadDialer, reachable func(string, string, time.Time) bool, limit int) *ReadCollector {
	return &ReadCollector{inventory: inv, dialer: d, reachable: reachable,
		slots: make(chan struct{}, max(1, min(4, limit))), entries: map[string]*readEntry{}, refused: map[refusalKey]ReadFault{}}
}
func (c *ReadCollector) refreshRead(ctx context.Context) {
	profile, targets, ok := c.inventory.ReadTargets(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	if !ok {
		profile, targets = "", nil
	}
	next := make(map[string]*readEntry, len(targets))
	for _, t := range targets {
		old := c.entries[t.ID]
		if profile == c.profile && old != nil && old.target == t {
			next[t.ID] = old
		} else {
			fault := c.refused[refusalKey{profile, t.ID, t.Stamp}]
			entryCtx, cancel := context.WithCancel(context.Background())
			next[t.ID] = &readEntry{ctx: entryCtx, cancel: cancel, target: t, value: DeviceReading{Fault: fault}}
		}
	}
	for id, e := range c.entries {
		if next[id] != e {
			e.cancel()
			if e.session != nil {
				e.session.Close()
			}
		}
	}
	c.profile, c.entries = profile, next
	c.round++
}
func (c *ReadCollector) Step(ctx context.Context, second int, at time.Time) {
	if ctx.Err() != nil {
		return
	}
	if second == 0 {
		c.refreshRead(ctx)
	}
	c.readSlot(ctx, second, at)
}
func (c *ReadCollector) readSlot(ctx context.Context, second int, at time.Time) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	profile, round := c.profile, c.round
	entries := []*readEntry{}
	for _, e := range c.entries {
		if ReadSlot(e.target.ID) == second && !e.busy && !terminalReadFault(e.value.Fault) && round >= e.next {
			e.busy = true
			entries = append(entries, e)
		}
	}
	c.mu.Unlock()
	var wg sync.WaitGroup
	for _, e := range entries {
		wg.Add(1)
		go func(e *readEntry) {
			defer wg.Done()
			c.readOne(ctx, profile, round, at, e)
		}(e)
	}
	wg.Wait()
}
func terminalReadFault(f ReadFault) bool { return f == ReadRefused || f == ReadKeyChanged }
func (c *ReadCollector) readOne(ctx context.Context, profile string, round int, at time.Time, e *readEntry) {
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(e.ctx, cancel)
	defer cancel()
	defer stop()
	c.mu.Lock()
	session := e.session
	current := c.currentEntry(profile, e)
	c.mu.Unlock()
	if !current || ctx.Err() != nil {
		c.finishRead(e)
		return
	}
	if session == nil && !c.reachable(profile, e.target.ID, at) {
		c.finishRead(e)
		return
	}
	fault := ReadOK
	if session == nil {
		session, fault = c.dialRead(ctx, profile, e)
	}
	var reading Reading
	if fault == ReadOK && session != nil {
		reading, fault = session.Read(ctx, e.target)
	}
	c.recordRead(ctx, readResult{profile: profile, round: round, at: at, entry: e, session: session, reading: reading, fault: fault})
}
func (c *ReadCollector) currentEntry(profile string, e *readEntry) bool {
	return !c.closed && c.profile == profile && c.entries[e.target.ID] == e
}
func (c *ReadCollector) finishRead(e *readEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e.busy = false
}
func (c *ReadCollector) dialRead(ctx context.Context, profile string, e *readEntry) (ReadSession, ReadFault) {
	select {
	case c.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ReadLost
	}
	defer func() { <-c.slots }()
	c.mu.Lock()
	current := c.currentEntry(profile, e)
	refused := c.refused[refusalKey{profile, e.target.ID, e.target.Stamp}]
	c.mu.Unlock()
	if terminalReadFault(refused) {
		return nil, refused
	}
	if !current || ctx.Err() != nil {
		return nil, ReadLost
	}
	session, fault := c.dialer.Dial(ctx, e.target)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rememberRefusal(profile, e, fault)
	if !c.currentEntry(profile, e) || ctx.Err() != nil {
		if session != nil {
			session.Close()
		}
		return nil, ReadLost
	}
	e.session = session
	return session, fault
}

type readResult struct {
	profile string
	round   int
	at      time.Time
	entry   *readEntry
	session ReadSession
	reading Reading
	fault   ReadFault
}

func (c *ReadCollector) recordRead(ctx context.Context, result readResult) {
	profile, round, at, e := result.profile, result.round, result.at, result.entry
	session, reading, fault := result.session, result.reading, result.fault
	c.mu.Lock()
	defer c.mu.Unlock()
	e.busy = false
	c.rememberRefusal(profile, e, fault)
	if !c.currentEntry(profile, e) || ctx.Err() != nil {
		if session != nil {
			session.Close()
		}
		return
	}
	if session == nil && fault == ReadOK {
		fault = ReadLost
	}
	e.value.Fault = fault
	if fault == ReadOK {
		e.failures, e.next = 0, 0
		e.value = DeviceReading{Reading: reading, HasReading: true, At: at}
		return
	}
	if session != nil {
		session.Close()
	}
	e.session = nil
	if terminalReadFault(fault) {
		c.rememberRefusal(profile, e, fault)
		return
	}
	e.failures = min(e.failures+1, 5)
	e.next = round + min(1<<(e.failures-1), 10)
}

// Called under mu, including for results whose target was invalidated in flight.
func (c *ReadCollector) rememberRefusal(profile string, e *readEntry, fault ReadFault) {
	if terminalReadFault(fault) {
		c.refused[refusalKey{profile, e.target.ID, e.target.Stamp}] = fault
	}
}
func (c *ReadCollector) Snapshot(profile string, now time.Time) (map[string]DeviceReading, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || profile == "" || profile != c.profile {
		return nil, false
	}
	out := make(map[string]DeviceReading, len(c.entries))
	for id, e := range c.entries {
		value := e.value
		value.Stamp = e.target.Stamp
		if now.Sub(value.At) > 180*time.Second {
			value.HasReading = false
		}
		out[id] = value
	}
	return out, true
}
func (c *ReadCollector) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	for _, e := range c.entries {
		e.cancel()
		if e.session != nil {
			e.session.Close()
			e.session = nil
		}
	}
	c.entries = map[string]*readEntry{}
}
