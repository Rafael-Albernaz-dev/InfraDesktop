// Package radio owns wireless AP/CPE/PTP readings, link association, cost measurement, and live sessions.
package radio

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"time"
)

type (
	Attempt            = equipment.Attempt
	Authorizer         = equipment.Authorizer
	Code               = equipment.Code
	Command            = equipment.Command
	Credential         = equipment.Credential
	Device             = equipment.Device
	IDs                = equipment.IDs
	Job                = equipment.Job
	Operation          = equipment.Operation
	PPAttempt          = equipment.PPAttempt
	PPInput            = equipment.PPInput
	PPInterval         = equipment.PPInterval
	PPObservation      = equipment.PPObservation
	PPProvenance       = equipment.PPProvenance
	PPQuality          = equipment.PPQuality
	PPRow              = equipment.PPRow
	PPStore            = equipment.PPStore
	PPTarget           = equipment.PPTarget
	PPView             = equipment.PPView
	RadioClient        = equipment.RadioClient
	RadioCost          = equipment.RadioCost
	RadioFailure       = equipment.RadioFailure
	RadioFamily        = equipment.RadioFamily
	RadioIndex         = equipment.RadioIndex
	RadioIndexEntry    = equipment.RadioIndexEntry
	RadioIndexRemote   = equipment.RadioIndexRemote
	RadioInput         = equipment.RadioInput
	RadioInterface     = equipment.RadioInterface
	RadioLink          = equipment.RadioLink
	RadioLinkChoice    = equipment.RadioLinkChoice
	RadioLoad          = equipment.RadioLoad
	RadioMeasurement   = equipment.RadioMeasurement
	RadioMetric        = equipment.RadioMetric
	RadioMode          = equipment.RadioMode
	RadioProfile       = equipment.RadioProfile
	RadioProposal      = equipment.RadioProposal
	RadioReadSession   = equipment.RadioReadSession
	RadioReading       = equipment.RadioReading
	RadioRemote        = equipment.RadioRemote
	RadioSessionRemote = equipment.RadioSessionRemote
	RadioStore         = equipment.RadioStore
	RadioTickReceipt   = equipment.RadioTickReceipt
	RadioTiming        = equipment.RadioTiming
	RadioView          = equipment.RadioView
	Remote             = equipment.Remote
	Role               = equipment.Role
	RunningJob         = inventory.RunningJob
	SealedCredential   = equipment.SealedCredential
	State              = equipment.State
	Store              = equipment.Store
	Transport          = equipment.Transport
	TrustAcceptance    = equipment.TrustAcceptance
	Vault              = equipment.Vault
	View               = equipment.View
	runningJob         = inventory.RunningJob
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

	RoleAP                  = equipment.RoleAP
	RoleCPE                 = equipment.RoleCPE
	RolePTP                 = equipment.RolePTP
	RoleConcentrator        = equipment.RoleConcentrator
	RadioAuto               = equipment.RadioAuto
	RadioStandalone         = equipment.RadioStandalone
	RadioUnsupported        = equipment.RadioUnsupported
	RadioRouterOSWirelessV6 = equipment.RadioRouterOSWirelessV6
	RadioAirOSV6            = equipment.RadioAirOSV6
	RadioIntelbrasWOM       = equipment.RadioIntelbrasWOM
	RadioOnce               = equipment.RadioOnce
	RadioMeasure            = equipment.RadioMeasure
	RadioLive               = equipment.RadioLive
	RadioModeIndex          = equipment.RadioModeIndex
	TransportSSH            = equipment.TransportSSH

	RadioCodeRemotePermissionDenied     = equipment.RadioCodeRemotePermissionDenied
	RadioCodeConnectFailed              = equipment.RadioCodeConnectFailed
	RadioCodeDeadlineExceeded           = equipment.RadioCodeDeadlineExceeded
	RadioCodeCPEUnreachable             = equipment.RadioCodeCPEUnreachable
	RadioCodeFamilyConfirmationRequired = equipment.RadioCodeFamilyConfirmationRequired
	RadioCodeLinkRequired               = equipment.RadioCodeLinkRequired
	RadioCodeLinkUnconfirmed            = equipment.RadioCodeLinkUnconfirmed
	RadioCodeLiveAborted                = equipment.RadioCodeLiveAborted
	RadioCodeLiveCostRequired           = equipment.RadioCodeLiveCostRequired
	RadioCodeLoadUnavailable            = equipment.RadioCodeLoadUnavailable
	RadioCodeSessionOffline             = equipment.RadioCodeSessionOffline
	RadioCodeWindowEnded                = equipment.RadioCodeWindowEnded
	RadioCodeRemoteAuthFailed           = equipment.RadioCodeRemoteAuthFailed
	RadioCodeSSHConnectionLost          = equipment.RadioCodeSSHConnectionLost
	RadioCodeSSHNegotiationFailed       = equipment.RadioCodeSSHNegotiationFailed
	RadioCodeUnsupported                = equipment.RadioCodeUnsupported

	MaxDevices      = equipment.MaxDevices
	MaxConcurrent   = equipment.MaxConcurrent
	MaxJobs         = equipment.MaxJobs
	MaxOperations   = equipment.MaxOperations
	PageSize        = equipment.PageSize
	QueryBudget     = equipment.QueryBudget
	JobRetention    = equipment.JobRetention
	RadioMaxClients = equipment.RadioMaxClients
	RadioMaxCPU     = equipment.RadioMaxCPU
	RadioPageSize   = equipment.RadioPageSize
	RadioIndexTTL   = equipment.RadioIndexTTL
	PPScope         = equipment.PPScope
)

// Service executes authorized radio inspection, link association, cost measurement, and live sessions.
type Service struct {
	*inventory.Service
	radioIndexes map[string]RadioIndex
	radioCosts   map[string]RadioCost
}

func New(inv *inventory.Service) *Service {
	return &Service{
		Service:      inv,
		radioIndexes: map[string]RadioIndex{},
		radioCosts:   map[string]RadioCost{},
	}
}

func NewWithInventory(
	auth Authorizer, store Store, vault Vault, remote Remote, ids IDs, now func() time.Time,
) *Service {
	return New(inventory.New(auth, store, vault, remote, ids, now))
}

func (s *Service) authorized(ctx context.Context, q Command, fn func(uint64) Code) Code {
	return s.Authorized(ctx, q, fn)
}

func (s *Service) statusKind(ctx context.Context, q Command, kind string) (*Job, Code) {
	return s.StatusKind(ctx, q, kind)
}

func (s *Service) check(ctx context.Context, j *RunningJob, reveal bool) (string, string, Code) {
	return s.Check(ctx, j, reveal)
}

func (s *Service) watchPP(ctx context.Context, j *RunningJob) func() {
	return s.Watch(ctx, j)
}

func (s *Service) prune() {
	s.PruneLocked()
}

func ValidateDevice(d Device) Code { return inventory.ValidateDevice(d) }
func ValidText(s string, max int) bool {
	return inventory.ValidText(s, max)
}
func device(st *State, id string) *Device { return inventory.DeviceByID(st, id) }
func credential(st *State, id string) *SealedCredential {
	return inventory.CredentialByID(st, id)
}
func sameScope(a, b Command) bool  { return inventory.SameScope(a, b) }
func copyJob(j *RunningJob) *Job   { return inventory.CopyJob(j) }
func Endpoint(d Device) string     { return equipment.Endpoint(d) }
func PPConnection(d Device) string { return equipment.PPConnection(d) }
func PPTime(t time.Time) string    { return equipment.PPTime(t) }
func ValidCollectorReason(c Code, reason string) bool {
	return equipment.ValidCollectorReason(c, reason)
}
