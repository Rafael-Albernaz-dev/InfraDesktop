// Package equipment owns inventory, reusable read credentials and finite queries.
package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"net/netip"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type Code = access.Code

const (
	OK                           = access.OK
	Invalid                      = access.Invalid
	Changed                      = access.Changed
	Unavailable                  = access.Unavailable
	InvalidDeviceName       Code = "DEVICE_NAME_INVALID"
	InvalidDeviceAddress    Code = "DEVICE_ADDRESS_INVALID"
	InvalidDevicePort       Code = "DEVICE_PORT_INVALID"
	InvalidCredentialAlias  Code = "CREDENTIAL_ALIAS_INVALID"
	InvalidCredentialUser   Code = "CREDENTIAL_USER_INVALID"
	InvalidCredentialSecret Code = "CREDENTIAL_SECRET_INVALID"
	Revision                Code = "REVISION_CONFLICT"
	VaultUnavailable        Code = "VAULT_UNAVAILABLE"
	MissingCredential       Code = "CREDENTIAL_NOT_FOUND"
	Inactive                Code = "DEVICE_INACTIVE"
	TrustRequired           Code = "TRUST_REQUIRED"
	TrustChanged            Code = "TRUST_CHANGED"
	Limit                   Code = "LIMIT_EXCEEDED"
	Cancelled               Code = "CANCELLED"
	Partial                 Code = "PARTIAL"
	MaxDevices                   = 256
	MaxCredentials               = 128
	PageSize                     = 16
	MaxJobs                      = 32
	MaxConcurrent                = 2
	MaxOperations                = 2048
	QueryBudget                  = 20 * time.Second
	JobRetention                 = 2 * time.Minute
)

// Safe projections have no secret, key or ciphertext fields.
type Credential struct {
	ID       string `json:"id"`
	Alias    string `json:"alias"`
	User     string `json:"user"`
	Revision uint64 `json:"revision"`
	Links    int    `json:"links"`
}
type InterfaceReading struct {
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Running  bool   `json:"running"`
	Disabled bool   `json:"disabled"`
}

// InterfacePart describes a bounded static-interface snapshot separately from summary.
type InterfacePart struct {
	CountsKnown bool   `json:"counts_known"`
	Status      string `json:"status"` // complete, limited, failed, stale
	Code        Code   `json:"code"`
	At          string `json:"observed_at"`
	Source      string `json:"source"`
	Total       int    `json:"total"`
	Eligible    int    `json:"eligible"`
	Dynamic     int    `json:"dynamic"`
}
type TrustAcceptance struct {
	Mode        string `json:"mode"`
	At          string `json:"accepted_at"`
	UID         uint32 `json:"uid"`
	Company     string `json:"company_id"`
	DeviceID    string `json:"device_id"`
	Address     string `json:"address"`
	Port        int    `json:"port"`
	Revision    uint64 `json:"revision"`
	Fingerprint string `json:"fingerprint"`
}
type Attempt struct {
	JobID string `json:"job_id"`
	State string `json:"state"`
	Code  Code   `json:"code"`
	At    string `json:"at"`
}
type Observation struct {
	InterfacePart *InterfacePart     `json:"interface_part,omitempty"`
	Identity      string             `json:"identity"`
	Version       string             `json:"version"`
	Family        string             `json:"family"`
	Board         string             `json:"board"`
	Uptime        string             `json:"uptime"`
	CPU           int                `json:"cpu_percent"`
	FreeMemory    uint64             `json:"free_memory_bytes"`
	TotalMemory   uint64             `json:"total_memory_bytes"`
	Interfaces    []InterfaceReading `json:"interfaces"`
	At            string             `json:"observed_at"`
	Source        string             `json:"source"`
	Quality       string             `json:"quality"`
}
type Device struct {
	Role            string           `json:"role"`
	PPPoE           *PPSummary       `json:"pppoe,omitempty"`
	TrustAcceptance *TrustAcceptance `json:"trust_acceptance,omitempty"`
	LastAttempt     *Attempt         `json:"last_attempt,omitempty"`
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Address         string           `json:"address"`
	Port            int              `json:"port"`
	Transport       string           `json:"transport"`
	CredentialID    string           `json:"credential_id"`
	Revision        uint64           `json:"revision"`
	Active          bool             `json:"active"`
	Fingerprint     string           `json:"fingerprint"`
	TrustOrigin     string           `json:"trust_origin"`
	Result          *Observation     `json:"result,omitempty"`
}
type PPSummary struct {
	Online  int    `json:"online"`
	Offline int    `json:"offline"`
	At      string `json:"at"`
}
type SealedCredential struct {
	Metadata Credential
	Envelope []byte
}
type Operation struct{ ID, Stamp, Entity string }
type State struct {
	Installation, Company string
	WrappedKey            []byte
	Credentials           []SealedCredential
	Devices               []Device
	Operations            []Operation
}
type Store interface {
	ReadEquipment(context.Context, string) (State, Code)
	UpdateEquipment(context.Context, string, func(*State) Code) Code
}
type Authorizer interface {
	WithAuthorization(context.Context, uint32, access.Command, func(uint64) Code) Code
}
type Vault interface {
	Available() bool
	Ready(string, string, []byte) bool
	Seal(installation, company, id string, revision uint64, secret string, wrapped []byte) ([]byte, []byte, Code)
	Open(installation, company, id string, revision uint64, envelope, wrapped []byte) (string, Code)
	Stamp(string) (string, Code)
}
type Remote interface {
	Inspect(context.Context, Device) (string, Code)
	Query(context.Context, Device, string, string, func() Code) (*Observation, Code)
}
type IDs interface{ ID() (string, Code) }
type Command struct {
	Authority                                                                        uint64
	Op                                                                               string
	Scope                                                                            access.Command
	UID                                                                              uint32
	Operation, ID, Name, Address, Transport, CredentialID, Alias, User, Secret, Role string
	Revision                                                                         uint64
	Port                                                                             int
	Active                                                                           bool
	Inactive                                                                         bool
	Cursor                                                                           int
	Search                                                                           string
	Affected                                                                         int
	JobID, Fingerprint                                                               string
}
type Job struct {
	PPPoE       *PPAttempt   `json:"pppoe,omitempty"`
	ID          string       `json:"id"`
	DeviceID    string       `json:"device_id"`
	Kind        string       `json:"kind"`
	State       string       `json:"state"`
	Code        Code         `json:"code"`
	Fingerprint string       `json:"fingerprint,omitempty"`
	Result      *Observation `json:"result,omitempty"`
}
type View struct {
	Total          int          `json:"total_devices"`
	OperationState string       `json:"operation_state,omitempty"`
	Devices        []Device     `json:"devices"`
	Credentials    []Credential `json:"credentials"`
	Device         *Device      `json:"device,omitempty"`
	Credential     *Credential  `json:"credential,omitempty"`
	Job            *Job         `json:"job,omitempty"`
	Next           int          `json:"next_cursor"`
	VaultAvailable bool         `json:"vault_available"`
}

func ValidText(s string, max int) bool {
	if strings.TrimSpace(s) != s || !utf8.ValidString(s) || utf8.RuneCountInString(s) < 1 || utf8.RuneCountInString(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// ValidateDevice reports safe field codes; values and secrets never enter errors.
func ValidateDevice(d Device) Code {
	if !ValidText(d.Name, 80) {
		return InvalidDeviceName
	}
	ip, e := netip.ParseAddr(d.Address)
	if e != nil || ip.Zone() != "" || ip.String() != d.Address || ip.IsUnspecified() || ip.IsMulticast() {
		return InvalidDeviceAddress
	}
	if d.Port < 1 || d.Port > 65535 {
		return InvalidDevicePort
	}
	if !access.ValidCompanyID(d.ID) || d.Transport != "ssh" || (d.CredentialID != "" && !access.ValidCompanyID(d.CredentialID)) || d.Revision == 0 {
		return Invalid
	}
	return OK
}
func ValidDevice(d Device) bool { return ValidateDevice(d) == OK }
func validateCredential(alias, user, secret string) Code {
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
func credential(st *State, id string) *SealedCredential {
	for i := range st.Credentials {
		if st.Credentials[i].Metadata.ID == id {
			return &st.Credentials[i]
		}
	}
	return nil
}
func device(st *State, id string) *Device {
	for i := range st.Devices {
		if st.Devices[i].ID == id {
			return &st.Devices[i]
		}
	}
	return nil
}
func links(st *State, id string) int {
	n := 0
	for _, d := range st.Devices {
		if d.CredentialID == id {
			n++
		}
	}
	return n
}
