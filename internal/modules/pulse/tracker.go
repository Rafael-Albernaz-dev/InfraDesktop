package pulse

import (
	"sync"
	"time"
)

type observation struct {
	target                     Target
	status                     DeviceStatus
	failures                   int
	firstFailure, thirdFailure time.Time
	outcome                    Outcome
}
type Tracker struct {
	mu      sync.Mutex
	devices map[string]*observation
}

func NewTracker() *Tracker { return &Tracker{devices: map[string]*observation{}} }
func (t *Tracker) Sync(targets []Target) {
	t.mu.Lock()
	defer t.mu.Unlock()
	next := make(map[string]*observation, len(targets))
	for _, target := range targets {
		old := t.devices[target.ID]
		if old == nil || old.target != target {
			old = &observation{target: target}
		}
		next[target.ID] = old
	}
	t.devices = next
}
func (t *Tracker) Record(target Target, outcome Outcome, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	d := t.devices[target.ID]
	if d == nil || d.target != target {
		return
	}
	previous := t.deviceStatus(d, now, t.network(now)).State
	d.status.CheckedAt, d.outcome = now, outcome
	switch outcome {
	case Reply:
		if previous != Online {
			d.status.Since = now
		}
		d.status.State, d.status.LastReply = Online, now
		d.failures, d.firstFailure, d.thirdFailure = 0, time.Time{}, time.Time{}
	case NoReply:
		d.failures++
		if d.failures == 1 {
			d.firstFailure = now
		}
		if d.failures == 3 {
			d.thirdFailure = now
		}
	}
}
func (t *Tracker) Snapshot(now time.Time) Snapshot {
	t.mu.Lock()
	defer t.mu.Unlock()
	snap := Snapshot{Devices: map[string]DeviceStatus{}, Network: t.network(now)}
	for id, d := range t.devices {
		status := t.deviceStatus(d, now, snap.Network)
		snap.Devices[id] = status
	}
	return snap
}
func stale(at, now time.Time) bool { return at.IsZero() || now.Sub(at) > 90*time.Second }
func (t *Tracker) otherReplied(d *observation) bool {
	if len(t.devices) == 1 {
		return true
	}
	for _, other := range t.devices {
		if other != d && other.status.LastReply.After(d.firstFailure) {
			return true
		}
	}
	return false
}
func (t *Tracker) network(now time.Time) Network {
	blocked, silent := 0, 0
	var latestReply, latestBlocked time.Time
	for _, d := range t.devices {
		if d.status.LastReply.After(latestReply) {
			latestReply = d.status.LastReply
		}
		if d.outcome == ProbeUnavailable {
			blocked++
			if d.status.CheckedAt.After(latestBlocked) {
				latestBlocked = d.status.CheckedAt
			}
		}
		if d.failures > 0 && !stale(d.status.CheckedAt, now) {
			silent++
		}
	}
	if blocked > 0 && !latestReply.After(latestBlocked) {
		return ProbeBlocked
	}
	if len(t.devices) > 1 && silent == len(t.devices) {
		return NoResponse
	}
	return NetworkOK
}

func (t *Tracker) deviceStatus(d *observation, now time.Time, network Network) DeviceStatus {
	status := d.status
	status.Address = d.target.Address
	if network != NetworkOK || d.outcome == ProbeUnavailable || stale(status.CheckedAt, now) {
		status.State = Unknown
	} else if d.failures >= 3 && t.otherReplied(d) {
		status.State, status.Since = Offline, d.thirdFailure
	}
	return status
}
