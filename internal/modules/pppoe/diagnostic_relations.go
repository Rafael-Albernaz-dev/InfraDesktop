package pppoe

import (
	"math"
	"net/netip"
	"time"
)

func PPPingBounds(
	target *PPPingTarget, elapsed time.Duration,
) (router, session, age PPInterval, ok bool) {
	if target == nil || target.Row.State == "OFFLINE" || elapsed < 0 {
		return
	}
	age = target.Age
	advance := elapsed.Nanoseconds()
	if !age.Known || age.Min < 0 || age.Max < age.Min ||
		age.Max > math.MaxInt64-advance-int64(QueryBudget) {
		return
	}
	age.Min += advance
	age.Max += advance + int64(QueryBudget)
	var valid bool
	router, valid = advancePingInterval(target.Observation.Provenance.RouterUptime, age)
	if !valid {
		return
	}
	session, ok = advancePingInterval(target.Row.Uptime, age)
	return
}

func advancePingInterval(old, age PPInterval) (PPInterval, bool) {
	if !old.Known || old.Min < 0 || old.Max < old.Min ||
		old.Max > math.MaxInt64-age.Max-int64(time.Second) {
		return PPInterval{}, false
	}
	return PPInterval{Known: true, Min: old.Min + age.Min, Max: old.Max + age.Max}, true
}

func PDValidScope(scope string) bool {
	return scope == PDCatalog || scope == PDRadius || scope == PDPing
}

func PPPingAddress(row PPRow) (string, bool) {
	ip, err := netip.ParseAddr(row.Address)
	if err != nil {
		return "", false
	}
	ip = ip.Unmap()
	if !ip.Is4() || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return "", false
	}
	return ip.String(), true
}

func PPPingStable(target *PPPingTarget, fresh *PPObservation, elapsed time.Duration) bool {
	if !compatiblePingObservation(target, fresh) {
		return false
	}
	age := target.Age
	advance := elapsed.Nanoseconds()
	if !age.Known || advance < 0 || age.Min < 0 || age.Max < age.Min ||
		age.Max > int64(^uint64(0)>>1)-advance {
		return false
	}
	age.Min += advance
	age.Max += advance
	if !intervalAdvance(target.Observation.Provenance.RouterUptime, fresh.Provenance.RouterUptime, age) {
		return false
	}
	address, ok := PPPingAddress(target.Row)
	if !ok {
		return false
	}
	return uniqueStablePingRow(target.Row, fresh.Rows, address, age)
}

func compatiblePingObservation(target *PPPingTarget, fresh *PPObservation) bool {
	return target != nil &&
		fresh != nil &&
		fresh.Quality.Status == "complete" &&
		target.Observation.Provenance.Version == fresh.Provenance.Version &&
		target.Observation.Provenance.Family == fresh.Provenance.Family
}

func uniqueStablePingRow(targetRow PPRow, rows []PPRow, address string, age PPInterval) bool {
	matches, addresses := 0, 0
	for _, row := range rows {
		if row.Address == address {
			addresses++
		}
		if row.RemoteID == targetRow.RemoteID && row.SessionID == targetRow.SessionID {
			if row.Address != address || !intervalAdvance(targetRow.Uptime, row.Uptime, age) {
				return false
			}
			matches++
		}
	}
	return matches == 1 && addresses == 1
}

// ResolvePDRelations resolves configuration references only; no session is an input.
func ResolvePDRelations(rows []PDRow) {
	names := map[string][]int{}
	vlans := map[string][]int{}
	for i, row := range rows {
		if row.Kind == "interface" {
			names[row.Name] = append(names[row.Name], i)
		}
		if row.Kind == "vlan" {
			vlans[row.Name] = append(vlans[row.Name], i)
		}
	}
	for i := range rows {
		resolveSinglePDRow(&rows[i], rows, names, vlans)
	}
}

func resolveSinglePDRow(
	row *PDRow, rows []PDRow, names, vlans map[string][]int,
) {
	if row.Kind != "vlan" && row.Kind != "service" {
		return
	}
	row.ParentID = ""
	parents := names[row.Parent]
	row.Relation = "missing"
	if len(parents) > 1 {
		row.Relation = "ambiguous"
		return
	}
	if len(parents) == 0 {
		return
	}
	row.ParentID = rows[parents[0]].ID
	row.Relation = walkVLANDepth(row, rows, names, vlans)
	if row.Relation != "resolved" {
		row.ParentID = ""
	}
}

func walkVLANDepth(
	row *PDRow, rows []PDRow, names, vlans map[string][]int,
) string {
	seen := map[string]bool{}
	name := row.Name
	if row.Kind == "service" {
		name = row.Parent
	}
	for depth := 0; ; depth++ {
		if depth >= 32 {
			return "depth_limited"
		}
		if seen[name] {
			return "cycle"
		}
		seen[name] = true
		candidates := vlans[name]
		if len(candidates) > 1 {
			return "ambiguous"
		}
		if len(candidates) == 0 {
			return "resolved"
		}
		parent := rows[candidates[0]].Parent
		if len(names[parent]) == 0 {
			return "missing"
		}
		if len(names[parent]) > 1 {
			return "ambiguous"
		}
		name = parent
	}
}

func PDCompatibleExport(target *PPObservation, diagnostic *PDDiagnostic) bool {
	if target == nil || diagnostic == nil || target.DeviceID != diagnostic.DeviceID {
		return false
	}
	p, q := target.Provenance, diagnostic.Provenance
	return q.Installation == p.Installation &&
		q.Company == p.Company &&
		q.Connection == p.Connection &&
		q.Revision == p.Revision &&
		q.CredentialRevision == p.CredentialRevision &&
		q.Family == p.Family &&
		q.Version == p.Version &&
		(diagnostic.Scope != PDPing || diagnostic.SourceObservation == target.ID)
}
