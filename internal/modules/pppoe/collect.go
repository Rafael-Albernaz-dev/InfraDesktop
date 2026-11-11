package pppoe

import (
	"context"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/inventory"
	"time"
)

type reserveFunc func(context.Context, string, PPInput, string, string) (string, bool, Code)

func (s *Service) startPP(ctx context.Context, q Command, in PPInput) (*Job, Code) {
	t, reserve, c := s.admitStartPP(ctx, q, in)
	if c != OK {
		return nil, c
	}
	d := t.Device
	q.ID, q.Operation, q.Revision = in.DeviceID, in.Operation, in.Revision
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	s.inv.PruneLocked()
	if s.inv.Closed {
		return nil, Unavailable
	}
	diagnostic := q.Op == "pppoe.diagnostic-start"
	if existing, code := s.findOrAdmitPPJobLocked(q, in, d, diagnostic); existing != nil || code != OK {
		return existing, code
	}
	in.SavedAt = PPTime(s.inv.Now())
	in.WorkerEpoch = s.inv.Epoch
	stamp := ppStamp(q.Op, in)
	ping, c := s.optionalPingTarget(ctx, q, in, t, diagnostic)
	if c != OK {
		return nil, c
	}
	id, c := s.inv.IDs.ID()
	if c != OK {
		return nil, c
	}
	entity, known, c := reserve(ctx, q.Scope.Profile, in, stamp, id)
	if c != OK {
		return nil, c
	}
	if known {
		return &Job{ID: entity}, CodeJobInterrupted
	}
	return s.launchPPJobLocked(id, q, d, t.Credential.Metadata.Revision, in, ping, diagnostic), OK
}

func (s *Service) admitStartPP(
	ctx context.Context, q Command, in PPInput,
) (PPTarget, reserveFunc, Code) {
	if !access.ValidCompanyID(in.Operation) || in.Revision == 0 {
		return PPTarget{}, nil, Invalid
	}
	store, ok := s.inv.Store.(PPStore)
	if !ok {
		return PPTarget{}, nil, Unavailable
	}
	reserve, c := s.resolveReserveFunc(store, q.Op == "pppoe.diagnostic-start")
	if c != OK {
		return PPTarget{}, nil, c
	}
	t, c := store.ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, true)
	if c != OK {
		return PPTarget{}, nil, c
	}
	if c := s.validatePPTargetDevice(t, in.Revision); c != OK {
		return PPTarget{}, nil, c
	}
	return t, reserve, OK
}

func (s *Service) resolveReserveFunc(store PPStore, diagnostic bool) (reserveFunc, Code) {
	if diagnostic {
		if _, ok := s.inv.Remote.(PDDiagnosticRemote); !ok {
			return nil, Unavailable
		}
		diagnosticStore, ok := s.inv.Store.(PDDiagnosticStore)
		if !ok {
			return nil, Unavailable
		}
		return diagnosticStore.ReservePPDiagnostic, OK
	}
	if _, ok := s.inv.Remote.(PPRemote); !ok {
		return nil, Unavailable
	}
	return store.ReservePPCollection, OK
}

func (s *Service) validatePPTargetDevice(t PPTarget, revision uint64) Code {
	d := t.Device
	if d.Revision != revision {
		return Revision
	}
	if !d.Active {
		return Inactive
	}
	if d.Fingerprint == "" {
		return TrustRequired
	}
	if !s.inv.Vault.Ready(t.Installation, t.Company, t.Wrapped) {
		return VaultUnavailable
	}
	return OK
}

func (s *Service) findOrAdmitPPJobLocked(
	q Command, in PPInput, d Device, diagnostic bool,
) (*Job, Code) {
	for _, j := range s.inv.Jobs {
		if j.View.State == "RUNNING" && inventory.SameScope(q, j.Command) && j.Command.Operation == q.Operation {
			if j.Command.Op != q.Op || j.Device.ID != d.ID || j.Device.Revision != d.Revision ||
				(diagnostic && ppStamp(q.Op, j.PPInput) != ppStamp(q.Op, in)) {
				return nil, Revision
			}
			return inventory.CopyJob(j), OK
		}
	}
	active := 0
	for _, j := range s.inv.Jobs {
		if !j.Released {
			active++
			if inventory.JobUsesEndpoint(j, d) {
				return nil, access.Busy
			}
		}
	}
	if active >= MaxConcurrent {
		return nil, access.Busy
	}
	if len(s.inv.Jobs) >= MaxJobs {
		return nil, CodeJobsFull
	}
	return nil, OK
}

func (s *Service) optionalPingTarget(
	ctx context.Context, q Command, in PPInput, t PPTarget, diagnostic bool,
) (*PPPingTarget, Code) {
	if !diagnostic || in.Scope != PDPing {
		return nil, OK
	}
	return s.preparePPPing(ctx, q, in, t)
}

func (s *Service) launchPPJobLocked(
	id string, q Command, d Device, credRev uint64, in PPInput, ping *PPPingTarget, diagnostic bool,
) *Job {
	jobctx, cancel := context.WithTimeout(context.Background(), QueryBudget)
	j := &RunningJob{
		View:               Job{ID: id, DeviceID: d.ID, Kind: q.Op, State: "RUNNING", Code: OK},
		Command:            q,
		Device:             d,
		CredentialRevision: credRev,
		Cancel:             cancel,
		PPInput:            in,
		Ping:               ping,
	}
	s.inv.Jobs[id] = j
	s.inv.WG.Add(1)
	if diagnostic {
		go s.runPD(jobctx, j)
	} else {
		go s.runPPRecorded(jobctx, j)
	}
	return inventory.CopyJob(j)
}

func (s *Service) runPP(ctx context.Context, j *RunningJob) {
	defer s.inv.WG.Done()
	defer func() { s.inv.Mu.Lock(); j.Released = true; s.inv.Mu.Unlock() }()
	defer j.Cancel()
	defer s.inv.Watch(ctx, j)()
	o, a := s.executePPCollect(ctx, j)
	s.persistPPCollect(ctx, j, o, &a)
}

func (s *Service) executePPCollect(ctx context.Context, j *RunningJob) (*PPObservation, PPAttempt) {
	user, secret, c := s.inv.Check(ctx, j, true)
	var o *PPObservation
	started := s.inv.Now()
	startMono := time.Since(s.inv.Origin).Nanoseconds()
	if c == OK {
		checkFn := func() Code { _, _, code := s.inv.Check(ctx, j, false); return code }
		o, c = s.inv.Remote.(PPRemote).CollectPPPoE(ctx, j.Device, user, secret, checkFn)
	}
	secret = ""
	a := PPAttempt{Epoch: s.inv.Epoch, JobID: j.View.ID, At: PPTime(s.inv.Now()), Code: c, State: "FAILED"}
	o, c = s.evaluatePPCollectOutcome(ctx, j, o, c, started, startMono, &a)
	a.Code = c
	if c == OK {
		a.State = "SUCCEEDED"
	} else if c == Cancelled {
		a.State = "CANCELLED"
	}
	if o != nil {
		a.ObservationID = o.ID
	}
	return o, a
}

func (s *Service) evaluatePPCollectOutcome(
	ctx context.Context, j *RunningJob, o *PPObservation, c Code,
	started time.Time, startMono int64, a *PPAttempt,
) (*PPObservation, Code) {
	if o != nil {
		a.Quality = o.Quality
		if c == OK && o.Quality.Status != "complete" {
			c = o.Quality.Code
			if c == OK {
				c = Partial
			}
		}
	} else if c == OK {
		c = Partial
	}
	if ctx.Err() != nil {
		o = nil
		c = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			c = CodeDeadlineExceeded
		}
	}
	if c == OK && o != nil && o.Quality.Status == "complete" {
		c = s.stampObservationProvenance(j, o, started, startMono)
	} else {
		o = nil
	}
	if c != OK {
		o = nil
	}
	return o, c
}

func (s *Service) stampObservationProvenance(
	j *RunningJob, o *PPObservation, started time.Time, startMono int64,
) Code {
	var c Code
	o.ID, c = s.inv.IDs.ID()
	o.DeviceID = j.Device.ID
	o.Scope = PPScope
	o.Provenance.Epoch = s.inv.Epoch
	o.Provenance.Started = PPTime(started)
	o.Provenance.Ended = PPTime(s.inv.Now())
	o.Provenance.Sample = PPInterval{
		Min: startMono, Max: time.Since(s.inv.Origin).Nanoseconds(), Known: true,
	}
	o.Provenance.Connection = PPConnection(j.Device)
	o.Provenance.Revision = j.Device.Revision
	o.Provenance.CredentialRevision = j.CredentialRevision
	for i := range o.Rows {
		if c != OK {
			break
		}
		o.Rows[i].ID, c = s.inv.IDs.ID()
		o.Rows[i].Sample = o.Provenance.Sample
	}
	return c
}

func (s *Service) persistPPCollect(
	ctx context.Context, j *RunningJob, o *PPObservation, a *PPAttempt,
) {
	// Each write try has PPPersistenceBudget; a slow disk retries it.
	persisted := s.inv.PersistWithRetry(PPPersistenceBudget, func(persistCtx context.Context) Code {
		return s.inv.Authorized(persistCtx, j.Command, func(uint64) Code {
			return s.commitPPCollection(persistCtx, ctx, j, o, a)
		})
	})
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	if persisted != OK {
		a.Code = persisted
		a.State = "FAILED"
		a.ObservationID = ""
		if persisted == Unavailable {
			a.Code = CodePersistenceFailed
		}
	}
	copyAttempt := *a
	j.View.Code = a.Code
	j.View.State = a.State
	j.View.PPPoE = &copyAttempt
	j.Finished = s.inv.Now()
}

func (s *Service) commitPPCollection(
	persistCtx, jobCtx context.Context, j *RunningJob, o *PPObservation, a *PPAttempt,
) Code {
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	if jobCtx.Err() != nil || j.View.State == "CANCELLED" {
		o = nil
		a.Code = Cancelled
		a.State = "CANCELLED"
		a.ObservationID = ""
	}
	store := s.inv.Store.(PPStore)
	t, code := store.ReadPPTarget(persistCtx, j.Command.Scope.Profile, j.Device.ID, false)
	if code != OK {
		return code
	}
	if t.Device.Revision != j.Device.Revision || t.Credential.Metadata.Revision != j.CredentialRevision {
		return Revision
	}
	if !s.inv.Vault.Ready(t.Installation, t.Company, t.Wrapped) {
		a.Code = VaultUnavailable
		a.State = "FAILED"
		a.ObservationID = ""
		a.Quality.Status = "failed"
		a.Quality.Code = VaultUnavailable
		return store.FinishPPCollection(persistCtx, j.Command.Scope.Profile, t, j.View.ID, *a, nil)
	}
	if o != nil {
		o.Provenance.Installation = t.Installation
		o.Provenance.Company = t.Company
	}
	code = store.FinishPPCollection(persistCtx, j.Command.Scope.Profile, t, j.View.ID, *a, o)
	if code == Limit && o != nil {
		a.Code = Limit
		a.State = "FAILED"
		a.ObservationID = ""
		a.Quality.Status = "limited"
		a.Quality.Code = Limit
		a.Quality.Reason = "storage"
		return store.FinishPPCollection(persistCtx, j.Command.Scope.Profile, t, j.View.ID, *a, nil)
	}
	return code
}
