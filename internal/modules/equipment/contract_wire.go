package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"time"
)

// Handler is the composite capability port satisfied by inventory, pppoe, radio, and monitoring.
type Handler interface {
	Handle(context.Context, Command) (View, Code)
	HandlePP(context.Context, Command, PPInput) (PPView, Code)
	HandleRadio(context.Context, Command, RadioInput) (RadioView, Code)
	VaultAvailable() bool
	Close()
}

// Service delegates to the capability services mounted in composition.
type Service struct {
	Handler
}

var DefaultFactory func(Authorizer, Store, Vault, Remote, IDs, func() time.Time) Handler

func New(auth Authorizer, store Store, vault Vault, remote Remote, ids IDs, now func() time.Time) *Service {
	if DefaultFactory == nil {
		return &Service{}
	}
	return &Service{Handler: DefaultFactory(auth, store, vault, remote, ids, now)}
}

var (
	ComparePP                      func(*PPObservation, *PPObservation) ([]PPDelta, Code)
	ReconcilePP                    func(*PPObservation, *PPObservation) Code
	ReconcileCatalogAbsences       func(*PDDiagnostic, *PDDiagnostic) Code
	ComparableConcentratorReadings func(*PPObservation, *PPObservation, time.Time) bool
	AssociatePPTunnels             func([]PPRow, []PPTunnel, PPInterval) []int
	MatchPPRead                    func(*PPReadTarget, []PPTunnel, PPInterval) []PPTunnel
	PPBandwidth                    func(PPTunnel, PPTunnel) (*PPRate, *PPRate)
	SortPPConsumption              func([]PPReadRow)
	PPReadScope                    func(string) bool
	PPReadBudget                   func(string) time.Duration
	PPReadWithinBudget             func(*PPReadResult) bool
	PPMonitoringContinuous         func(*PPObservation, *PPObservation) bool
	PPMonitoringDrop               func(int, int) bool
	MonitoringRetention            func(time.Time) (int64, int64, int64)
	DetectedDeviceReboot           func(*DeviceHealth, *DeviceHealth) bool
	PortNegotiationChanged         func(*DevicePort, *DevicePort) bool
	StampRadio                     func(*RadioReading, Device, uint64, time.Time)
)

// ValidateDevice reports safe field codes; values and secrets never enter errors.
func ValidateDevice(d Device) Code {
	if !ValidBlock(d.Block) {
		return Invalid
	}
	if !ValidText(d.Name, 80) {
		return InvalidDeviceName
	}
	if code := validateDeviceEndpoint(d); code != OK {
		return code
	}
	if !access.ValidCompanyID(d.ID) || d.Transport != TransportSSH || d.Revision == 0 {
		return Invalid
	}
	if d.CredentialID != "" && !access.ValidCompanyID(d.CredentialID) {
		return Invalid
	}
	return OK
}

func ValidDevice(d Device) bool { return ValidateDevice(d) == OK }
