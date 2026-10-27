package equipment

import (
	"sort"
	"time"
)

// An absent item is evidence only after a completed, valid collection. The
// caller publishes this result atomically with its persisted previous input.
func ReconcilePP(previous, next *PPObservation) Code {
	if next == nil || next.Quality.Status != "complete" || !next.Quality.Terminated || !next.Quality.MembershipChecked || len(next.Rows) > PPMaxRows {
		return Partial
	}
	now, err := time.Parse(time.RFC3339Nano, next.Provenance.Ended)
	if err != nil {
		return Invalid
	}
	present, reconnected := map[string]bool{}, map[string]bool{}
	key := func(r PPRow) string { return r.RemoteID + "\x00" + r.SessionID + "\x00" + r.Login }
	previousKeys := map[string]bool{}
	compatible := PPCompatible(previous, next)
	if compatible {
		for _, r := range previous.Rows {
			if r.State != "OFFLINE" {
				previousKeys[key(r)] = true
			}
		}
	}
	for i := range next.Rows {
		r := &next.Rows[i]
		r.State = "ONLINE"
		r.AbsentStart = ""
		r.AbsentEnd = ""
		present[key(*r)] = true
		if r.Login != "" && !previousKeys[key(*r)] {
			reconnected[r.Login] = true
		}
	}
	offline := []PPRow{}
	if compatible {
		for _, r := range previous.Rows {
			if present[key(r)] || reconnected[r.Login] {
				continue
			}
			if r.State != "OFFLINE" {
				r.State = "OFFLINE"
				r.AbsentStart = previous.Provenance.Started
				r.AbsentEnd = next.Provenance.Ended
			}
			at, e := time.Parse(time.RFC3339Nano, r.AbsentEnd)
			if e != nil || now.Before(at) || now.Sub(at) >= 24*time.Hour {
				continue
			}
			offline = append(offline, r)
		}
	}
	sort.Slice(offline, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, offline[i].AbsentEnd)
		b, _ := time.Parse(time.RFC3339Nano, offline[j].AbsentEnd)
		if a.Equal(b) {
			return offline[i].ID < offline[j].ID
		}
		return a.After(b)
	})
	room := PPMaxRows - len(next.Rows)
	if len(offline) > room {
		offline = offline[:room]
	}
	next.Online = len(next.Rows)
	next.Offline = len(offline)
	next.Rows = append(next.Rows, offline...)
	if previous != nil && previous.ContextOmitted {
		for _, row := range offline {
			if row.Context == nil {
				next.ContextOmitted = true
				break
			}
		}
	}
	next.Count = len(next.Rows)
	// Duplicity describes simultaneous online sessions, never retained absences.
	counts := map[string]int{}
	for _, r := range next.Rows {
		if r.State == "ONLINE" && r.Login != "" {
			counts[r.Login]++
		}
	}
	for i := range next.Rows {
		next.Rows[i].Duplicates = 0
		if next.Rows[i].State == "ONLINE" {
			next.Rows[i].Duplicates = counts[next.Rows[i].Login]
		}
	}
	return OK
}
