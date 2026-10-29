package equipment

import (
	"context"
	"time"
)

// DeviceHealth is role neutral. Collection and protocol details belong outside
// this capability; unavailable fields have no numeric value.
type DeviceHealth struct {
	FastTrack   *uint64    `json:"fasttrack_rules,omitempty"`
	Started     string     `json:"started_at"`
	At          string     `json:"at"`
	CPU         *uint64    `json:"cpu,omitempty"`
	FreeMemory  *uint64    `json:"free_memory,omitempty"`
	TotalMemory *uint64    `json:"total_memory,omitempty"`
	Uptime      PPInterval `json:"uptime"`
	Version     string     `json:"version,omitempty"`
	Temperature *float64   `json:"temperature,omitempty"`
	ClockOffset PPInterval `json:"clock_offset"`
	NTPEnabled  *bool      `json:"ntp_enabled,omitempty"`
	NTPStatus   string     `json:"ntp_status,omitempty"`
	ErrorCount  *uint64    `json:"error_count,omitempty"`
}
type DeviceMonitoringPoint struct {
	At         string        `json:"at"`
	Resolution int           `json:"resolution"`
	SessionAt  string        `json:"session_at,omitempty"`
	OnlineMin  *int          `json:"online_min,omitempty"`
	OnlineMax  *int          `json:"online_max,omitempty"`
	Online     *int          `json:"online,omitempty"`
	Health     *DeviceHealth `json:"health,omitempty"`
	HealthAt   string        `json:"health_at,omitempty"`
	Gap        bool          `json:"gap"`
	Drop       bool          `json:"drop"`
	Samples    int           `json:"samples"`
}
type DeviceMonitoringStore interface {
	SaveDeviceMonitoring(context.Context, string, Device, uint64, *PPReadResult) Code
	DeviceMonitoring(context.Context, string, PPInput) (PPView, Code)
}

// PPMonitoringContinuous rejects reboot/clock/context ambiguity before it can
// become a session-drop or a flap. Failed reads are separately fenced by store.
func PPMonitoringContinuous(a, b *PPObservation) bool {
	if !PPCompatible(a, b) || a.Provenance.Revision != b.Provenance.Revision || a.Provenance.Epoch == "" || a.Provenance.Epoch != b.Provenance.Epoch {
		return false
	}
	elapsed := ppJointElapsed(a.Provenance.Sample, b.Provenance.Sample, ppWallElapsed(a.Provenance, b.Provenance))
	return intervalAdvance(a.Provenance.RouterUptime, b.Provenance.RouterUptime, elapsed)
}
func PPMonitoringDrop(previous, current int) bool {
	return previous > 0 && previous-current >= 5 && current*100 <= previous*80
}
func MonitoringRetention(at time.Time) (minuteCutoff, hourCutoff, transitionCutoff int64) {
	return at.Add(-24 * time.Hour).Unix(), at.Add(-7 * 24 * time.Hour).Unix(), at.Add(-24 * time.Hour).Unix()
}
