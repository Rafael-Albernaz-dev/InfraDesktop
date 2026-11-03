package pppoe

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

type PPTrafficAccumulator struct {
	down, up         uint64
	covered          PPInterval
	gaps             int
	overflow         bool
	points           []PPTrafficPoint
	peakDown, peakUp *PPRate
}

func PPTunnelMatches(row PPRow, t PPTunnel, age PPInterval) bool {
	if row.State == "OFFLINE" || row.Login == "" || row.Caller == "" || row.Address == "" {
		return false
	}
	if row.Login != t.Login || !strings.EqualFold(row.Caller, t.Caller) || row.Address != t.Address {
		return false
	}
	return intervalAdvance(row.Uptime, t.Uptime, age)
}

// AssociatePPTunnels enforces a one-to-one relation in both directions.
func AssociatePPTunnels(rows []PPRow, tunnels []PPTunnel, age PPInterval) []int {
	matches := make([]int, len(rows))
	uses := make([]int, len(tunnels))
	for i, row := range rows {
		matches[i] = -1
		count := 0
		for n, tunnel := range tunnels {
			if PPTunnelMatches(row, tunnel, age) {
				count++
				matches[i] = n
				uses[n]++
			}
		}
		if count != 1 {
			matches[i] = -1
		}
	}
	applyUniqueTunnelContexts(rows, tunnels, matches, uses)
	return matches
}

func applyUniqueTunnelContexts(rows []PPRow, tunnels []PPTunnel, matches, uses []int) {
	for i, n := range matches {
		rows[i].Context = nil
		if n < 0 || uses[n] != 1 {
			matches[i] = -1
			continue
		}
		value := tunnels[n].Context
		value.Association = "associated"
		addresses := 0
		for _, row := range rows {
			if row.Address == rows[i].Address {
				addresses++
			}
		}
		if addresses != 1 {
			value.UploadLimit = ""
			value.DownloadLimit = ""
		}
		rows[i].Context = &value
	}
}

func PPReadScope(scope string) bool {
	switch scope {
	case "traffic", "operations", "bandwidth", "live", "top", "log", "health", "secret":
		return true
	default:
		return false
	}
}

func PPReadBudget(scope string) time.Duration {
	if scope == "live" || scope == "traffic" {
		return 60 * time.Second
	}
	return QueryBudget
}

func PPReadWithinBudget(v *PPReadResult) bool {
	if v == nil || len(v.Rows) > PPMaxRows {
		return false
	}
	if v.PayloadBytes <= 0 || v.PayloadBytes > PPObservationBytes {
		return false
	}
	if v.MaxRowBytes < 0 || v.MaxRowBytes > PPRowBytes || (len(v.Rows) > 0 && v.MaxRowBytes <= 0) {
		return false
	}
	return v.MetadataBytes > 0 && v.MetadataBytes <= 16*1024
}

// PPBandwidth subtracts counters exactly as uint64, then uses outward rounding.
func PPBandwidth(a, b PPTunnel) (download, upload *PPRate) {
	if !compatibleBandwidthTunnels(a, b) {
		return nil, nil
	}
	x, y := a.Sample, b.Sample
	if !x.Known || !y.Known || x.Min < 0 || x.Max < x.Min || y.Min <= x.Max || y.Max < y.Min {
		return nil, nil
	}
	age := PPInterval{Known: true, Min: y.Min - x.Max, Max: y.Max - x.Min}
	if !intervalAdvance(a.Uptime, b.Uptime, age) || !intervalAdvance(a.RouterUptime, b.RouterUptime, age) {
		return nil, nil
	}
	return rateFromBytes(*b.TX-*a.TX, age), rateFromBytes(*b.RX-*a.RX, age)
}

func compatibleBandwidthTunnels(a, b PPTunnel) bool {
	if a.ID == "" || a.ID != b.ID || a.Name != b.Name ||
		a.Login != b.Login || a.Caller != b.Caller || a.Address != b.Address {
		return false
	}
	if a.RX == nil || a.TX == nil || b.RX == nil || b.TX == nil {
		return false
	}
	return *b.RX >= *a.RX && *b.TX >= *a.TX
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

// SortPPConsumption sorts known lower bounds first; unknown never participates as zero.
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

func rateFromBytes(delta uint64, age PPInterval) *PPRate {
	if age.Min <= 0 || age.Max < age.Min {
		return nil
	}
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

func trafficPeak(old, v *PPRate) *PPRate {
	if v == nil {
		return old
	}
	if old == nil {
		copyRate := *v
		return &copyRate
	}
	return &PPRate{Min: math.Max(old.Min, v.Min), Max: math.Max(old.Max, v.Max)}
}

// Add owns its accumulator, returning an immutable snapshot on each sample.
func (a *PPTrafficAccumulator) Add(first, second PPTunnel) *PPTraffic {
	d, u := PPBandwidth(first, second)
	point := PPTrafficPoint{T: second.Sample.Max / int64(time.Millisecond)}
	if d == nil || u == nil {
		a.gaps++
		point.Gap = true
	} else {
		a.accumulateSample(first, second, d, u, &point)
	}
	if n := len(a.points); n > 0 && a.points[n-1].T/5000 == point.T/5000 {
		point.Gap = point.Gap || a.points[n-1].Gap
		a.points[n-1] = point
	} else {
		a.points = append(a.points, point)
	}
	out := &PPTraffic{
		Points:        append([]PPTrafficPoint{}, a.points...),
		DownloadBytes: strconv.FormatUint(a.down, 10),
		UploadBytes:   strconv.FormatUint(a.up, 10),
		CoveredNS:     a.covered.Max,
		CoveredMinNS:  a.covered.Min,
		Gaps:          a.gaps,
		Overflow:      a.overflow,
		PeakDownload:  a.peakDown,
		PeakUpload:    a.peakUp,
	}
	if !a.overflow {
		out.AverageDownload = rateFromBytes(a.down, a.covered)
		out.AverageUpload = rateFromBytes(a.up, a.covered)
	}
	return out
}

func (a *PPTrafficAccumulator) accumulateSample(
	first, second PPTunnel, d, u *PPRate, point *PPTrafficPoint,
) {
	deltaD, deltaU := *second.TX-*first.TX, *second.RX-*first.RX
	lo, hi := second.Sample.Min-first.Sample.Max, second.Sample.Max-first.Sample.Min
	if math.MaxUint64-a.down < deltaD || math.MaxUint64-a.up < deltaU ||
		math.MaxInt64-a.covered.Min < lo || math.MaxInt64-a.covered.Max < hi {
		a.overflow = true
		a.gaps++
		point.Gap = true
		return
	}
	a.down += deltaD
	a.up += deltaU
	a.covered.Min += lo
	a.covered.Max += hi
	a.peakDown = trafficPeak(a.peakDown, d)
	a.peakUp = trafficPeak(a.peakUp, u)
	dv, uv := math.Floor(d.Min*100)/100, math.Floor(u.Min*100)/100
	point.D, point.U = &dv, &uv
}

func TrafficLoadAbort(v TrafficLoadSafety) bool {
	return v.HealthDuration > 10*time.Second ||
		v.TimedOut ||
		v.CPU == nil ||
		*v.CPU > 60 ||
		v.PingSent <= 0 ||
		v.PingReceived != v.PingSent
}
