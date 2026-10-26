package equipment

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/access"
	"net/netip"
	"strings"
	"sync"
	"time"
)

type runningJob struct {
	ppInput            PPInput
	ping               *PPPingTarget
	view               Job
	command            Command
	device             Device
	credentialRevision uint64
	finished           time.Time
	released           bool
	cancel             context.CancelFunc
}
type Service struct {
	auth   Authorizer
	store  Store
	vault  Vault
	remote Remote
	ids    IDs
	now    func() time.Time
	mu     sync.Mutex
	jobs   map[string]*runningJob
	closed bool
	wg     sync.WaitGroup
	epoch  string
	origin time.Time
}

func New(auth Authorizer, store Store, vault Vault, remote Remote, ids IDs, now func() time.Time) *Service {
	epoch, c := ids.ID()
	if c != OK {
		epoch = ""
	}
	return &Service{auth: auth, store: store, vault: vault, remote: remote, ids: ids, now: now, jobs: map[string]*runningJob{}, epoch: epoch, origin: time.Now()}
}
func (s *Service) authorized(ctx context.Context, q Command, fn func(uint64) Code) Code {
	if q.Scope.Profile == "" || q.Scope.ExpectedProfile != q.Scope.Profile || q.Scope.ExpectedActivation != q.Scope.Activation {
		return Changed
	}
	return s.auth.WithAuthorization(ctx, q.UID, q.Scope, func(version uint64) Code {
		if q.Authority != 0 && q.Authority != version {
			return Changed
		}
		return fn(version)
	})
}
func (s *Service) Handle(ctx context.Context, q Command) (View, Code) {
	v := View{Devices: []Device{}, Credentials: []Credential{}, VaultAvailable: s.vault.Available()}
	c := s.authorized(ctx, q, func(version uint64) Code {
		q.Authority = version
		if q.Op == "device.query-start" || q.Op == "trust.inspect" {
			var c Code
			v.Job, c = s.start(ctx, q)
			return c
		}
		if q.Op == "device.query-status" || q.Op == "device.query-cancel" {
			var c Code
			v.Job, c = s.status(ctx, q)
			return c
		}
		if q.Op == "trust.confirm" {
			return s.confirm(ctx, q, &v)
		}
		if q.Op == "inventory.list" || q.Op == "inventory.get" || q.Op == "credential.list" || q.Op == "inventory.enroll-status" {
			st, c := s.store.ReadEquipment(ctx, q.Scope.Profile)
			if c != OK {
				return c
			}
			v.VaultAvailable = s.vaultReady(&st)
			v.Total = len(st.Devices)
			if q.Op == "inventory.enroll-status" {
				if !access.ValidCompanyID(q.Operation) || !access.ValidCompanyID(q.ID) {
					return Invalid
				}
				v.OperationState = "NOT_FOUND"
				for _, op := range st.Operations {
					if op.ID == q.Operation {
						if op.Entity != q.ID || !(strings.HasPrefix(op.Stamp, `["inventory.enroll" `) || strings.HasPrefix(op.Stamp, `["inventory.create" `)) {
							return Revision
						}
						v.OperationState = "COMMITTED"
						v.Device = device(&st, q.ID)
						if v.Device == nil {
							return Unavailable
						}
						break
					}
				}
				return OK
			}
			if q.Op == "inventory.get" {
				d := device(&st, q.ID)
				if d == nil {
					return Invalid
				}
				v.Device = d
				return OK
			}
			if q.Cursor < 0 || len(q.Search) > 160 {
				return Invalid
			}
			if q.Op == "credential.list" {
				if q.Cursor > len(st.Credentials) {
					return Invalid
				}
				end := q.Cursor + PageSize
				if end > len(st.Credentials) {
					end = len(st.Credentials)
				}
				for _, c := range st.Credentials[q.Cursor:end] {
					m := c.Metadata
					m.Links = links(&st, m.ID)
					v.Credentials = append(v.Credentials, m)
				}
				if end < len(st.Credentials) {
					v.Next = end
				}
				return OK
			}
			filtered := []Device{}
			for _, d := range st.Devices {
				if d.Active != q.Inactive && strings.Contains(strings.ToLower(d.Name+" "+d.Address), strings.ToLower(q.Search)) {
					if d.Result != nil {
						summary := *d.Result
						summary.Interfaces = nil
						d.Result = &summary
					}
					filtered = append(filtered, d)
				}
			}
			if q.Cursor > len(filtered) {
				return Invalid
			}
			end := q.Cursor + PageSize
			if end > len(filtered) {
				end = len(filtered)
			}
			v.Devices = filtered[q.Cursor:end]
			if end < len(filtered) {
				v.Next = end
			}
			return OK
		}
		return s.mutate(ctx, q, &v)
	})
	return v, c
}
func (s *Service) mutate(ctx context.Context, q Command, v *View) Code {
	// Canonicalize only non-secret identifiers, before the idempotency stamp.
	if q.Op == "inventory.enroll" || q.Op == "inventory.create" || q.Op == "inventory.update" {
		q.Name = strings.TrimSpace(q.Name)
		q.Address = strings.TrimSpace(q.Address)
		if ip, e := netip.ParseAddr(q.Address); e == nil {
			q.Address = ip.String()
		}
	}
	if q.Op == "inventory.enroll" || q.Op == "credential.put" {
		q.User = strings.TrimSpace(q.User)
		q.Alias = strings.TrimSpace(q.Alias)
	}
	if !access.ValidCompanyID(q.Operation) || !access.ValidCompanyID(q.ID) {
		return Invalid
	}
	secretStamp := ""
	if q.Op == "credential.put" || q.Op == "inventory.enroll" {
		var c Code
		secretStamp, c = s.vault.Stamp(q.Secret)
		if c != OK {
			return c
		}
	}
	stamp := fmt.Sprintf("%q", []string{q.Op, q.ID, q.Name, q.Address, fmt.Sprint(q.Port), q.Transport, q.CredentialID, fmt.Sprint(q.Revision), fmt.Sprint(q.Active), q.Alias, q.User, secretStamp, fmt.Sprint(q.Affected)})
	mutated := false
	c := s.store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		for _, op := range st.Operations {
			if op.ID == q.Operation {
				if op.Stamp != stamp && !(q.Op == "trust.reset" && strings.HasPrefix(op.Stamp, stamp+";previous:")) {
					return Revision
				}
				if q.Op == "credential.put" {
					r := credential(st, op.Entity)
					if r == nil {
						return Unavailable
					}
					m := r.Metadata
					m.Links = links(st, m.ID)
					v.Credential = &m
				} else {
					v.Device = device(st, op.Entity)
				}
				return OK
			}
		}
		if len(st.Operations) >= MaxOperations {
			return Limit
		}
		switch q.Op {
		case "credential.put":
			if !s.vaultReady(st) {
				return VaultUnavailable
			}
			if c := validateCredential(q.Alias, q.User, q.Secret); c != OK {
				return c
			}
			old := credential(st, q.ID)
			revision := uint64(1)
			if old != nil {
				if old.Metadata.Revision != q.Revision || links(st, q.ID) != q.Affected {
					return Revision
				}
				revision = old.Metadata.Revision + 1
			} else if q.Revision != 0 {
				return Revision
			}
			if old == nil && len(st.Credentials) >= MaxCredentials {
				return Limit
			}
			envelope, wrapped, c := s.vault.Seal(st.Installation, st.Company, q.ID, revision, q.Secret, st.WrappedKey)
			if c != OK {
				return c
			}
			st.WrappedKey = wrapped
			item := SealedCredential{Metadata: Credential{ID: q.ID, Alias: q.Alias, User: q.User, Revision: revision}, Envelope: envelope}
			if old != nil {
				*old = item
			} else {
				st.Credentials = append(st.Credentials, item)
			}
			for i := range st.Devices {
				if st.Devices[i].CredentialID == q.ID {
					st.Devices[i].Result = nil
				}
			}
			m := item.Metadata
			m.Links = links(st, q.ID)
			v.Credential = &m
		case "inventory.enroll":
			if !s.vaultReady(st) {
				return VaultUnavailable
			}
			if q.Revision != 0 || q.CredentialID != "" || device(st, q.ID) != nil || credential(st, q.ID) != nil {
				return Revision
			}
			if len(st.Devices) >= MaxDevices || len(st.Credentials) >= MaxCredentials {
				return Limit
			}
			d := Device{ID: q.ID, Name: q.Name, Address: q.Address, Port: q.Port, Transport: q.Transport, CredentialID: q.ID, Revision: 1, Active: true}
			if c := ValidateDevice(d); c != OK {
				return c
			}
			if c := validateCredential(q.Name, q.User, q.Secret); c != OK {
				return c
			}
			envelope, wrapped, c := s.vault.Seal(st.Installation, st.Company, q.ID, 1, q.Secret, st.WrappedKey)
			if c != OK {
				return c
			}
			st.WrappedKey = wrapped
			st.Credentials = append(st.Credentials, SealedCredential{Metadata: Credential{ID: q.ID, Alias: q.Name, User: q.User, Revision: 1}, Envelope: envelope})
			st.Devices = append(st.Devices, d)
			v.Device = &d
		case "trust.reset":
			d := device(st, q.ID)
			if d == nil || d.Revision != q.Revision {
				return Revision
			}
			if !d.Active {
				return Inactive
			}
			if d.Fingerprint == "" {
				return TrustRequired
			}
			// Preserve old acceptance in the reset receipt; legacy origin remains historical.
			stamp += fmt.Sprintf(";previous:%q:%q", d.Fingerprint, fmt.Sprint(d.TrustAcceptance))
			d.Fingerprint = ""
			d.TrustAcceptance = nil
			d.Revision++
			d.LastAttempt = nil
			v.Device = d
		case "inventory.create", "inventory.update", "inventory.set-active":
			old := device(st, q.ID)
			d := Device{ID: q.ID, Name: q.Name, Address: q.Address, Port: q.Port, Transport: q.Transport, CredentialID: q.CredentialID, Revision: 1, Active: true}
			if q.Op == "inventory.create" {
				if old != nil || q.Revision != 0 {
					return Revision
				}
				if len(st.Devices) >= MaxDevices {
					return Limit
				}
			} else {
				if old == nil || old.Revision != q.Revision {
					return Revision
				}
				d.Revision = old.Revision + 1
				d.Active = old.Active
				if q.Op == "inventory.set-active" {
					// Activity changes preserve the last completed reading.
					d = *old
					d.Active = q.Active
					d.Revision++
				}
				if d.Address == old.Address && d.Port == old.Port && d.Transport == old.Transport {
					d.Fingerprint = old.Fingerprint
					d.TrustOrigin = old.TrustOrigin
					d.TrustAcceptance = old.TrustAcceptance
					if d.CredentialID == old.CredentialID {
						// Metadata edits preserve readings tied to the same connection.
						d.Result = old.Result
						d.LastAttempt = old.LastAttempt
					}
				}
			}
			if c := ValidateDevice(d); c != OK {
				return c
			}
			if d.CredentialID != "" && credential(st, d.CredentialID) == nil {
				return MissingCredential
			}
			if old != nil {
				*old = d
			} else {
				st.Devices = append(st.Devices, d)
			}
			v.Device = &d
		default:
			return Invalid
		}
		st.Operations = append(st.Operations, Operation{q.Operation, stamp, q.ID})
		mutated = true
		return OK
	})
	if c == OK && mutated {
		s.invalidate(q.Scope.Profile, q.ID, q.Op == "credential.put")
	}
	return c
}
func sameScope(a, b Command) bool {
	return a.Authority == b.Authority && a.UID == b.UID && a.Scope.Owner == b.Scope.Owner && a.Scope.Session == b.Scope.Session && a.Scope.Profile == b.Scope.Profile && a.Scope.Activation == b.Scope.Activation
}
func (s *Service) prune() {
	for id, j := range s.jobs {
		if j.released && !j.finished.IsZero() && s.now().Sub(j.finished) >= JobRetention {
			delete(s.jobs, id)
		}
	}
}
func copyJob(j *runningJob) *Job { v := j.view; return &v }

// Called within the authorization lease, never while awaiting a socket.
func (s *Service) start(ctx context.Context, q Command) (*Job, Code) {
	if !access.ValidCompanyID(q.Operation) {
		return nil, Invalid
	}
	st, c := s.store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return nil, c
	}
	d := device(&st, q.ID)
	if d == nil {
		return nil, Invalid
	}
	if d.Revision != q.Revision {
		return nil, Revision
	}
	if !d.Active {
		return nil, Inactive
	}
	credentialRevision := uint64(0)
	if q.Op == "device.query-start" {
		if d.Fingerprint == "" {
			return nil, TrustRequired
		}
		cr := credential(&st, d.CredentialID)
		if cr == nil {
			return nil, MissingCredential
		}
		credentialRevision = cr.Metadata.Revision
		if !s.vaultReady(&st) {
			return nil, VaultUnavailable
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if s.closed {
		return nil, Unavailable
	}
	for _, j := range s.jobs {
		if sameScope(j.command, q) && j.command.Operation == q.Operation {
			if j.command.ID != q.ID || j.command.Revision != q.Revision || j.command.Op != q.Op {
				return nil, Revision
			}
			return copyJob(j), OK
		}
	}
	stamp := fmt.Sprintf("job:%q:%d:%q:%d", q.Op, q.Revision, q.ID, credentialRevision)
	for _, op := range st.Operations {
		if op.ID == q.Operation {
			if op.Stamp != stamp {
				return nil, Revision
			}
			return nil, "JOB_INTERRUPTED"
		}
	}
	if len(st.Operations) >= MaxOperations {
		return nil, Limit
	}
	active := 0
	for _, j := range s.jobs {
		if !j.released {
			if Endpoint(j.device) == Endpoint(*d) {
				return nil, access.Busy
			}
			active++
		}
	}
	if active >= MaxConcurrent {
		return nil, access.Busy
	}
	if len(s.jobs) >= MaxJobs {
		return nil, Limit
	}
	id, c := s.ids.ID()
	if c != OK {
		return nil, c
	}
	c = s.store.UpdateEquipment(ctx, q.Scope.Profile, func(current *State) Code {
		current.Operations = append(current.Operations, Operation{q.Operation, stamp, id})
		if q.Op == "device.query-start" {
			target := device(current, q.ID)
			if target == nil || target.Revision != q.Revision {
				return Revision
			}
			target.LastAttempt = &Attempt{JobID: id, State: "STARTED", Code: OK, At: s.now().UTC().Format(time.RFC3339Nano)}
		}
		return OK
	})
	if c != OK {
		return nil, c
	}
	jobctx, cancel := context.WithTimeout(context.Background(), QueryBudget)
	j := &runningJob{view: Job{ID: id, DeviceID: d.ID, Kind: q.Op, State: "RUNNING", Code: OK}, command: q, device: *d, credentialRevision: credentialRevision, cancel: cancel}
	s.jobs[id] = j
	s.wg.Add(1)
	go s.run(jobctx, j)
	return copyJob(j), OK
}
func (s *Service) check(ctx context.Context, j *runningJob, reveal bool) (string, string, Code) {
	var user, secret string
	c := s.authorized(ctx, j.command, func(uint64) Code {
		if j.command.Op == "pppoe.collect-start" || j.command.Op == "pppoe.diagnostic-start" {
			return s.checkPP(ctx, j, reveal, &user, &secret)
		}
		st, c := s.store.ReadEquipment(ctx, j.command.Scope.Profile)
		if c != OK {
			return c
		}
		d := device(&st, j.device.ID)
		if d == nil || d.Revision != j.device.Revision {
			return Revision
		}
		if !d.Active {
			return Inactive
		}
		if j.command.Op == "trust.inspect" {
			return OK
		}
		cr := credential(&st, d.CredentialID)
		if cr == nil {
			return MissingCredential
		}
		if cr.Metadata.Revision != j.credentialRevision {
			return Revision
		}
		user = cr.Metadata.User
		if !reveal {
			if !s.vaultReady(&st) {
				return VaultUnavailable
			}
			return OK
		}
		secret, c = s.vault.Open(st.Installation, st.Company, cr.Metadata.ID, cr.Metadata.Revision, cr.Envelope, st.WrappedKey)
		return c
	})
	return user, secret, c
}
func (s *Service) run(ctx context.Context, j *runningJob) {
	if j.command.Op == "pppoe.diagnostic-start" {
		s.runPD(ctx, j)
		return
	}
	if j.command.Op == "pppoe.collect-start" {
		s.runPP(ctx, j)
		return
	}
	defer s.wg.Done()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	defer j.cancel()
	// Finite watcher promptly closes adapter resources after authorization changes.
	watchDone := make(chan struct{})
	watcherExited := make(chan struct{})
	defer func() { close(watchDone); <-watcherExited }()
	go func() {
		defer close(watcherExited)
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watchDone:
				return
			case <-ticker.C:
				if _, _, c := s.check(ctx, j, false); c != OK {
					j.cancel()
					return
				}
			}
		}
	}()
	user, secret, c := s.check(ctx, j, true)
	var result *Observation
	fingerprint := ""
	if c == OK {
		if j.command.Op == "trust.inspect" {
			fingerprint, c = s.remote.Inspect(ctx, j.device)
		} else {
			result, c = s.remote.Query(ctx, j.device, user, secret, func() Code { _, _, c := s.check(ctx, j, false); return c })
		}
	}
	secret = ""
	if ctx.Err() != nil {
		if ctx.Err() == context.DeadlineExceeded {
			c = "DEADLINE_EXCEEDED"
		} else {
			c = Cancelled
		}
		result = nil
		fingerprint = ""
	}

	finish := func(code Code) {
		if code != OK {
			result = nil
			fingerprint = ""
		}
		j.view.Result = result
		j.view.Fingerprint = fingerprint
		j.view.Code = code
		j.view.State = "FAILED"
		if code == OK {
			j.view.State = "SUCCEEDED"
		}
		if code == Cancelled {
			j.view.State = "CANCELLED"
		}
		j.finished = s.now()
	}
	// Persist completion under a fresh bounded context, including failures/cancellation.
	// The authorization/revision lease still guards the exact company and destination.
	persistCtx, cancelPersist := context.WithTimeout(context.Background(), time.Second)
	defer cancelPersist()
	outcome := c
	persisted := s.authorized(persistCtx, j.command, func(uint64) Code {
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() != nil || j.view.State == "CANCELLED" {
			outcome = Cancelled
			if ctx.Err() == context.DeadlineExceeded {
				outcome = "DEADLINE_EXCEEDED"
			}
			result = nil
		}
		code := s.store.UpdateEquipment(persistCtx, j.command.Scope.Profile, func(st *State) Code {
			d := device(st, j.device.ID)
			if d == nil || d.Revision != j.device.Revision || !d.Active {
				return Revision
			}
			if j.command.Op == "device.query-start" {
				cr := credential(st, d.CredentialID)
				if cr == nil || cr.Metadata.Revision != j.credentialRevision {
					return Revision
				}
				state := "FAILED"
				if outcome == OK {
					state = "SUCCEEDED"
				}
				if outcome == Cancelled {
					state = "CANCELLED"
				}
				d.LastAttempt = &Attempt{JobID: j.view.ID, State: state, Code: outcome, At: s.now().UTC().Format(time.RFC3339Nano)}
				if outcome == OK && result != nil {
					if result.InterfacePart != nil && result.InterfacePart.Status == "failed" && d.Result != nil {
						// Keep the prior interface part with its own provenance, never relabel it as current.
						result.Interfaces = d.Result.Interfaces
						part := InterfacePart{Status: "stale", Code: result.InterfacePart.Code, At: d.Result.At, Source: d.Result.Source}
						if d.Result.InterfacePart != nil {
							part = *d.Result.InterfacePart
							part.Status = "stale"
							part.Code = result.InterfacePart.Code
						}
						if part.At != "" {
							result.InterfacePart = &part
						}
					}
					d.Result = result
				}
			}
			return OK
		})
		if code == OK {
			finish(outcome)
		}
		return code
	})
	if persisted == OK {
		return
	}
	c = persisted
	if persisted == Unavailable || persisted == Limit {
		c = "PERSISTENCE_FAILED"
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() == context.DeadlineExceeded {
		c = "DEADLINE_EXCEEDED"
	} else if ctx.Err() != nil {
		c = Cancelled
	}
	finish(c)
}

func (s *Service) status(ctx context.Context, q Command) (*Job, Code) {
	return s.statusKind(ctx, q, "legacy")
}
func (s *Service) statusKind(ctx context.Context, q Command, kind string) (*Job, Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	j := s.jobs[q.JobID]
	if j == nil || !sameScope(q, j.command) {
		return nil, Changed
	}
	if (kind == "legacy" && j.view.Kind != "device.query-start" && j.view.Kind != "trust.inspect") || (kind != "legacy" && j.view.Kind != kind) {
		return nil, Invalid
	}
	if q.Op == "device.query-cancel" && (j.finished.IsZero() || j.command.Op == "trust.inspect") {
		j.cancel()
		j.view.Result = nil
		j.view.Fingerprint = ""
		j.view.Code = Cancelled
		j.view.State = "CANCELLED"
		if j.command.Op == "device.query-start" {
			code := s.store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
				d := device(st, j.device.ID)
				if d == nil || d.Revision != j.device.Revision {
					return Revision
				}
				if d.LastAttempt != nil && d.LastAttempt.JobID == j.view.ID {
					d.LastAttempt = &Attempt{JobID: j.view.ID, State: "CANCELLED", Code: Cancelled, At: s.now().UTC().Format(time.RFC3339Nano)}
				}
				return OK
			})
			if code != OK {
				return nil, "PERSISTENCE_FAILED"
			}
		}
	}
	return copyJob(j), OK
}
func (s *Service) confirm(ctx context.Context, q Command, v *View) Code {
	if !access.ValidCompanyID(q.Operation) {
		return Invalid
	}
	stamp := fmt.Sprintf("first-acceptance:%q:%q:%d:%q", q.ID, q.JobID, q.Revision, q.Fingerprint)
	st, c := s.store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return c
	}
	for _, op := range st.Operations {
		if op.ID == q.Operation {
			if !strings.HasPrefix(op.Stamp, stamp+";accepted:") {
				return Revision
			}
			v.Device = device(&st, q.ID)
			if v.Device == nil || v.Device.Fingerprint != q.Fingerprint {
				return Revision
			}
			return OK
		}
	}
	if len(st.Operations) >= MaxOperations {
		return Limit
	}
	s.mu.Lock()
	j := s.jobs[q.JobID]
	if j == nil || !sameScope(q, j.command) || j.command.Op != "trust.inspect" || j.view.State != "SUCCEEDED" || s.now().Sub(j.finished) > JobRetention || j.device.ID != q.ID || j.device.Revision != q.Revision || j.view.Fingerprint != q.Fingerprint {
		s.mu.Unlock()
		return TrustRequired
	}
	candidate := j.view.Fingerprint
	s.mu.Unlock()
	if !strings.HasPrefix(candidate, "SHA256:") {
		return Invalid
	}
	c = s.store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		d := device(st, q.ID)
		if d == nil || d.Revision != q.Revision {
			return Revision
		}
		if !d.Active {
			return Inactive
		}
		if d.Fingerprint != "" {
			if d.Fingerprint != candidate {
				return TrustChanged
			}
			return Revision // already pinned, never reclassify historical verification as a first acceptance
		}
		d.Fingerprint = candidate
		d.TrustAcceptance = &TrustAcceptance{Mode: "explicit-first-acceptance", At: s.now().UTC().Format(time.RFC3339Nano), UID: q.UID, Company: q.Scope.Profile, DeviceID: d.ID, Address: d.Address, Port: d.Port, Revision: d.Revision, Fingerprint: candidate}
		d.Revision++
		v.Device = d
		receipt := stamp + fmt.Sprintf(";accepted:%q", fmt.Sprint(*d.TrustAcceptance))
		st.Operations = append(st.Operations, Operation{q.Operation, receipt, q.ID})
		return OK
	})
	if c == OK {
		s.invalidate(q.Scope.Profile, q.ID, false)
	}
	return c
}
func (s *Service) invalidate(profile, id string, cred bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.command.Scope.Profile == profile && (j.device.ID == id || cred && j.device.CredentialID == id) {
			j.cancel()
			j.view.Result = nil
			j.view.Fingerprint = ""
			j.view.Code = Cancelled
			j.view.State = "CANCELLED"
		}
	}
}
func (s *Service) Close() {
	s.mu.Lock()
	s.closed = true
	for _, j := range s.jobs {
		j.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

// Existing ciphertext must never be paired with a freshly generated company key.
func (s *Service) vaultReady(st *State) bool {
	if len(st.Credentials) > 0 && len(st.WrappedKey) == 0 {
		return false
	}
	return s.vault.Ready(st.Installation, st.Company, st.WrappedKey)
}

// VaultAvailable probes only the configured provider; no account, key or ciphertext leaves the service.
func (s *Service) VaultAvailable() bool { return s.vault.Available() }
