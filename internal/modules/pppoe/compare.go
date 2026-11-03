package pppoe

import (
	"infradesktop/internal/modules/equipment"
	"time"
)

func PPCompatible(a, b *PPObservation) bool {
	return equipment.PPCompatible(a, b)
}

func intervalAdvance(a, b, elapsed PPInterval) bool {
	if !a.Known || !b.Known || !elapsed.Known {
		return false
	}
	if a.Min < 0 || b.Min < 0 || a.Max < a.Min || b.Max < b.Min || elapsed.Min < 0 || elapsed.Max < elapsed.Min {
		return false
	}
	return b.Min > elapsed.Max && b.Min-a.Max <= elapsed.Max && b.Max-a.Min >= elapsed.Min
}

func ppWallElapsed(p, q PPProvenance) PPInterval {
	a0, e0 := time.Parse(time.RFC3339Nano, p.Started)
	a1, e1 := time.Parse(time.RFC3339Nano, p.Ended)
	b0, e2 := time.Parse(time.RFC3339Nano, q.Started)
	b1, e3 := time.Parse(time.RFC3339Nano, q.Ended)
	if e0 != nil || e1 != nil || e2 != nil || e3 != nil || a1.Before(a0) || b1.Before(b0) {
		return PPInterval{}
	}
	return PPInterval{Min: b0.Sub(a1).Nanoseconds(), Max: b1.Sub(a0).Nanoseconds(), Known: true}
}

func ppJointElapsed(a, b, wall PPInterval) PPInterval {
	if !a.Known || !b.Known || !wall.Known || a.Min < 0 || a.Max < a.Min || b.Min < 0 || b.Max < b.Min {
		return PPInterval{}
	}
	elapsed := PPInterval{Min: b.Min - a.Max, Max: b.Max - a.Min, Known: true}
	if elapsed.Min > wall.Max || wall.Min > elapsed.Max {
		return PPInterval{}
	}
	if wall.Min < elapsed.Min {
		elapsed.Min = wall.Min
	}
	if wall.Max > elapsed.Max {
		elapsed.Max = wall.Max
	}
	return elapsed
}

func continuity(a, b *PPObservation, x, y PPRow, wall PPInterval) bool {
	p, q := a.Provenance, b.Provenance
	if p.Epoch == "" || p.Epoch != q.Epoch ||
		!x.Sample.Known || !y.Sample.Known || !p.Sample.Known || !q.Sample.Known {
		return false
	}
	dt := ppJointElapsed(x.Sample, y.Sample, wall)
	rt := ppJointElapsed(p.Sample, q.Sample, wall)
	return x.Login == y.Login &&
		intervalAdvance(p.RouterUptime, q.RouterUptime, rt) &&
		intervalAdvance(x.Uptime, y.Uptime, dt)
}

func ppDeltaSession(r PPRow) *PPDeltaSession {
	state := r.State
	if state == "" {
		state = "ONLINE"
	}
	return &PPDeltaSession{
		Login: r.Login, Address: r.Address, Caller: r.Caller, Uptime: r.Uptime, State: state,
	}
}

func ppSessionKey(r PPRow) string {
	if r.RemoteID == "" || r.SessionID == "" {
		return ""
	}
	return r.RemoteID + "\x00" + r.SessionID + "\x00" + r.Login
}

// ComparePP never merges a duplicated candidate or uses username as durable identity.
func ComparePP(a, b *PPObservation) ([]PPDelta, Code) {
	if !PPCompatible(a, b) {
		return nil, ObservationIncompatible
	}
	if a.ID == b.ID {
		return sameObservationDeltas(a.Rows), OK
	}
	startA, eA := time.Parse(time.RFC3339Nano, a.Provenance.Started)
	startB, eB := time.Parse(time.RFC3339Nano, b.Provenance.Started)
	if eA != nil || eB != nil || startB.Before(startA) {
		return nil, Invalid
	}
	wall := ppWallElapsed(a.Provenance, b.Provenance)
	am, al := indexPPRows(a.Rows)
	bm, bl := indexPPRows(b.Rows)
	out := make([]PPDelta, 0, len(a.Rows)+len(b.Rows))
	used := map[int]bool{}
	for _, x := range a.Rows {
		out = append(out, compareBasePPRow(a, b, x, wall, am, bm, al, bl, used))
	}
	for i, y := range b.Rows {
		if !used[i] {
			out = append(out, newTargetDelta(y, al[y.Login] > 0))
		}
	}
	return out, OK
}

func sameObservationDeltas(rows []PPRow) []PPDelta {
	out := make([]PPDelta, 0, len(rows))
	for _, r := range rows {
		out = append(out, PPDelta{
			Base: ppDeltaSession(r), Target: ppDeltaSession(r),
			BaseRow: r.ID, TargetRow: r.ID,
			Category: "unchanged", Reason: "same_observation",
		})
	}
	return out
}

func indexPPRows(rows []PPRow) (map[string][]int, map[string]int) {
	byKey := map[string][]int{}
	byLogin := map[string]int{}
	for i, r := range rows {
		k := ppSessionKey(r)
		byKey[k] = append(byKey[k], i)
		byLogin[r.Login]++
	}
	return byKey, byLogin
}

func compareBasePPRow(
	a, b *PPObservation, x PPRow, wall PPInterval,
	am, bm map[string][]int, al, bl map[string]int, used map[int]bool,
) PPDelta {
	k := ppSessionKey(x)
	ys, reconnected := matchTargetPPRows(x, b.Rows, bm[k], al, bl)
	if (k != "" && len(am[k]) == 1 && len(ys) == 1) || reconnected {
		yi := ys[0]
		used[yi] = true
		return buildMatchedDelta(a, b, x, b.Rows[yi], wall)
	}
	cat, reason := "absent", "only_in_base"
	if bl[x.Login] > 0 || len(ys) > 0 {
		cat, reason = "ambiguous", "possible_reconnection"
	}
	return PPDelta{Base: ppDeltaSession(x), BaseRow: x.ID, Category: cat, Reason: reason}
}

func matchTargetPPRows(
	x PPRow, bRows []PPRow, ys []int, al, bl map[string]int,
) ([]int, bool) {
	if x.Login != "" && x.State == "OFFLINE" && al[x.Login] == 1 && bl[x.Login] == 1 {
		for yi, y := range bRows {
			if y.Login == x.Login && y.State != "OFFLINE" {
				return []int{yi}, true
			}
		}
	}
	return ys, false
}

func buildMatchedDelta(a, b *PPObservation, x, y PPRow, wall PPInterval) PPDelta {
	d := PPDelta{
		Base: ppDeltaSession(x), Target: ppDeltaSession(y),
		BaseRow: x.ID, TargetRow: y.ID,
		Category: "ambiguous", Reason: "continuity_unknown",
	}
	if x.State == "OFFLINE" && y.State != "OFFLINE" {
		d.Category = "changed"
		d.Reason = "reconnected"
		return d
	}
	if x.State == "OFFLINE" || y.State == "OFFLINE" {
		d.Category = "changed"
		d.Reason = "state_changed"
		if x.State == y.State {
			d.Category = "unchanged"
			d.Reason = "retained_offline"
		}
		return d
	}
	if a.ID == b.ID || continuity(a, b, x, y, wall) {
		d.Category = "unchanged"
		d.Reason = "continuity_supported"
		d.AddressChanged = x.Address != y.Address
		d.CallerChanged = x.Caller != y.Caller
		if d.AddressChanged || d.CallerChanged {
			d.Category = "changed"
		}
	}
	return d
}

func newTargetDelta(y PPRow, hasBaseLogin bool) PPDelta {
	cat, reason := "new", "only_in_target"
	if hasBaseLogin {
		cat, reason = "ambiguous", "possible_reconnection"
	}
	return PPDelta{Target: ppDeltaSession(y), TargetRow: y.ID, Category: cat, Reason: reason}
}

func ComparableConcentratorReadings(a, b *PPObservation, now time.Time) bool {
	if a == nil || b == nil || a.Quality.Status != "complete" || b.Quality.Status != "complete" {
		return false
	}
	a0, a1, okA := parseObservationWindow(a.Provenance)
	b0, b1, okB := parseObservationWindow(b.Provenance)
	if !okA || !okB {
		return false
	}
	return freshConcentrationEnd(a1, now) &&
		freshConcentrationEnd(b1, now) &&
		!a1.Before(b0) &&
		!b1.Before(a0)
}

func parseObservationWindow(p PPProvenance) (time.Time, time.Time, bool) {
	t0, e0 := time.Parse(time.RFC3339Nano, p.Started)
	t1, e1 := time.Parse(time.RFC3339Nano, p.Ended)
	if e0 != nil || e1 != nil || t1.Before(t0) {
		return time.Time{}, time.Time{}, false
	}
	return t0, t1, true
}

func freshConcentrationEnd(ended, now time.Time) bool {
	return !ended.After(now) && now.Sub(ended) <= 15*time.Minute
}
