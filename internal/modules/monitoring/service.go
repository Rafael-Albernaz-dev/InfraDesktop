// Package monitoring owns device health, port negotiation, and monitoring series rules.
package monitoring

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"time"
)

type (
	Authorizer            = equipment.Authorizer
	Code                  = equipment.Code
	Command               = equipment.Command
	Device                = equipment.Device
	DeviceHealth          = equipment.DeviceHealth
	DeviceMonitoringPoint = equipment.DeviceMonitoringPoint
	DeviceMonitoringStore = equipment.DeviceMonitoringStore
	DevicePort            = equipment.DevicePort
	PPInput               = equipment.PPInput
	PPInterval            = equipment.PPInterval
	PPObservation         = equipment.PPObservation
	PPProvenance          = equipment.PPProvenance
	PPReadResult          = equipment.PPReadResult
	PPView                = equipment.PPView
)

const (
	OK              = equipment.OK
	Invalid         = equipment.Invalid
	Changed         = equipment.Changed
	Unavailable     = equipment.Unavailable
	Partial         = equipment.Partial
	CollectorDefect = equipment.CollectorDefect
)

// Service executes authorized device monitoring queries.
type Service struct {
	auth  Authorizer
	store any
}

func New(auth Authorizer, store any) *Service {
	return &Service{auth: auth, store: store}
}

func (s *Service) Handle(ctx context.Context, q Command, in PPInput) (PPView, Code) {
	store, ok := s.store.(DeviceMonitoringStore)
	if !ok {
		return PPView{}, Unavailable
	}
	return store.DeviceMonitoring(ctx, q.Scope.Profile, in)
}

// PPMonitoringContinuous rejects reboot/clock/context ambiguity before it can
// become a session-drop or a flap. Failed reads are separately fenced by store.
func PPMonitoringContinuous(a, b *PPObservation) bool {
	if !equipment.PPCompatible(a, b) || !sameMonitoringEpoch(a, b) {
		return false
	}
	wall := ppWallElapsed(a.Provenance, b.Provenance)
	elapsed := ppJointElapsed(a.Provenance.Sample, b.Provenance.Sample, wall)
	return intervalAdvance(a.Provenance.RouterUptime, b.Provenance.RouterUptime, elapsed)
}

func intervalAdvance(a, b, elapsed PPInterval) bool {
	if !a.Known || !b.Known || !elapsed.Known {
		return false
	}
	if a.Min < 0 || b.Min < 0 || a.Max < a.Min || b.Max < b.Min || elapsed.Min < 0 || elapsed.Max < elapsed.Min {
		return false
	}
	return b.Min > elapsed.Max && b.Min-a.Max <= elapsed.Max && b.Max-a.Min >= elapsed.Min
}

func ppWallElapsed(p, q PPProvenance) PPInterval {
	a0, e0 := time.Parse(time.RFC3339Nano, p.Started)
	a1, e1 := time.Parse(time.RFC3339Nano, p.Ended)
	b0, e2 := time.Parse(time.RFC3339Nano, q.Started)
	b1, e3 := time.Parse(time.RFC3339Nano, q.Ended)
	if e0 != nil || e1 != nil || e2 != nil || e3 != nil || a1.Before(a0) || b1.Before(b0) {
		return PPInterval{}
	}
	return PPInterval{Min: b0.Sub(a1).Nanoseconds(), Max: b1.Sub(a0).Nanoseconds(), Known: true}
}

func ppJointElapsed(a, b, wall PPInterval) PPInterval {
	if !a.Known || !b.Known || !wall.Known || a.Min < 0 || a.Max < a.Min || b.Min < 0 || b.Max < b.Min {
		return PPInterval{}
	}
	elapsed := PPInterval{Min: b.Min - a.Max, Max: b.Max - a.Min, Known: true}
	if elapsed.Min > wall.Max || wall.Min > elapsed.Max {
		return PPInterval{}
	}
	if wall.Min < elapsed.Min {
		elapsed.Min = wall.Min
	}
	if wall.Max > elapsed.Max {
		elapsed.Max = wall.Max
	}
	return elapsed
}

func sameMonitoringEpoch(a, b *PPObservation) bool {
	return a.Provenance.Revision == b.Provenance.Revision &&
		a.Provenance.Epoch != "" &&
		a.Provenance.Epoch == b.Provenance.Epoch
}

func PPMonitoringDrop(previous, current int) bool {
	return previous > 0 && previous-current >= 5 && current*100 <= previous*80
}

func MonitoringRetention(at time.Time) (minuteCutoff, hourCutoff, transitionCutoff int64) {
	day := at.Add(-24 * time.Hour).Unix()
	week := at.Add(-7 * 24 * time.Hour).Unix()
	return day, week, day
}

// DetectedDeviceReboot counts at most one restart in a saved interval.
func DetectedDeviceReboot(previous, current *DeviceHealth) bool {
	if !validRebootUptimes(previous, current) {
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

func validRebootUptimes(previous, current *DeviceHealth) bool {
	if previous == nil || current == nil || !previous.Uptime.Known || !current.Uptime.Known {
		return false
	}
	return previous.Uptime.Min >= 0 &&
		current.Uptime.Min >= 0 &&
		previous.Uptime.Max >= previous.Uptime.Min &&
		current.Uptime.Max >= current.Uptime.Min
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
	speedChanged := a.SpeedBPS != nil && b.SpeedBPS != nil && *a.SpeedBPS != *b.SpeedBPS
	duplexChanged := a.FullDuplex != nil && b.FullDuplex != nil && *a.FullDuplex != *b.FullDuplex
	return speedChanged || duplexChanged
}

func ValidCollectorReason(code Code, reason string) bool {
	return equipment.ValidCollectorReason(code, reason)
}
