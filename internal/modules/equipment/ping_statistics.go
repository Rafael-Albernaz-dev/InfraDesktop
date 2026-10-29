package equipment

import (
	"sort"
	"strconv"
	"strings"
)

func PingJitter(p *PDPingResult) *int64 {
	if p == nil || p.Sent != 3 || len(p.Probes) != 3 {
		return nil
	}
	var total uint64
	var pairs int64
	for i := 1; i < len(p.Probes); i++ {
		a, b := p.Probes[i-1], p.Probes[i]
		if a.RTT == nil || b.RTT == nil || *a.RTT < 0 || *b.RTT < 0 || b.Sequence != a.Sequence+1 {
			continue
		}
		if *a.RTT > *b.RTT {
			total += uint64(*a.RTT - *b.RTT)
		} else {
			total += uint64(*b.RTT - *a.RTT)
		}
		pairs++
	}
	if pairs == 0 {
		return nil
	}
	v := int64(total / uint64(pairs))
	return &v
}
func PingMedianTen(history []int64) *int64 {
	if len(history) < 10 {
		return nil
	}
	v := append([]int64(nil), history[len(history)-10:]...)
	for _, n := range v {
		if n < 0 {
			return nil
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	m := v[4]/2 + v[5]/2 + (v[4]%2+v[5]%2)/2
	return &m
}
func (s *Service) pingStatsKey(j *runningJob) string {
	if j.ping == nil {
		return ""
	}
	return strings.Join([]string{j.command.Scope.Profile, j.command.Scope.Activation, strconv.FormatUint(j.command.Authority, 10), j.device.ID, PPConnection(j.device), strconv.FormatUint(j.credentialRevision, 10), j.ping.Row.Address, j.ping.Row.RemoteID, j.ping.Row.SessionID, j.ping.Row.Login, j.ping.Row.Caller}, "\x00")
}

// Caller holds Service.mu under current authorization during publication.
func (s *Service) decoratePingLocked(j *runningJob, o *PDDiagnostic) {
	if o == nil || o.Scope != PDPing {
		return
	}
	key := s.pingStatsKey(j)
	history := append([]int64(nil), s.pingHistory[key]...)
	for i := range o.Rows {
		p := o.Rows[i].Ping
		if p != nil {
			p.Jitter = PingJitter(p)
			p.Baseline = PingMedianTen(history)
			p.BaselineSamples = len(history)
		}
	}
}
func (s *Service) rememberPingLocked(j *runningJob, o *PDDiagnostic) {
	if o == nil || o.Scope != PDPing {
		return
	}
	key := s.pingStatsKey(j)
	if key == "" {
		return
	}
	for _, r := range o.Rows {
		p := r.Ping
		if p == nil || p.Sent != 3 || p.Received != 3 || p.Avg == nil || *p.Avg < 0 {
			continue
		}
		if s.pingHistory == nil {
			s.pingHistory = map[string][]int64{}
		}
		if _, exists := s.pingHistory[key]; !exists {
			s.pingOrder = append(s.pingOrder, key)
			if len(s.pingOrder) > MaxDevices {
				delete(s.pingHistory, s.pingOrder[0])
				s.pingOrder = s.pingOrder[1:]
			}
		}
		h := append(s.pingHistory[key], *p.Avg)
		if len(h) > 10 {
			h = h[len(h)-10:]
		}
		s.pingHistory[key] = h
	}
}
