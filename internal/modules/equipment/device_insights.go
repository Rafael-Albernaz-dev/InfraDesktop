package equipment

import "time"

// Physical negotiation is independent of its transport. Missing fields are not
// zero, half duplex or DOWN. LinkDowns stays decimal text to preserve uint64 in JS.
type DevicePort struct {
	Running    *bool   `json:"running,omitempty"`
	SpeedBPS   *uint64 `json:"speed_bps,omitempty"`
	FullDuplex *bool   `json:"full_duplex,omitempty"`
	LinkDowns  *string `json:"link_downs,omitempty"`
}

// Count at most one restart in a saved interval. Disjoint uptime uncertainty is
// necessary; wall-clock reversal and unavailable data cannot prove a restart.
func DetectedDeviceReboot(previous, current *DeviceHealth) bool {
	if previous == nil || current == nil || !previous.Uptime.Known || !current.Uptime.Known || previous.Uptime.Min < 0 || current.Uptime.Min < 0 || previous.Uptime.Max < previous.Uptime.Min || current.Uptime.Max < current.Uptime.Min {
		return false
	}
	a, e := time.Parse(time.RFC3339Nano, previous.At)
	if e != nil {
		return false
	}
	b, e := time.Parse(time.RFC3339Nano, current.Started)
	if e != nil || b.Before(a) {
		return false
	}
	delta := b.Sub(a).Nanoseconds()
	return current.Uptime.Max < previous.Uptime.Min+delta
}
func PortNegotiationChanged(a, b *DevicePort) bool {
	if a == nil || b == nil || a.Running == nil || b.Running == nil {
		return false
	}
	if *a.Running != *b.Running {
		return true
	}
	if !*a.Running {
		return false
	}
	return (a.SpeedBPS != nil && b.SpeedBPS != nil && *a.SpeedBPS != *b.SpeedBPS) || (a.FullDuplex != nil && b.FullDuplex != nil && *a.FullDuplex != *b.FullDuplex)
}
