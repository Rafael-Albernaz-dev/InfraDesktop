// Package pppoe owns PPPoE session collection, comparison, live reads, and diagnostics.
package pppoe

import (
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/monitoring"
	"time"
)

type (
	Authorizer            = equipment.Authorizer
	Code                  = equipment.Code
	Command               = equipment.Command
	Credential            = equipment.Credential
	Device                = equipment.Device
	DeviceHealth          = equipment.DeviceHealth
	DeviceMonitoringPoint = equipment.DeviceMonitoringPoint
	DeviceMonitoringStore = equipment.DeviceMonitoringStore
	DevicePort            = equipment.DevicePort
	IDs                   = equipment.IDs
	Job                   = equipment.Job
	PDDiagnostic          = equipment.PDDiagnostic
	PDDiagnosticRemote    = equipment.PDDiagnosticRemote
	PDDiagnosticRequest   = equipment.PDDiagnosticRequest
	PDDiagnosticStore     = equipment.PDDiagnosticStore
	PDPingProbe           = equipment.PDPingProbe
	PDPingResult          = equipment.PDPingResult
	PDRow                 = equipment.PDRow
	PPAttempt             = equipment.PPAttempt
	PPContext             = equipment.PPContext
	PPCrossDuplicate      = equipment.PPCrossDuplicate
	PPDelta               = equipment.PPDelta
	PPDeltaSession        = equipment.PPDeltaSession
	PPInput               = equipment.PPInput
	PPInterval            = equipment.PPInterval
	PPObservation         = equipment.PPObservation
	PPPeerSession         = equipment.PPPeerSession
	PPPingTarget          = equipment.PPPingTarget
	PPProvenance          = equipment.PPProvenance
	PPQuality             = equipment.PPQuality
	PPQuery               = equipment.PPQuery
	PPRate                = equipment.PPRate
	PPReadCost            = equipment.PPReadCost
	PPReadRemote          = equipment.PPReadRemote
	PPReadRequest         = equipment.PPReadRequest
	PPReadResult          = equipment.PPReadResult
	PPReadRow             = equipment.PPReadRow
	PPReadTarget          = equipment.PPReadTarget
	PPReference           = equipment.PPReference
	PPRemote              = equipment.PPRemote
	PPRow                 = equipment.PPRow
	PPSecretEvidence      = equipment.PPSecretEvidence
	PPStore               = equipment.PPStore
	PPTarget              = equipment.PPTarget
	PPTraffic             = equipment.PPTraffic
	PPTrafficPoint        = equipment.PPTrafficPoint
	PPTunnel              = equipment.PPTunnel
	PPView                = equipment.PPView
	Remote                = equipment.Remote
	Role                  = equipment.Role
	RunningJob            = inventory.RunningJob
	SealedCredential      = equipment.SealedCredential
	Store                 = equipment.Store
	TrafficLoadSafety     = equipment.TrafficLoadSafety
	Vault                 = equipment.Vault
)

const (
	OK                      = equipment.OK
	Invalid                 = equipment.Invalid
	Changed                 = equipment.Changed
	Unavailable             = equipment.Unavailable
	Revision                = equipment.Revision
	VaultUnavailable        = equipment.VaultUnavailable
	MissingCredential       = equipment.MissingCredential
	Inactive                = equipment.Inactive
	TrustRequired           = equipment.TrustRequired
	TrustChanged            = equipment.TrustChanged
	Limit                   = equipment.Limit
	Cancelled               = equipment.Cancelled
	Partial                 = equipment.Partial
	CollectorDefect         = equipment.CollectorDefect
	CursorInvalid           = equipment.CursorInvalid
	ObservationIncompatible = equipment.ObservationIncompatible
	ObservationMissing      = equipment.ObservationMissing

	CodeJobsFull          = equipment.Code("pp_jobs_full")
	CodeJobInterrupted    = equipment.Code("JOB_INTERRUPTED")
	CodeDeadlineExceeded  = equipment.Code("DEADLINE_EXCEEDED")
	CodePersistenceFailed = equipment.Code("PERSISTENCE_FAILED")
	CodePingReadingStale  = equipment.Code("PING_READING_STALE")
	CodePingOffline       = equipment.Code("PING_OFFLINE")

	MaxDevices          = equipment.MaxDevices
	MaxConcurrent       = equipment.MaxConcurrent
	MaxJobs             = equipment.MaxJobs
	PageSize            = equipment.PageSize
	QueryBudget         = equipment.QueryBudget
	JobRetention        = equipment.JobRetention
	TrafficRetention    = equipment.TrafficRetention
	PPMaxRows           = equipment.PPMaxRows
	PPObservationBytes  = equipment.PPObservationBytes
	PPRowBytes          = equipment.PPRowBytes
	PPPersistenceBudget = equipment.PPPersistenceBudget
	PPScope             = equipment.PPScope
	PDCatalog           = equipment.PDCatalog
	PDRadius            = equipment.PDRadius
	PDPing              = equipment.PDPing
)

// Service executes authorized PPPoE collection, comparison, read, and diagnostic operations.
type Service struct {
	inv         *inventory.Service
	mon         *monitoring.Service
	pingHistory map[string][]int64
	pingOrder   []string
}

func New(inv *inventory.Service, mon *monitoring.Service) *Service {
	return &Service{inv: inv, mon: mon}
}

func PPTime(t time.Time) string    { return equipment.PPTime(t) }
func PPConnection(d Device) string { return equipment.PPConnection(d) }
func PPValidQuery(q *PPQuery) bool { return equipment.PPValidQuery(q) }
func ValidCollectorReason(c Code, reason string) bool {
	return equipment.ValidCollectorReason(c, reason)
}
