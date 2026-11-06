package inventory

import (
	"context"
	"strings"
	"sync"
	"time"
)

type RunningJob struct {
	Radio              *RadioView
	RadioInput         RadioInput
	RadioDevices       []Device
	RetainTraffic      bool
	Read               *PPReadResult
	ReadTarget         *PPReadTarget
	PPInput            PPInput
	Ping               *PPPingTarget
	View               Job
	Command            Command
	Device             Device
	CredentialRevision uint64
	Finished           time.Time
	Released           bool
	Cancel             context.CancelFunc
}

type Service struct {
	Parser   ImportParser
	Digester ImportDigester
	Auth     Authorizer
	Store    Store
	Vault    Vault
	Remote   Remote
	IDs      IDs
	Now      func() time.Time
	Mu       sync.Mutex
	Jobs     map[string]*RunningJob
	Closed   bool
	WG       sync.WaitGroup
	Epoch    string
	Origin   time.Time
}

func New(auth Authorizer, store Store, vault Vault, remote Remote, ids IDs, now func() time.Time) *Service {
	epoch, c := ids.ID()
	if c != OK {
		epoch = ""
	}
	return &Service{
		Auth:   auth,
		Store:  store,
		Vault:  vault,
		Remote: remote,
		IDs:    ids,
		Now:    now,
		Jobs:   map[string]*RunningJob{},
		Epoch:  epoch,
		Origin: time.Now(),
	}
}

func matchingScopeProfile(q Command) bool {
	return q.Scope.Profile != "" &&
		q.Scope.ExpectedProfile == q.Scope.Profile &&
		q.Scope.ExpectedActivation == q.Scope.Activation
}

func (s *Service) Authorized(ctx context.Context, q Command, fn func(uint64) Code) Code {
	if !matchingScopeProfile(q) {
		return Changed
	}
	return s.Auth.WithAuthorization(ctx, q.UID, q.Scope, func(version uint64) Code {
		if q.Authority != 0 && q.Authority != version {
			return Changed
		}
		return fn(version)
	})
}

func (s *Service) Handle(ctx context.Context, q Command) (View, Code) {
	v := View{Devices: []Device{}, Credentials: []Credential{}, VaultAvailable: s.Vault.Available()}
	c := s.Authorized(ctx, q, func(version uint64) Code {
		q.Authority = version
		return s.dispatchAuthorized(ctx, q, &v)
	})
	return v, c
}

func (s *Service) dispatchAuthorized(ctx context.Context, q Command, v *View) Code {
	switch q.Op {
	case "device.query-start", "trust.inspect":
		var c Code
		v.Job, c = s.start(ctx, q)
		return c
	case "device.query-status", "device.query-cancel":
		var c Code
		v.Job, c = s.status(ctx, q)
		return c
	case "trust.confirm":
		return s.confirm(ctx, q, v)
	case "inventory.import-preview", "inventory.import-confirm":
		return s.handleImport(ctx, q, v)
	case "inventory.bulk-update":
		return s.bulkUpdate(ctx, q, v)
	case "inventory.list", "inventory.get", "credential.list", "inventory.enroll-status":
		return s.handleReadOp(ctx, q, v)
	default:
		return s.mutate(ctx, q, v)
	}
}

func (s *Service) handleReadOp(ctx context.Context, q Command, v *View) Code {
	st, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return c
	}
	v.CredentialDefaults = st.CredentialDefaults
	v.VaultAvailable = s.VaultReady(&st)
	v.Total = len(st.Devices)
	switch q.Op {
	case "inventory.enroll-status":
		return s.readEnrollStatus(&st, q, v)
	case "inventory.get":
		d := DeviceByID(&st, q.ID)
		if d == nil {
			return Invalid
		}
		v.Device = d
		return OK
	case "credential.list":
		return s.listCredentials(&st, q, v)
	default:
		return s.listDevices(&st, q, v)
	}
}

func (s *Service) readEnrollStatus(st *State, q Command, v *View) Code {
	if !ValidID(q.Operation) || !ValidID(q.ID) {
		return Invalid
	}
	v.OperationState = "NOT_FOUND"
	for _, op := range st.Operations {
		if op.ID != q.Operation {
			continue
		}
		if !validEnrollOperation(op, q.ID) {
			return Revision
		}
		v.OperationState = "COMMITTED"
		v.Device = DeviceByID(st, q.ID)
		if v.Device == nil {
			return Unavailable
		}
		break
	}
	return OK
}

func validEnrollOperation(op Operation, id string) bool {
	if op.Entity != id {
		return false
	}
	return strings.HasPrefix(op.Stamp, `["inventory.enroll" `) ||
		strings.HasPrefix(op.Stamp, `["inventory.create" `)
}

func (s *Service) listCredentials(st *State, q Command, v *View) Code {
	if q.Cursor < 0 || len(q.Search) > 160 || q.Cursor > len(st.Credentials) {
		return Invalid
	}
	end := min(q.Cursor+PageSize, len(st.Credentials))
	for _, c := range st.Credentials[q.Cursor:end] {
		m := c.Metadata
		m.Links = CredentialLinks(st, m.ID)
		v.Credentials = append(v.Credentials, m)
	}
	if end < len(st.Credentials) {
		v.Next = end
	}
	return OK
}

func (s *Service) listDevices(st *State, q Command, v *View) Code {
	if q.Cursor < 0 || len(q.Search) > 160 {
		return Invalid
	}
	if c := validateInventoryFilters(q); c != OK {
		return c
	}
	v.Blocks, v.Models = inventoryOptions(st.Devices)
	filtered := filterDevices(st.Devices, q)
	if q.Cursor > len(filtered) {
		return Invalid
	}
	end := min(q.Cursor+PageSize, len(filtered))
	v.Devices = filtered[q.Cursor:end]
	if end < len(filtered) {
		v.Next = end
	}
	return OK
}

func (s *Service) PruneLocked() {
	for id, j := range s.Jobs {
		retention := JobRetention
		if j.PPInput.Scope == "traffic" {
			retention = TrafficRetention
		}
		if j.Released && !j.Finished.IsZero() && s.Now().Sub(j.Finished) >= retention {
			delete(s.Jobs, id)
		}
	}
}

func (s *Service) Close() {
	s.Mu.Lock()
	s.Closed = true
	for _, j := range s.Jobs {
		j.Cancel()
	}
	s.Mu.Unlock()
	s.WG.Wait()
}

// Existing ciphertext must never be paired with a freshly generated company key.
func (s *Service) VaultReady(st *State) bool {
	if len(st.Credentials) > 0 && len(st.WrappedKey) == 0 {
		return false
	}
	return s.Vault.Ready(st.Installation, st.Company, st.WrappedKey)
}

func (s *Service) VaultAvailable() bool { return s.Vault.Available() }
