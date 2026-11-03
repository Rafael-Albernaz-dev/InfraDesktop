package pppoe

import (
	"sort"
	"time"
)

// ReconcilePP marks absent items only after a completed, valid collection.
func ReconcilePP(previous, next *PPObservation) Code {
	if !validCompletedPP(next) {
		return Partial
	}
	now, err := time.Parse(time.RFC3339Nano, next.Provenance.Ended)
	if err != nil {
		return Invalid
	}
	compatible := PPCompatible(previous, next)
	previousKeys := collectPreviousOnlineKeys(previous, compatible)
	present, reconnected := markNextRowsOnline(next.Rows, previousKeys)
	offline := collectRetainedOfflineRows(previous, next, now, compatible, present, reconnected)
	sortOfflineRows(offline)
	room := PPMaxRows - len(next.Rows)
	if len(offline) > room {
		offline = offline[:room]
	}
	next.Online = len(next.Rows)
	next.Offline = len(offline)
	next.Rows = append(next.Rows, offline...)
	propagateContextOmitted(previous, next, offline)
	next.Count = len(next.Rows)
	assignOnlineDuplicateCounts(next.Rows)
	return OK
}

func validCompletedPP(next *PPObservation) bool {
	return next != nil &&
		next.Quality.Status == "complete" &&
		next.Quality.Terminated &&
		next.Quality.MembershipChecked &&
		len(next.Rows) <= PPMaxRows
}

func reconcileRowKey(r PPRow) string {
	return r.RemoteID + "\x00" + r.SessionID + "\x00" + r.Login
}

func collectPreviousOnlineKeys(previous *PPObservation, compatible bool) map[string]bool {
	previousKeys := map[string]bool{}
	if !compatible {
		return previousKeys
	}
	for _, r := range previous.Rows {
		if r.State != "OFFLINE" {
			previousKeys[reconcileRowKey(r)] = true
		}
	}
	return previousKeys
}

func markNextRowsOnline(
	rows []PPRow, previousKeys map[string]bool,
) (map[string]bool, map[string]bool) {
	present := map[string]bool{}
	reconnected := map[string]bool{}
	for i := range rows {
		r := &rows[i]
		r.State = "ONLINE"
		r.AbsentStart = ""
		r.AbsentEnd = ""
		k := reconcileRowKey(*r)
		present[k] = true
		if r.Login != "" && !previousKeys[k] {
			reconnected[r.Login] = true
		}
	}
	return present, reconnected
}

func collectRetainedOfflineRows(
	previous, next *PPObservation, now time.Time, compatible bool,
	present, reconnected map[string]bool,
) []PPRow {
	offline := []PPRow{}
	if !compatible {
		return offline
	}
	for _, r := range previous.Rows {
		if present[reconcileRowKey(r)] || reconnected[r.Login] {
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
	return offline
}

func sortOfflineRows(offline []PPRow) {
	sort.Slice(offline, func(i, j int) bool {
		a, _ := time.Parse(time.RFC3339Nano, offline[i].AbsentEnd)
		b, _ := time.Parse(time.RFC3339Nano, offline[j].AbsentEnd)
		if a.Equal(b) {
			return offline[i].ID < offline[j].ID
		}
		return a.After(b)
	})
}

func propagateContextOmitted(previous, next *PPObservation, offline []PPRow) {
	if previous == nil || !previous.ContextOmitted {
		return
	}
	for _, row := range offline {
		if row.Context == nil {
			next.ContextOmitted = true
			return
		}
	}
}

func assignOnlineDuplicateCounts(rows []PPRow) {
	counts := map[string]int{}
	for _, r := range rows {
		if r.State == "ONLINE" && r.Login != "" {
			counts[r.Login]++
		}
	}
	for i := range rows {
		rows[i].Duplicates = 0
		if rows[i].State == "ONLINE" {
			rows[i].Duplicates = counts[rows[i].Login]
		}
	}
}

// ReconcileCatalogAbsences retains missing dynamic interfaces for 24h after a complete catalog.
func ReconcileCatalogAbsences(previous, next *PDDiagnostic) Code {
	if !validCompletedCatalog(next) {
		return Partial
	}
	present := filterPresentCatalogRows(next)
	if previous == nil || previous.Scope != PDCatalog || !compatibleCatalogPair(previous, next) {
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
	absent := collectAbsentCatalogRows(previous, now, present)
	sortAbsentCatalogRows(absent)
	room := max(0, PPMaxRows-len(next.Rows))
	if len(absent) > room {
		absent = absent[:room]
		next.AbsencesLimited = true
	}
	next.AbsentInterfaces = len(absent)
	next.Rows = append(next.Rows, absent...)
	return OK
}

func validCompletedCatalog(next *PDDiagnostic) bool {
	return next != nil &&
		next.Scope == PDCatalog &&
		next.Quality.Status == "complete" &&
		next.Quality.Terminated &&
		next.Quality.MembershipChecked
}

func filterPresentCatalogRows(next *PDDiagnostic) map[string]bool {
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
	return present
}

func compatibleCatalogPair(previous, next *PDDiagnostic) bool {
	a := &PPObservation{
		DeviceID: previous.DeviceID, Scope: PPScope,
		Quality: previous.Quality, Provenance: previous.Provenance,
	}
	b := &PPObservation{
		DeviceID: next.DeviceID, Scope: PPScope,
		Quality: next.Quality, Provenance: next.Provenance,
	}
	return PPCompatible(a, b) && previous.Provenance.Revision == next.Provenance.Revision
}

func collectAbsentCatalogRows(
	previous *PDDiagnostic, now time.Time, present map[string]bool,
) []PDRow {
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
	return absent
}

func sortAbsentCatalogRows(absent []PDRow) {
	sort.Slice(absent, func(i, j int) bool {
		if absent[i].NotSeenSince == absent[j].NotSeenSince {
			return absent[i].ID < absent[j].ID
		}
		return absent[i].NotSeenSince > absent[j].NotSeenSince
	})
}
