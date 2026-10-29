package equipment

import (
	"math"
	"testing"
	"time"
)

func trafficPair() (PPTunnel, PPTunnel) {
	ptr := func(n uint64) *uint64 { return &n }
	a := PPTunnel{ID: "a", Name: "tunnel", Login: "synthetic", Caller: "caller", Address: "198.51.100.1", RX: ptr(100), TX: ptr(200), Sample: PPInterval{Known: true, Min: int64(time.Second), Max: int64(1100 * time.Millisecond)}, Uptime: PPInterval{Known: true, Min: int64(time.Hour), Max: int64(time.Hour + time.Second)}, RouterUptime: PPInterval{Known: true, Min: int64(24 * time.Hour), Max: int64(24*time.Hour + time.Second)}}
	b := a
	b.RX = ptr(1100)
	b.TX = ptr(2200)
	b.Sample = PPInterval{Known: true, Min: int64(3100 * time.Millisecond), Max: int64(3300 * time.Millisecond)}
	b.Uptime.Min += int64(2 * time.Second)
	b.Uptime.Max += int64(2 * time.Second)
	b.RouterUptime.Min += int64(2 * time.Second)
	b.RouterUptime.Max += int64(2 * time.Second)
	return a, b
}
func TestL06TTrafficSummaryExactDirectionGapsAndImmutablePublication(t *testing.T) {
	a, b := trafficPair()
	acc := PPTrafficAccumulator{}
	first := acc.Add(a, b)
	if first.DownloadBytes != "2000" || first.UploadBytes != "1000" || first.AverageDownload == nil || first.AverageDownload.Min > 16000.0/2.3 || first.AverageDownload.Max < 8000 || first.Gaps != 0 {
		t.Fatal("wrong byte totals or coverage", first)
	}
	peak := *first.PeakDownload
	point := first.Points[0]
	reset := b
	n := uint64(1)
	reset.TX = &n
	reset.Sample = PPInterval{Known: true, Min: int64(4 * time.Second), Max: int64(4100 * time.Millisecond)}
	second := acc.Add(b, reset)
	if second.Gaps != 1 || second.DownloadBytes != "2000" || !second.Points[0].Gap || second.PeakDownload.Min != peak.Min {
		t.Fatal("reset became zero or lost sampled peak", second)
	}
	if first.Gaps != 0 || first.Points[0].Gap || first.Points[0].T != point.T || first.PeakDownload.Min != peak.Min {
		t.Fatal("later sample mutated published snapshot")
	}
	for _, change := range []string{"reconnect", "missing", "overlap"} {
		x, y := trafficPair()
		switch change {
		case "reconnect":
			y.ID = "new"
		case "missing":
			y.RX = nil
		case "overlap":
			y.Sample = x.Sample
		}
		fresh := PPTrafficAccumulator{}
		r := fresh.Add(x, y)
		if r.Gaps != 1 || r.AverageDownload != nil || r.CoveredNS != 0 {
			t.Fatal("uncertainty became observed zero", change, r)
		}
	}
	zero := a
	zero.Sample = b.Sample
	zero.Uptime = b.Uptime
	zero.RouterUptime = b.RouterUptime
	fresh := PPTrafficAccumulator{}
	r := fresh.Add(a, zero)
	if r.AverageDownload == nil || r.AverageDownload.Max != 0 || r.Gaps != 0 {
		t.Fatal("valid zero lost", r)
	}
	full := PPTrafficAccumulator{down: math.MaxUint64}
	r = full.Add(a, b)
	if !r.Overflow || r.AverageDownload != nil || r.Gaps != 1 {
		t.Fatal("overflow silently wrapped", r)
	}
}
func TestL06TTrafficLoadAbortAtExactContractBoundaries(t *testing.T) {
	cpu := uint64(60)
	v := TrafficLoadSafety{HealthDuration: 10 * time.Second, CPU: &cpu, PingSent: 3, PingReceived: 3}
	if TrafficLoadAbort(v) {
		t.Fatal("valid boundary rejected")
	}
	for _, why := range []string{"health", "timeout", "cpu", "loss", "unknown"} {
		x := v
		switch why {
		case "health":
			x.HealthDuration++
		case "timeout":
			x.TimedOut = true
		case "cpu":
			n := uint64(61)
			x.CPU = &n
		case "loss":
			x.PingReceived = 2
		case "unknown":
			x.CPU = nil
		}
		if !TrafficLoadAbort(x) {
			t.Fatal("unsafe load was admitted", why)
		}
	}
}
func TestL06TTrafficRetentionDoesNotExpandOtherScopes(t *testing.T) {
	now := time.Now()
	s := Service{now: func() time.Time { return now }, jobs: map[string]*runningJob{}}
	for _, scope := range []string{"traffic", "live", "secret"} {
		s.jobs[scope] = &runningJob{released: true, finished: now.Add(-3 * time.Minute), ppInput: PPInput{Scope: scope}}
	}
	s.prune()
	if len(s.jobs) != 1 || s.jobs["traffic"] == nil {
		t.Fatal("retention was expanded globally", s.jobs)
	}
	now = now.Add(2 * time.Minute)
	s.prune()
	if len(s.jobs) != 0 {
		t.Fatal("traffic cache unbounded")
	}
}
