package equipment

import (
	"math"
	"strconv"
	"time"
)

const TrafficRetention = 5 * time.Minute

type PPTrafficPoint struct {
	T   int64    `json:"t"`
	D   *float64 `json:"d,omitempty"`
	U   *float64 `json:"u,omitempty"`
	Gap bool     `json:"gap,omitempty"`
}
type PPTraffic struct {
	Points          []PPTrafficPoint `json:"points"`
	DownloadBytes   string           `json:"download_bytes"`
	UploadBytes     string           `json:"upload_bytes"`
	CoveredNS       int64            `json:"covered_ns"`
	CoveredMinNS    int64            `json:"covered_min_ns"`
	Gaps            int              `json:"gaps"`
	Overflow        bool             `json:"overflow,omitempty"`
	AverageDownload *PPRate          `json:"average_download,omitempty"`
	AverageUpload   *PPRate          `json:"average_upload,omitempty"`
	PeakDownload    *PPRate          `json:"peak_download,omitempty"`
	PeakUpload      *PPRate          `json:"peak_upload,omitempty"`
}
type PPTrafficAccumulator struct {
	down, up         uint64
	covered          PPInterval
	gaps             int
	overflow         bool
	points           []PPTrafficPoint
	peakDown, peakUp *PPRate
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
		copy := *v
		return &copy
	}
	return &PPRate{Min: math.Max(old.Min, v.Min), Max: math.Max(old.Max, v.Max)}
}

// Add owns its accumulator, but every returned snapshot is immutable. No
// point/rate from a previous publication is ever changed by a later sample.
func (a *PPTrafficAccumulator) Add(first, second PPTunnel) *PPTraffic {
	d, u := PPBandwidth(first, second)
	point := PPTrafficPoint{T: second.Sample.Max / int64(time.Millisecond)}
	if d == nil || u == nil {
		a.gaps++
		point.Gap = true
	} else {
		deltaD, deltaU := *second.TX-*first.TX, *second.RX-*first.RX
		lo, hi := second.Sample.Min-first.Sample.Max, second.Sample.Max-first.Sample.Min
		if math.MaxUint64-a.down < deltaD || math.MaxUint64-a.up < deltaU || math.MaxInt64-a.covered.Min < lo || math.MaxInt64-a.covered.Max < hi {
			a.overflow = true
			a.gaps++
			point.Gap = true
		} else {
			a.down += deltaD
			a.up += deltaU
			a.covered.Min += lo
			a.covered.Max += hi
			a.peakDown = trafficPeak(a.peakDown, d)
			a.peakUp = trafficPeak(a.peakUp, u)
			// Precision here is only for the small plot, never for totals/statistics.
			dv, uv := math.Floor(d.Min*100)/100, math.Floor(u.Min*100)/100
			point.D, point.U = &dv, &uv
		}
	}
	if n := len(a.points); n > 0 && a.points[n-1].T/5000 == point.T/5000 {
		point.Gap = point.Gap || a.points[n-1].Gap
		a.points[n-1] = point
	} else {
		a.points = append(a.points, point)
	}
	out := &PPTraffic{Points: append([]PPTrafficPoint{}, a.points...), DownloadBytes: strconv.FormatUint(a.down, 10), UploadBytes: strconv.FormatUint(a.up, 10), CoveredNS: a.covered.Max, CoveredMinNS: a.covered.Min, Gaps: a.gaps, Overflow: a.overflow, PeakDownload: a.peakDown, PeakUpload: a.peakUp}
	if !a.overflow {
		out.AverageDownload = rateFromBytes(a.down, a.covered)
		out.AverageUpload = rateFromBytes(a.up, a.covered)
	}
	return out
}

// Load measurements use the same fixed contract on both firmwares. Unknown
// safety measurements are a refusal, not an invented healthy zero.
type TrafficLoadSafety struct {
	HealthDuration         time.Duration
	CPU                    *uint64
	PingSent, PingReceived int
	TimedOut               bool
}

func TrafficLoadAbort(v TrafficLoadSafety) bool {
	return v.HealthDuration > 10*time.Second || v.TimedOut || v.CPU == nil || *v.CPU > 60 || v.PingSent <= 0 || v.PingReceived != v.PingSent
}
