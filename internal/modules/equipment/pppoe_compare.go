package equipment

import "time"

func PPCompatible(a, b *PPObservation) bool {
	return a != nil && b != nil && a.DeviceID == b.DeviceID && a.Scope == PPScope && b.Scope == PPScope && a.Quality.Status == "complete" && b.Quality.Status == "complete" && a.Provenance.Installation == b.Provenance.Installation && a.Provenance.Company == b.Provenance.Company && a.Provenance.Connection == b.Provenance.Connection && a.Provenance.CredentialRevision == b.Provenance.CredentialRevision && a.Provenance.Family == b.Provenance.Family && a.Provenance.Version == b.Provenance.Version && a.Provenance.Projection != "" && a.Provenance.Projection == b.Provenance.Projection && a.Provenance.Codec != "" && a.Provenance.Codec == b.Provenance.Codec && a.Provenance.Capabilities != "" && a.Provenance.Capabilities == b.Provenance.Capabilities
}

// intervalAdvance never adds untrusted durations (overflow), and uses no guessed tolerance.
func intervalAdvance(a, b, elapsed PPInterval) bool {
	if !a.Known || !b.Known || !elapsed.Known || a.Min < 0 || b.Min < 0 || a.Max < a.Min || b.Max < b.Min || elapsed.Min < 0 || elapsed.Max < elapsed.Min {
		return false
	}
	return b.Min > elapsed.Max && b.Min-a.Max <= elapsed.Max && b.Max-a.Min >= elapsed.Min
}

// UTC and monotonic collection windows must support the same elapsed interval.
// Their outer bounds are retained conservatively; no guessed clock tolerance.
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
	if p.Epoch == "" || p.Epoch != q.Epoch || !x.Sample.Known || !y.Sample.Known || !p.Sample.Known || !q.Sample.Known {
		return false
	}
	dt := ppJointElapsed(x.Sample, y.Sample, wall)
	rt := ppJointElapsed(p.Sample, q.Sample, wall)
	return x.Login == y.Login && intervalAdvance(p.RouterUptime, q.RouterUptime, rt) && intervalAdvance(x.Uptime, y.Uptime, dt)
}

// ComparePP never merges a duplicated candidate or uses username as durable identity.
func ComparePP(a, b *PPObservation) ([]PPDelta, Code) {
	if !PPCompatible(a, b) {
		return nil, ObservationIncompatible
	}
	if a.ID == b.ID {
		out := make([]PPDelta, 0, len(a.Rows))
		for _, r := range a.Rows {
			out = append(out, PPDelta{BaseRow: r.ID, TargetRow: r.ID, Category: "unchanged", Reason: "same_observation"})
		}
		return out, OK
	}
	startA, eA := time.Parse(time.RFC3339Nano, a.Provenance.Started)
	startB, eB := time.Parse(time.RFC3339Nano, b.Provenance.Started)
	if eA != nil || eB != nil || startB.Before(startA) {
		return nil, Invalid
	}
	wall := ppWallElapsed(a.Provenance, b.Provenance)
	key := func(r PPRow) string {
		if r.RemoteID == "" || r.SessionID == "" {
			return ""
		}
		return r.RemoteID + "\x00" + r.SessionID
	}
	am, bm := map[string][]int{}, map[string][]int{}
	al, bl := map[string]int{}, map[string]int{}
	for i, r := range a.Rows {
		am[key(r)] = append(am[key(r)], i)
		al[r.Login]++
	}
	for i, r := range b.Rows {
		bm[key(r)] = append(bm[key(r)], i)
		bl[r.Login]++
	}
	out := make([]PPDelta, 0, len(a.Rows)+len(b.Rows))
	used := map[int]bool{}
	for _, x := range a.Rows {
		k := key(x)
		ys := bm[k]
		if k != "" && len(am[k]) == 1 && len(ys) == 1 {
			yi := ys[0]
			y := b.Rows[yi]
			used[yi] = true
			d := PPDelta{BaseRow: x.ID, TargetRow: y.ID, Category: "ambiguous", Reason: "continuity_unknown"}
			if a.ID == b.ID || continuity(a, b, x, y, wall) {
				d.Category = "unchanged"
				d.Reason = "continuity_supported"
				d.AddressChanged = x.Address != y.Address
				d.CallerChanged = x.Caller != y.Caller
				if d.AddressChanged || d.CallerChanged {
					d.Category = "changed"
				}
			}
			out = append(out, d)
		} else {
			cat, reason := "absent", "only_in_base"
			if bl[x.Login] > 0 || len(ys) > 0 {
				cat, reason = "ambiguous", "possible_reconnection"
			}
			out = append(out, PPDelta{BaseRow: x.ID, Category: cat, Reason: reason})
		}
	}
	for i, y := range b.Rows {
		if !used[i] {
			cat, reason := "new", "only_in_target"
			if al[y.Login] > 0 {
				cat, reason = "ambiguous", "possible_reconnection"
			}
			out = append(out, PPDelta{TargetRow: y.ID, Category: cat, Reason: reason})
		}
	}
	return out, OK
}
