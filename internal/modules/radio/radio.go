package radio

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

func radioFinite(n float64) bool    { return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 }
func radioSeconds(n float64) string { return strings.ReplaceAll(fmt.Sprintf("%.2f", n), ".", ",") }
func RadioLiveLimit(reading *RadioReading, cost RadioCost) bool {
	if reading == nil ||
		!radioFinite(reading.CommandSeconds) ||
		reading.CommandSeconds <= 0 ||
		reading.CommandSeconds > 3 ||
		reading.PingRTT == nil ||
		!radioFinite(*reading.PingRTT) ||
		*reading.PingRTT > cost.PingLimit ||
		len(reading.CPU) < 1 {
		return true
	}
	for _, cpu := range reading.CPU {
		if cpu < 0 || cpu > RadioMaxCPU {
			return true
		}
	}
	return false
}

// A full path may exceed three seconds; only each established-session command
// is limited. The caller retains the two-consecutive-violations state machine.
func RadioLiveTickViolation(ap, cpe *RadioReading, cost RadioCost) bool {
	return RadioLiveLimit(ap, cost) || cpe == nil || !radioFinite(cpe.CommandSeconds) || cpe.CommandSeconds <= 0 || cpe.CommandSeconds > 3
}

func IsRadioFamily(f RadioFamily) bool {
	return f == RadioRouterOSWirelessV6 || f == RadioAirOSV6 || f == RadioIntelbrasWOM || f == RadioUnsupported
}
func RadioMAC(s string) (string, bool) {
	s = strings.ToUpper(s)
	if len(s) != 17 {
		return "", false
	}
	for i, c := range s {
		if i%3 == 2 {
			if c != ':' {
				return "", false
			}
		} else if !strings.ContainsRune("0123456789ABCDEF", c) {
			return "", false
		}
	}
	if s == "00:00:00:00:00:00" || s == "FF:FF:FF:FF:FF:FF" {
		return "", false
	}
	return s, true
}
func RadioLinkPresent(link RadioLink, reading *RadioReading) bool {
	if reading == nil || !reading.Compatible || reading.DeviceID != link.AP {
		return false
	}
	for _, c := range reading.Clients {
		if c.MAC == link.MAC && c.Interface == link.Interface && c.BSSID == link.BSSID {
			return true
		}
	}
	return false
}
func RadioCostGate(samples []float64, cpu [][]int) RadioCost {
	c := RadioCost{Samples: append([]float64{}, samples...), Interval: 5, Reason: "Custo ainda não medido no caminho completo."}
	if len(samples) != 10 || len(cpu) != 10 {
		return c
	}
	times := append([]float64{}, samples...)
	for i, seconds := range times {
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || len(cpu[i]) < 3 {
			return c
		}
		for _, n := range cpu[i] {
			if n < 0 || n > 100 {
				return c
			}
		}
	}
	sort.Float64s(times)
	c.P95 = times[int(math.Ceil(.95*float64(len(times))))-1]
	c.Interval = math.Max(5, 3*c.P95)
	for i := range samples {
		for _, n := range cpu[i] {
			if n > RadioMaxCPU {
				c.Reason = "CPU do AP passou de 90%; use consulta pontual."
				return c
			}
		}
	}
	c.Enabled, c.Reason = true, "Custo medido em dez leituras."
	return c
}
func StampRadio(reading *RadioReading, device Device, credential uint64, at time.Time) {
	reading.DeviceName, reading.Address = device.Name, device.Address
	reading.Compatible = true
	reading.DeviceID,
		reading.Revision,
		reading.CredentialRevision,
		reading.Connection,
		reading.At = device.ID,
		device.Revision,
		credential,
		PPConnection(device),
		PPTime(at)
	stamp := func(metrics []RadioMetric, iface string) {
		for i := range metrics {
			m := &metrics[i]
			m.Perspective, m.DeviceID, m.Interface = reading.Perspective, device.ID, iface
			if m.Quality != "initial" || m.At == "" {
				m.At = reading.At
			}
			if m.Quality == "" {
				m.Quality = "observed"
			}
			if strings.HasPrefix(m.Name, "chain") {
				v := m.Value
				m.Best, m.Worst = &v, &v
			}
		}
	}
	stamp(reading.Metrics, "wireless")
	for i := range reading.Interfaces {
		stamp(reading.Interfaces[i].Metrics, reading.Interfaces[i].Name)
		stamp(reading.Interfaces[i].PowerMetrics, reading.Interfaces[i].Name)
	}
	for i := range reading.Clients {
		stamp(reading.Clients[i].Metrics, reading.Clients[i].Interface)
	}
}

// Extrema belong to this finite observation only; PHY rates never form a series.
func AccumulateRadioChains(previous, next *RadioReading) {
	if previous == nil ||
		next == nil ||
		previous.DeviceID != next.DeviceID ||
		previous.Connection != next.Connection ||
		previous.Revision != next.Revision ||
		previous.CredentialRevision != next.CredentialRevision {
		return
	}
	mergeRadioChainExtrema(previous.Metrics, next.Metrics)
	for i := range next.Clients {
		for _, old := range previous.Clients {
			if old.MAC == next.Clients[i].MAC && old.Interface == next.Clients[i].Interface && old.BSSID == next.Clients[i].BSSID {
				mergeRadioChainExtrema(old.Metrics, next.Clients[i].Metrics)
			}
		}
	}
}

func mergeRadioChainExtrema(before, after []RadioMetric) {
	for i := range after {
		m := &after[i]
		if !strings.HasPrefix(m.Name, "chain") {
			continue
		}
		for _, old := range before {
			if old.Name == m.Name && old.Direction == m.Direction && old.Interface == m.Interface {
				best, worst := m.Value, m.Value
				if old.Best != nil {
					best = math.Max(best, *old.Best)
				}
				if old.Worst != nil {
					worst = math.Min(worst, *old.Worst)
				}
				m.Best, m.Worst = &best, &worst
			}
		}
	}
}
