package equipment

import (
	"sort"
	"time"
)

// A complete catalog proves current absence. Missing rows have no current
// running state; their last known identity and time remain explicitly dated.
func ReconcileCatalogAbsences(previous, next *PDDiagnostic) Code {
	if next == nil || next.Scope != PDCatalog || next.Quality.Status != "complete" || !next.Quality.Terminated || !next.Quality.MembershipChecked {
		return Partial
	}
	next.AbsentInterfaces = 0
	next.AbsencesLimited = false
	current := next.Rows[:0]
	for _, row := range next.Rows {
		if row.NotSeenSince == "" {
			current = append(current, row)
		}
	}
	next.Rows = current
	present := map[string]bool{}
	for i := range next.Rows {
		present[next.Rows[i].ID] = true
	}
	if previous == nil || previous.Scope != PDCatalog {
		return OK
	}
	a := &PPObservation{DeviceID: previous.DeviceID, Scope: PPScope, Quality: previous.Quality, Provenance: previous.Provenance}
	b := &PPObservation{DeviceID: next.DeviceID, Scope: PPScope, Quality: next.Quality, Provenance: next.Provenance}
	if !PPCompatible(a, b) || previous.Provenance.Revision != next.Provenance.Revision {
		return OK
	}
	now, e := time.Parse(time.RFC3339Nano, next.Provenance.Ended)
	if e != nil {
		return Invalid
	}
	prior, e := time.Parse(time.RFC3339Nano, previous.Provenance.Ended)
	if e != nil || !now.After(prior) {
		return OK
	}
	absent := []PDRow{}
	for _, r := range previous.Rows {
		if r.Kind != "interface" || r.Dynamic == nil || !*r.Dynamic || present[r.ID] {
			continue
		}
		if r.NotSeenSince == "" {
			r.NotSeenSince = previous.Provenance.Ended
		}
		r.Running = nil
		seen, e := time.Parse(time.RFC3339Nano, r.NotSeenSince)
		if e != nil || now.Before(seen) || now.Sub(seen) >= 24*time.Hour {
			continue
		}
		absent = append(absent, r)
	}
	sort.Slice(absent, func(i, j int) bool {
		if absent[i].NotSeenSince == absent[j].NotSeenSince {
			return absent[i].ID < absent[j].ID
		}
		return absent[i].NotSeenSince > absent[j].NotSeenSince
	})
	room := max(0, PPMaxRows-len(next.Rows))
	if len(absent) > room {
		absent = absent[:room]
		next.AbsencesLimited = true
	}
	next.AbsentInterfaces = len(absent)
	next.Rows = append(next.Rows, absent...)
	return OK
}
