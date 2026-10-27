package equipment

import (
	"context"
	"math"
	"sort"
	"time"
)

type PPRate struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}
type PPReadRow struct {
	Secret   *PPSecretEvidence `json:"secret,omitempty"`
	RowID    string            `json:"row_id,omitempty"`
	Kind     string            `json:"kind"`
	Name     string            `json:"name,omitempty"`
	Login    string            `json:"login,omitempty"`
	TX       *PPRate           `json:"tx,omitempty"`
	RX       *PPRate           `json:"rx,omitempty"`
	Running  *bool             `json:"running,omitempty"`
	Disabled *bool             `json:"disabled,omitempty"`
	Download *PPRate           `json:"download,omitempty"`
	Upload   *PPRate           `json:"upload,omitempty"`
	Uptime   PPInterval        `json:"uptime"`
	At       string            `json:"at,omitempty"`
	Event    string            `json:"event,omitempty"`
	Used     *uint64           `json:"used,omitempty"`
	Size     *uint64           `json:"size,omitempty"`
	RXErrors *string           `json:"rx_errors,omitempty"`
	TXErrors *string           `json:"tx_errors,omitempty"`
	RXDrops  *string           `json:"rx_drops,omitempty"`
	TXDrops  *string           `json:"tx_drops,omitempty"`
}

// A selected secret's closed projection is transient and never part of PPRow.
type PPSecretEvidence struct {
	Disabled   *bool   `json:"disabled,omitempty"`
	Service    *string `json:"service,omitempty"`
	Profile    *string `json:"profile,omitempty"`
	LastOut    *string `json:"last_logged_out,omitempty"`
	LastCaller *string `json:"last_caller_id,omitempty"`
	LastReason *string `json:"last_disconnect_reason,omitempty"`
	Comment    *string `json:"comment,omitempty"`
}
type PPReadCost struct {
	Commands   int     `json:"ssh_commands"`
	Bytes      int     `json:"ssh_bytes"`
	FirstCount int     `json:"first_tunnels"`
	LastCount  int     `json:"last_tunnels"`
	CPUStart   *uint64 `json:"cpu_start,omitempty"`
	CPUEnd     *uint64 `json:"cpu_end,omitempty"`
}
type PPReadResult struct {
	// Serialization sizes are measured by the adapter, not the domain.
	PayloadBytes     int         `json:"-"`
	MaxRowBytes      int         `json:"-"`
	MetadataBytes    int         `json:"-"`
	PoolsAvailable   *bool       `json:"pools_available,omitempty"`
	UplinksAvailable *bool       `json:"uplinks_available,omitempty"`
	PoolTotal        int         `json:"pool_total"`
	UplinkTotal      int         `json:"uplink_total"`
	Version          string      `json:"remote_version,omitempty"`
	Cost             PPReadCost  `json:"cost"`
	Scope            string      `json:"scope"`
	Started          string      `json:"started_at"`
	Ended            string      `json:"ended_at"`
	Duration         int64       `json:"duration_ns"`
	Rows             []PPReadRow `json:"rows"`
}
type PPReadTarget struct {
	Observation PPObservation
	Rows        []PPRow
	Age         PPInterval
	Prepared    time.Time
}
type PPReadRemote interface {
	ReadPP(context.Context, Device, string, string, string, *PPReadTarget, func(*PPReadResult) Code, func() Code) (*PPReadResult, Code)
}

func PPReadScope(scope string) bool {
	return scope == "bandwidth" || scope == "live" || scope == "top" || scope == "log" || scope == "health" || scope == "secret"
}
func PPReadBudget(scope string) time.Duration {
	if scope == "live" {
		return 60 * time.Second
	}
	return QueryBudget
}
func PPReadWithinBudget(v *PPReadResult) bool {
	return v != nil && len(v.Rows) <= PPMaxRows && v.PayloadBytes > 0 && v.PayloadBytes <= PPObservationBytes && v.MaxRowBytes >= 0 && v.MaxRowBytes <= PPRowBytes && (len(v.Rows) == 0 || v.MaxRowBytes > 0) && v.MetadataBytes > 0 && v.MetadataBytes <= 16*1024
}

// PPBandwidth subtracts counters exactly as uint64, then uses outward rounding
// at EVERY float conversion and arithmetic step. Identity checks precede zero.
func PPBandwidth(a, b PPTunnel) (download, upload *PPRate) {
	if a.ID == "" || a.ID != b.ID || a.Name != b.Name || a.Login != b.Login || a.Caller != b.Caller || a.Address != b.Address || a.RX == nil || a.TX == nil || b.RX == nil || b.TX == nil || *b.RX < *a.RX || *b.TX < *a.TX {
		return
	}
	x, y := a.Sample, b.Sample
	if !x.Known || !y.Known || x.Min < 0 || x.Max < x.Min || y.Min <= x.Max || y.Max < y.Min {
		return
	}
	age := PPInterval{Known: true, Min: y.Min - x.Max, Max: y.Max - x.Min}
	if !intervalAdvance(a.Uptime, b.Uptime, age) || !intervalAdvance(a.RouterUptime, b.RouterUptime, age) {
		return
	}
	rate := func(delta uint64) *PPRate {
		if delta == 0 {
			return &PPRate{}
		}
		down := func(v float64) float64 { return math.Nextafter(v, math.Inf(-1)) }
		up := func(v float64) float64 { return math.Nextafter(v, math.Inf(1)) }
		lo := down(down(down(float64(delta))*8000000000) / up(float64(age.Max)))
		hi := up(up(up(float64(delta))*8000000000) / down(float64(age.Min)))
		if math.IsInf(hi, 0) || math.IsNaN(hi) {
			return nil
		}
		return &PPRate{Min: math.Max(0, lo), Max: hi}
	}
	return rate(*b.TX - *a.TX), rate(*b.RX - *a.RX)
}

// MatchPPRead requires router continuity as well as the unique session tuple.
func MatchPPRead(target *PPReadTarget, tunnels []PPTunnel, age PPInterval) []PPTunnel {
	rows := append([]PPRow(nil), target.Rows...)
	filtered := append([]PPTunnel(nil), tunnels...)
	for i := range filtered {
		if !intervalAdvance(target.Observation.Provenance.RouterUptime, filtered[i].RouterUptime, age) {
			filtered[i].Uptime.Known = false
		}
	}
	indexes := AssociatePPTunnels(rows, filtered, age)
	out := make([]PPTunnel, len(rows))
	for i, n := range indexes {
		if n >= 0 {
			out[i] = filtered[n]
		}
	}
	return out
}

// Known lower bounds sort first; unknown never participates as numeric zero.
func SortPPConsumption(rows []PPReadRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		ak, bk := a.Download != nil && a.Upload != nil, b.Download != nil && b.Upload != nil
		if ak != bk {
			return ak
		}
		if ak {
			x, y := a.Download.Min+a.Upload.Min, b.Download.Min+b.Upload.Min
			if x != y {
				return x > y
			}
		}
		if a.Login != b.Login {
			return a.Login < b.Login
		}
		return a.Name < b.Name
	})
}
