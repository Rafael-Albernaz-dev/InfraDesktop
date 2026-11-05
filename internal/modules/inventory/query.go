package inventory

import (
	"context"
	"infradesktop/internal/modules/access"
	"strings"
	"time"
)

func (s *Service) start(ctx context.Context, q Command) (*Job, Code) {
	if !ValidID(q.Operation) {
		return nil, Invalid
	}
	d, credentialRevision, c := s.admitQueryStart(ctx, q)
	if c != OK {
		return nil, c
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.PruneLocked()
	if s.Closed {
		return nil, Unavailable
	}
	if existing, code := s.findOrAdmitQueryJobLocked(q, *d); existing != nil || code != OK {
		return existing, code
	}
	id, c := s.IDs.ID()
	if c != OK {
		return nil, c
	}
	if c := s.recordStartedAttempt(ctx, q, id); c != OK {
		return nil, c
	}
	jobctx, cancel := context.WithTimeout(context.Background(), QueryBudget)
	j := &RunningJob{
		View:               Job{ID: id, DeviceID: d.ID, Kind: q.Op, State: "RUNNING", Code: OK},
		Command:            q,
		Device:             *d,
		CredentialRevision: credentialRevision,
		Cancel:             cancel,
	}
	s.Jobs[id] = j
	s.WG.Add(1)
	go s.run(jobctx, j)
	return CopyJob(j), OK
}

func (s *Service) admitQueryStart(ctx context.Context, q Command) (*Device, uint64, Code) {
	st, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return nil, 0, c
	}
	d := DeviceByID(&st, q.ID)
	if d == nil {
		return nil, 0, Invalid
	}
	if d.Revision != q.Revision {
		return nil, 0, Revision
	}
	if !d.Active {
		return nil, 0, Inactive
	}
	if d.Role == RolePTP || roleWithoutDriver(d.Role) {
		return nil, 0, CodeUnsupported
	}
	if q.Op != "device.query-start" {
		return d, 0, OK
	}
	if d.Fingerprint == "" {
		return nil, 0, TrustRequired
	}
	cr := CredentialByID(&st, d.CredentialID)
	if cr == nil {
		return nil, 0, MissingCredential
	}
	if !s.VaultReady(&st) {
		return nil, 0, VaultUnavailable
	}
	return d, cr.Metadata.Revision, OK
}

func (s *Service) findOrAdmitQueryJobLocked(q Command, d Device) (*Job, Code) {
	for _, j := range s.Jobs {
		if j.View.State == "RUNNING" && SameScope(j.Command, q) && j.Command.Operation == q.Operation {
			if j.Command.ID != q.ID || j.Command.Revision != q.Revision || j.Command.Op != q.Op {
				return nil, Revision
			}
			return CopyJob(j), OK
		}
	}
	active := 0
	for _, j := range s.Jobs {
		if !j.Released {
			if JobUsesEndpoint(j, d) {
				return nil, access.Busy
			}
			active++
		}
	}
	if active >= MaxConcurrent {
		return nil, access.Busy
	}
	if len(s.Jobs) >= MaxJobs {
		return nil, Limit
	}
	return nil, OK
}

func (s *Service) recordStartedAttempt(ctx context.Context, q Command, id string) Code {
	return s.Store.UpdateEquipment(ctx, q.Scope.Profile, func(current *State) Code {
		if q.Op == "device.query-start" {
			target := DeviceByID(current, q.ID)
			if target == nil || target.Revision != q.Revision {
				return Revision
			}
			at := s.Now().UTC().Format(time.RFC3339Nano)
			target.LastAttempt = &Attempt{JobID: id, State: "STARTED", Code: OK, At: at}
		}
		return OK
	})
}

func (s *Service) Check(ctx context.Context, j *RunningJob, reveal bool) (string, string, Code) {
	var user, secret string
	c := s.Authorized(ctx, j.Command, func(uint64) Code {
		if isPPPoEOrRadioJobOp(j.Command.Op) {
			return s.CheckPP(ctx, j, reveal, &user, &secret)
		}
		return s.checkInventoryJob(ctx, j, reveal, &user, &secret)
	})
	return user, secret, c
}

func isPPPoEOrRadioJobOp(op string) bool {
	return op == "pppoe.collect-start" ||
		op == "pppoe.diagnostic-start" ||
		op == "pppoe.read-start" ||
		strings.HasPrefix(op, "radio.")
}

func (s *Service) checkInventoryJob(ctx context.Context, j *RunningJob, reveal bool, user, secret *string) Code {
	st, c := s.Store.ReadEquipment(ctx, j.Command.Scope.Profile)
	if c != OK {
		return c
	}
	d := DeviceByID(&st, j.Device.ID)
	if d == nil || d.Revision != j.Device.Revision {
		return Revision
	}
	if !d.Active {
		return Inactive
	}
	if j.Command.Op == "trust.inspect" {
		return OK
	}
	cr := CredentialByID(&st, d.CredentialID)
	if cr == nil {
		return MissingCredential
	}
	if cr.Metadata.Revision != j.CredentialRevision {
		return Revision
	}
	*user = cr.Metadata.User
	if !reveal {
		if !s.VaultReady(&st) {
			return VaultUnavailable
		}
		return OK
	}
	*secret, c = s.Vault.Open(
		st.Installation, st.Company, cr.Metadata.ID, cr.Metadata.Revision, cr.Envelope, st.WrappedKey,
	)
	return c
}

func (s *Service) CheckPP(ctx context.Context, j *RunningJob, reveal bool, user, secret *string) Code {
	store, ok := s.Store.(PPStore)
	if !ok {
		return Unavailable
	}
	t, c := store.ReadPPTarget(ctx, j.Command.Scope.Profile, j.Device.ID, reveal)
	if c != OK {
		return c
	}
	if !matchingPPTarget(t, j) {
		return Revision
	}
	if !t.Device.Active {
		return Inactive
	}
	if !s.Vault.Ready(t.Installation, t.Company, t.Wrapped) {
		return VaultUnavailable
	}
	if reveal {
		*user = t.Credential.Metadata.User
		*secret, c = s.Vault.Open(
			t.Installation, t.Company, t.Credential.Metadata.ID,
			t.Credential.Metadata.Revision, t.Credential.Envelope, t.Wrapped,
		)
		return c
	}
	return OK
}

func matchingPPTarget(t PPTarget, j *RunningJob) bool {
	return t.Device.Revision == j.Device.Revision &&
		PPConnection(t.Device) == PPConnection(j.Device) &&
		t.Credential.Metadata.Revision == j.CredentialRevision
}

func (s *Service) Watch(ctx context.Context, j *RunningJob) func() {
	watcherDone := make(chan struct{})
	watcherExit := make(chan struct{})
	go func() {
		defer close(watcherExit)
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watcherDone:
				return
			case <-ticker.C:
				if _, _, c := s.Check(ctx, j, false); c != OK {
					j.Cancel()
					return
				}
			}
		}
	}()
	return func() { close(watcherDone); <-watcherExit }
}

func (s *Service) run(ctx context.Context, j *RunningJob) {
	defer s.WG.Done()
	defer func() { s.Mu.Lock(); j.Released = true; s.Mu.Unlock() }()
	defer j.Cancel()
	defer s.Watch(ctx, j)()
	result, fingerprint, c := s.executeRemoteQuery(ctx, j)
	if ctx.Err() != nil {
		c = contextCancelCode(ctx)
		result = nil
		fingerprint = ""
	}
	s.persistAndFinishQuery(ctx, j, result, fingerprint, c)
}

func (s *Service) executeRemoteQuery(ctx context.Context, j *RunningJob) (*Observation, string, Code) {
	user, secret, c := s.Check(ctx, j, true)
	if c != OK {
		return nil, "", c
	}
	if j.Command.Op == "trust.inspect" {
		fp, code := s.Remote.Inspect(ctx, j.Device)
		return nil, fp, code
	}
	checkFn := func() Code { _, _, code := s.Check(ctx, j, false); return code }
	obs, code := s.Remote.Query(ctx, j.Device, user, secret, checkFn)
	return obs, "", code
}

func contextCancelCode(ctx context.Context) Code {
	if ctx.Err() == context.DeadlineExceeded {
		return CodeDeadlineExceeded
	}
	return Cancelled
}

func (s *Service) persistAndFinishQuery(
	ctx context.Context, j *RunningJob, result *Observation, fingerprint string, c Code,
) {
	persistCtx, cancelPersist := context.WithTimeout(context.Background(), time.Second)
	defer cancelPersist()
	outcome := c
	persisted := s.Authorized(persistCtx, j.Command, func(uint64) Code {
		s.Mu.Lock()
		defer s.Mu.Unlock()
		if ctx.Err() != nil || j.View.State == "CANCELLED" {
			outcome = contextCancelCode(ctx)
			result = nil
		}
		code := s.Store.UpdateEquipment(persistCtx, j.Command.Scope.Profile, func(st *State) Code {
			return s.applyQueryUpdate(st, j, result, fingerprint, &outcome)
		})
		if code == OK {
			s.finishQueryJobLocked(j, result, fingerprint, outcome)
		}
		return code
	})
	if persisted == OK {
		return
	}
	c = persisted
	if persisted == Unavailable || persisted == Limit {
		c = CodePersistenceFailed
	}
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if ctx.Err() != nil {
		c = contextCancelCode(ctx)
	}
	s.finishQueryJobLocked(j, result, fingerprint, c)
}

func (s *Service) finishQueryJobLocked(j *RunningJob, result *Observation, fingerprint string, code Code) {
	if code != OK {
		result = nil
		fingerprint = ""
	}
	j.View.Result = result
	j.View.Fingerprint = fingerprint
	j.View.Code = code
	j.View.State = jobStateForCode(code)
	j.Finished = s.Now()
}

func jobStateForCode(code Code) string {
	if code == OK {
		return "SUCCEEDED"
	}
	if code == Cancelled {
		return "CANCELLED"
	}
	return "FAILED"
}

func (s *Service) applyQueryUpdate(
	st *State, j *RunningJob, result *Observation, fingerprint string, outcome *Code,
) Code {
	d := DeviceByID(st, j.Device.ID)
	if d == nil || d.Revision != j.Device.Revision || !d.Active {
		return Revision
	}
	if j.Command.Op == "trust.inspect" && *outcome == OK && (d.Role == RoleAP || d.Role == RoleCPE) {
		return s.applyAutomaticTOFU(st, d, j, fingerprint, outcome)
	}
	if j.Command.Op == "device.query-start" {
		return s.applyDeviceQueryResult(st, d, j, result, *outcome)
	}
	return OK
}

func (s *Service) applyAutomaticTOFU(
	st *State, d *Device, j *RunningJob, fingerprint string, outcome *Code,
) Code {
	if d.Fingerprint != "" && d.Fingerprint != fingerprint {
		*outcome = TrustChanged
		return OK
	}
	if d.Fingerprint == "" {
		if len(st.Operations) >= MaxOperations {
			return Limit
		}
		d.Fingerprint = fingerprint
		d.TrustOrigin = "automatic-tofu"
		d.TrustAcceptance = &TrustAcceptance{
			Mode: "automatic-tofu", At: PPTime(s.Now()), UID: j.Command.UID,
			Company: j.Command.Scope.Profile, DeviceID: d.ID, Address: d.Address,
			Port: d.Port, Revision: d.Revision, Fingerprint: fingerprint,
		}
		d.Revision++
		op := Operation{ID: j.Command.Operation, Stamp: "automatic-tofu:" + fingerprint, Entity: d.ID}
		st.Operations = append(st.Operations, op)
	}
	return OK
}

func (s *Service) applyDeviceQueryResult(
	st *State, d *Device, j *RunningJob, result *Observation, outcome Code,
) Code {
	cr := CredentialByID(st, d.CredentialID)
	if cr == nil || cr.Metadata.Revision != j.CredentialRevision {
		return Revision
	}
	at := s.Now().UTC().Format(time.RFC3339Nano)
	d.LastAttempt = &Attempt{JobID: j.View.ID, State: jobStateForCode(outcome), Code: outcome, At: at}
	if outcome == OK && result != nil {
		preserveStaleInterfaces(d, result)
		d.Result = result
	}
	return OK
}

func preserveStaleInterfaces(d *Device, result *Observation) {
	if result.InterfacePart == nil || result.InterfacePart.Status != "failed" || d.Result == nil {
		return
	}
	result.Interfaces = d.Result.Interfaces
	part := InterfacePart{
		Status: "stale", Code: result.InterfacePart.Code, At: d.Result.At, Source: d.Result.Source,
	}
	if d.Result.InterfacePart != nil {
		part = *d.Result.InterfacePart
		part.Status = "stale"
		part.Code = result.InterfacePart.Code
	}
	if part.At != "" {
		result.InterfacePart = &part
	}
}
