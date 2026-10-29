package equipment

import (
	"testing"
	"time"
)

func TestL06T3RestartAndNegotiationBoundaries(t *testing.T) {
	at := time.Date(2026, 10, 28, 1, 0, 0, 0, time.UTC)
	health := func(stamp time.Time, seconds int64) *DeviceHealth {
		return &DeviceHealth{Started: PPTime(stamp), At: PPTime(stamp.Add(time.Second)), Uptime: PPInterval{Known: true, Min: seconds * int64(time.Second), Max: (seconds + 1) * int64(time.Second)}}
	}
	previous := health(at, 100)
	later := health(at.Add(time.Hour), 1000)
	if !DetectedDeviceReboot(previous, later) {
		t.Fatal("restart during long gap was missed because current uptime grew")
	}
	for _, good := range []*DeviceHealth{health(at.Add(time.Minute), 160), health(at, 100), health(at.Add(-time.Minute), 1), {Started: PPTime(at.Add(time.Hour)), Uptime: PPInterval{}}} {
		if DetectedDeviceReboot(previous, good) {
			t.Fatal("continuity/replay/reversed/absent became restart", good)
		}
	}
	up, full, half := true, true, false
	speed := uint64(100000000)
	a := &DevicePort{Running: &up, SpeedBPS: &speed, FullDuplex: &full}
	b := &DevicePort{Running: &up, SpeedBPS: &speed, FullDuplex: &half}
	if !PortNegotiationChanged(a, b) || PortNegotiationChanged(a, a) || PortNegotiationChanged(nil, b) {
		t.Fatal("half/full/unknown comparison")
	}
}
