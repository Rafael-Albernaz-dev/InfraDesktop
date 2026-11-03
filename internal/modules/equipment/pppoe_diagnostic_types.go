package equipment

import (
	"context"
	"net/netip"
	"time"
)

const (
	PDCatalog    = "catalog"
	PDRadius     = "radius"
	PDPing       = "ping"
	PDMaxRadius  = 32
	PDPingBudget = 5 * time.Second
)

type PDRadiusCounters struct {
	Pending    string     `json:"pending"`
	Requests   string     `json:"requests"`
	Accepts    string     `json:"accepts"`
	Rejects    string     `json:"rejects"`
	Resends    string     `json:"resends"`
	Timeouts   string     `json:"timeouts"`
	BadReplies string     `json:"bad_replies"`
	LastRTT    PPInterval `json:"last_rtt"`
}

type PDPingProbe struct {
	Sequence int    `json:"sequence"`
	Status   string `json:"status"`
	RTT      *int64 `json:"rtt_ns,omitempty"`
}

type PDPingResult struct {
	Jitter          *int64        `json:"jitter_ns,omitempty"`
	Baseline        *int64        `json:"baseline_median_ns,omitempty"`
	BaselineSamples int           `json:"baseline_samples"`
	Sent            int           `json:"sent"`
	Received        int           `json:"received"`
	Loss            float64       `json:"loss_percent"`
	Min             *int64        `json:"min_ns,omitempty"`
	Avg             *int64        `json:"avg_ns,omitempty"`
	Max             *int64        `json:"max_ns,omitempty"`
	Probes          []PDPingProbe `json:"probes"`
	Context         string        `json:"context"`
}

type PDRow struct {
	NotSeenSince   string            `json:"not_seen_since,omitempty"`
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	RemoteID       string            `json:"remote_id,omitempty"`
	Name           string            `json:"name,omitempty"`
	Parent         string            `json:"parent,omitempty"`
	ParentID       string            `json:"parent_id,omitempty"`
	Relation       string            `json:"relation,omitempty"`
	Type           string            `json:"type,omitempty"`
	Running        *bool             `json:"running,omitempty"`
	Disabled       *bool             `json:"disabled,omitempty"`
	Dynamic        *bool             `json:"dynamic,omitempty"`
	Invalid        *bool             `json:"invalid,omitempty"`
	VLANID         int               `json:"vlan_id,omitempty"`
	Range          string            `json:"range,omitempty"`
	ServiceTag     *bool             `json:"service_tag,omitempty"`
	AcceptUntagged *bool             `json:"accept_untagged,omitempty"`
	Address        string            `json:"address,omitempty"`
	Services       string            `json:"services,omitempty"`
	AuthPort       int               `json:"auth_port,omitempty"`
	AccountingPort int               `json:"accounting_port,omitempty"`
	Protocol       string            `json:"protocol,omitempty"`
	UseRadius      *bool             `json:"use_radius,omitempty"`
	Accounting     *bool             `json:"accounting,omitempty"`
	Counters       *PDRadiusCounters `json:"counters,omitempty"`
	Ping           *PDPingResult     `json:"ping,omitempty"`
}

type PDDiagnostic struct {
	AbsentInterfaces  int          `json:"absent_interfaces,omitempty"`
	AbsencesLimited   bool         `json:"absences_limited,omitempty"`
	ID                string       `json:"diagnostic_id"`
	DeviceID          string       `json:"device_id"`
	Scope             string       `json:"scope"`
	Provenance        PPProvenance `json:"provenance"`
	Quality           PPQuality    `json:"quality"`
	SourceObservation string       `json:"source_observation,omitempty"`
	SourceRow         string       `json:"source_row,omitempty"`
	Count             int          `json:"count"`
	Bytes             int          `json:"bytes"`
	Rows              []PDRow      `json:"-"`
}

type PPPingTarget struct {
	Observation PPObservation
	Row         PPRow
	Age         PPInterval
	Prepared    time.Time
}

type PDDiagnosticRemote interface {
	CollectPPDiagnostic(
		context.Context, Device, string, string, string, *PPPingTarget, func() Code,
	) (*PDDiagnostic, Code)
}

type PDDiagnosticStore interface {
	ReservePPDiagnostic(context.Context, string, PPInput, string, string) (string, bool, Code)
	FinishPPDiagnostic(
		context.Context, string, PPTarget, PPInput, string, PPAttempt, *PDDiagnostic,
	) Code
	PPDiagnosticCurrent(context.Context, string, PPInput) (PPView, Code)
	PPDiagnosticPage(context.Context, string, PPInput) (PPView, Code)
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

func resolveSinglePDRow(row *PDRow, rows []PDRow, names, vlans map[string][]int) {
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

func walkVLANDepth(row *PDRow, rows []PDRow, names, vlans map[string][]int) string {
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
