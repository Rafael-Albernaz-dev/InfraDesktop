package inventory

import (
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"time"
)

type (
	Attempt          = equipment.Attempt
	Authorizer       = equipment.Authorizer
	Code             = equipment.Code
	Command          = equipment.Command
	Credential       = equipment.Credential
	Device           = equipment.Device
	IDs              = equipment.IDs
	InterfacePart    = equipment.InterfacePart
	InterfaceReading = equipment.InterfaceReading
	Job              = equipment.Job
	Observation      = equipment.Observation
	Operation        = equipment.Operation
	PPAttempt        = equipment.PPAttempt
	PPInput          = equipment.PPInput
	PPPingTarget     = equipment.PPPingTarget
	PPReadResult     = equipment.PPReadResult
	PPReadTarget     = equipment.PPReadTarget
	PPStore          = equipment.PPStore
	PPTarget         = equipment.PPTarget
	RadioFailure     = equipment.RadioFailure
	RadioInput       = equipment.RadioInput
	RadioView        = equipment.RadioView
	Remote           = equipment.Remote
	Role             = equipment.Role
	SealedCredential = equipment.SealedCredential
	State            = equipment.State
	Store            = equipment.Store
	Transport        = equipment.Transport
	TrustAcceptance  = equipment.TrustAcceptance
	Vault            = equipment.Vault
	View             = equipment.View
)

const (
	OK                      = equipment.OK
	Invalid                 = equipment.Invalid
	Changed                 = equipment.Changed
	Unavailable             = equipment.Unavailable
	InvalidDeviceName       = equipment.InvalidDeviceName
	InvalidDeviceAddress    = equipment.InvalidDeviceAddress
	InvalidDevicePort       = equipment.InvalidDevicePort
	InvalidCredentialAlias  = equipment.InvalidCredentialAlias
	InvalidCredentialUser   = equipment.InvalidCredentialUser
	InvalidCredentialSecret = equipment.InvalidCredentialSecret
	Revision                = equipment.Revision
	VaultUnavailable        = equipment.VaultUnavailable
	MissingCredential       = equipment.MissingCredential
	Inactive                = equipment.Inactive
	TrustRequired           = equipment.TrustRequired
	TrustChanged            = equipment.TrustChanged
	Limit                   = equipment.Limit
	Cancelled               = equipment.Cancelled
	Partial                 = equipment.Partial
	MaxDevices              = equipment.MaxDevices
	MaxCredentials          = equipment.MaxCredentials
	PageSize                = equipment.PageSize
	MaxJobs                 = equipment.MaxJobs
	MaxConcurrent           = equipment.MaxConcurrent
	MaxOperations           = equipment.MaxOperations
	QueryBudget             = equipment.QueryBudget
	JobRetention            = equipment.JobRetention
	TrafficRetention        = equipment.TrafficRetention
	RoleAP                  = equipment.RoleAP
	RoleCPE                 = equipment.RoleCPE
	RolePTP                 = equipment.RolePTP
	RoleConcentrator        = equipment.RoleConcentrator
	TransportSSH            = equipment.TransportSSH
	CodeDeadlineExceeded    = equipment.RadioCodeDeadlineExceeded
	CodeUnsupported         = equipment.RadioCodeUnsupported
	CodePersistenceFailed   = Code("PERSISTENCE_FAILED")
)

func ValidText(s string, max int) bool { return equipment.ValidText(s, max) }
func ValidateDevice(d Device) Code     { return equipment.ValidateDevice(d) }
func ValidDevice(d Device) bool        { return equipment.ValidDevice(d) }
func PPTime(t time.Time) string        { return equipment.PPTime(t) }
func PPConnection(d Device) string     { return equipment.PPConnection(d) }
func Endpoint(d Device) string         { return equipment.Endpoint(d) }
func CopyJob(j *RunningJob) *Job       { v := j.View; return &v }
func ValidID(id string) bool           { return access.ValidCompanyID(id) }

func ValidateCredential(alias, user, secret string) Code {
	if !ValidText(alias, 80) {
		return InvalidCredentialAlias
	}
	if !ValidText(user, 80) {
		return InvalidCredentialUser
	}
	if len(secret) < 1 || len(secret) > 1024 {
		return InvalidCredentialSecret
	}
	return OK
}

func CredentialByID(st *State, id string) *SealedCredential {
	for i := range st.Credentials {
		if st.Credentials[i].Metadata.ID == id {
			return &st.Credentials[i]
		}
	}
	return nil
}

func DeviceByID(st *State, id string) *Device {
	for i := range st.Devices {
		if st.Devices[i].ID == id {
			return &st.Devices[i]
		}
	}
	return nil
}

func CredentialLinks(st *State, id string) int {
	n := 0
	for _, d := range st.Devices {
		if d.CredentialID == id {
			n++
		}
	}
	return n
}

func SameScope(a, b Command) bool {
	return a.Authority == b.Authority &&
		a.UID == b.UID &&
		a.Scope.Owner == b.Scope.Owner &&
		a.Scope.Session == b.Scope.Session &&
		a.Scope.Profile == b.Scope.Profile &&
		a.Scope.Activation == b.Scope.Activation
}

func JobUsesEndpoint(j *RunningJob, d Device) bool {
	if Endpoint(j.Device) == Endpoint(d) {
		return true
	}
	for _, target := range j.RadioDevices {
		if Endpoint(target) == Endpoint(d) {
			return true
		}
	}
	return false
}
