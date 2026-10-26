package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"time"
)

func (s *Service) handlePD(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	if q.Op == "pppoe.diagnostic-status" || q.Op == "pppoe.diagnostic-cancel" {
		q.JobID = in.JobID
		if q.Op == "pppoe.diagnostic-cancel" {
			q.Op = "device.query-cancel"
		} else {
			q.Op = "device.query-status"
		}
		var c Code
		v.Job, c = s.statusKind(ctx, q, "pppoe.diagnostic-start")
		return c
	}
	if !access.ValidCompanyID(in.DeviceID) || !PDValidScope(in.Scope) {
		return Invalid
	}
	store, ok := s.store.(PDDiagnosticStore)
	if !ok {
		return Unavailable
	}
	switch q.Op {
	case "pppoe.diagnostic-start":
		if in.Scope == PDPing {
			if !access.ValidCompanyID(in.ObservationID) || !access.ValidCompanyID(in.RowID) {
				return Invalid
			}
		} else if in.ObservationID != "" || in.RowID != "" {
			return Invalid
		}
		var c Code
		v.Job, c = s.startPP(ctx, q, in)
		if c == "pp_jobs_full" {
			v.Reason = "job_slots"
			return Limit
		}
		if c == Limit {
			v.Reason = "operation_receipts"
		}
		if c == "JOB_INTERRUPTED" && v.Job != nil {
			v.Entity = v.Job.ID
			v.Job = nil
		}
		return c
	case "pppoe.diagnostic-current":
		result, c := store.PPDiagnosticCurrent(ctx, q.Scope.Profile, in)
		*v = result
		if c == OK && v.Attempt != nil && v.Attempt.State == "RUNNING" {
			s.mu.Lock()
			j := s.jobs[v.Attempt.JobID]
			if j != nil && j.view.PPPoE != nil {
				a := *j.view.PPPoE
				v.Attempt = &a
			} else if j == nil || v.Attempt.Epoch != s.epoch {
				v.Attempt.State = "INTERRUPTED"
				v.Attempt.Code = "JOB_INTERRUPTED"
				v.Attempt.Quality.Status = "failed"
				v.Attempt.Quality.Code = "JOB_INTERRUPTED"
			}
			s.mu.Unlock()
		}
		return c
	case "pppoe.diagnostic-list":
		if !access.ValidCompanyID(in.DiagnosticID) {
			return Invalid
		}
		result, c := store.PPDiagnosticPage(ctx, q.Scope.Profile, in)
		*v = result
		return c
	}
	return Invalid
}

func (s *Service) preparePPPing(ctx context.Context, q Command, in PPInput, target PPTarget) (*PPPingTarget, Code) {
	store := s.store.(PPStore)
	current, c := store.PPCurrent(ctx, q.Scope.Profile, in.DeviceID)
	if c != OK {
		return nil, c
	}
	o := current.Current
	if o == nil || o.ID != in.ObservationID {
		return nil, ObservationMissing
	}
	p := o.Provenance
	now := time.Since(s.origin).Nanoseconds()
	if p.Epoch != s.epoch || p.Connection != PPConnection(target.Device) || p.Revision != target.Device.Revision || p.CredentialRevision != target.Credential.Metadata.Revision || !p.Sample.Known || p.Sample.Min < 0 || p.Sample.Max < p.Sample.Min || now < p.Sample.Max || now-p.Sample.Min > int64(PDPingAge) || !p.RouterUptime.Known {
		return nil, ObservationIncompatible
	}
	detail, c := store.PPSession(ctx, q.Scope.Profile, in)
	if c != OK {
		return nil, c
	}
	if len(detail.Rows) != 1 {
		return nil, ObservationMissing
	}
	row := detail.Rows[0]
	address, ok := PPPingAddress(row)
	if !ok || !row.Uptime.Known || row.RemoteID == "" || row.SessionID == "" {
		return nil, ObservationIncompatible
	}
	row.Address = address
	return &PPPingTarget{Observation: *o, Row: row, Age: PPInterval{Min: now - p.Sample.Max, Max: now - p.Sample.Min, Known: true}, Prepared: time.Now()}, OK
}

func (s *Service) runPD(ctx context.Context, j *runningJob) {
	defer s.wg.Done()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	defer j.cancel()
	defer s.watchPP(ctx, j)()
	user, secret, c := s.check(ctx, j, true)
	started := s.now()
	startMono := time.Since(s.origin).Nanoseconds()
	var o *PDDiagnostic
	if c == OK {
		o, c = s.remote.(PDDiagnosticRemote).CollectPPDiagnostic(ctx, j.device, user, secret, j.ppInput.Scope, j.ping, func() Code { _, _, c := s.check(ctx, j, false); return c })
	}
	secret = ""
	attempt := PPAttempt{Epoch: s.epoch, JobID: j.view.ID, At: PPTime(s.now()), Code: c, State: "FAILED", Operation: j.ppInput.Operation, Scope: j.ppInput.Scope, SourceObservation: j.ppInput.ObservationID, SourceRow: j.ppInput.RowID}
	if o != nil {
		attempt.Quality = o.Quality
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
		c = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			c = "DEADLINE_EXCEEDED"
		}
	}
	if c == OK && o != nil {
		o.ID, c = s.ids.ID()
		o.DeviceID = j.device.ID
		o.Scope = j.ppInput.Scope
		p := &o.Provenance
		p.Epoch = s.epoch
		p.Started = PPTime(started)
		p.Ended = PPTime(s.now())
		p.Sample = PPInterval{Min: startMono, Max: time.Since(s.origin).Nanoseconds(), Known: true}
		p.Connection = PPConnection(j.device)
		p.Revision = j.device.Revision
		p.CredentialRevision = j.credentialRevision
		if j.ping != nil {
			o.SourceObservation = j.ping.Observation.ID
			o.SourceRow = j.ping.Row.ID
		}
		for i := range o.Rows {
			if c != OK {
				break
			}
			o.Rows[i].ID, c = s.ids.ID()
		}
		ResolvePDRelations(o.Rows)
	}
	if c != OK {
		o = nil
		if attempt.Quality.Status == "" {
			attempt.Quality.Status = "failed"
			attempt.Quality.Code = c
		}
		if c == Limit {
			attempt.Quality.Status = "limited"
			attempt.Quality.Code = Limit
			attempt.Quality.Reason = "wire_bytes"
		}
	}
	attempt.Code = c
	if c == OK {
		attempt.State = "SUCCEEDED"
	}
	if c == Cancelled {
		attempt.State = "CANCELLED"
	}
	if o != nil {
		attempt.ObservationID = o.ID
	}
	persist, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	persisted := s.authorized(persist, j.command, func(uint64) Code {
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() != nil || j.view.State == "CANCELLED" {
			o = nil
			attempt.Code = Cancelled
			attempt.State = "CANCELLED"
			attempt.ObservationID = ""
		}
		target, code := s.store.(PPStore).ReadPPTarget(persist, j.command.Scope.Profile, j.device.ID, false)
		if code != OK {
			return code
		}
		if target.Device.Revision != j.device.Revision || target.Credential.Metadata.Revision != j.credentialRevision {
			return Revision
		}
		if !s.vault.Ready(target.Installation, target.Company, target.Wrapped) {
			o = nil
			attempt.Code = VaultUnavailable
			attempt.State = "FAILED"
			attempt.ObservationID = ""
		}
		if o != nil {
			o.Provenance.Installation = target.Installation
			o.Provenance.Company = target.Company
		}
		store := s.store.(PDDiagnosticStore)
		code = store.FinishPPDiagnostic(persist, j.command.Scope.Profile, target, j.ppInput, j.view.ID, attempt, o)
		if code == Limit && o != nil {
			attempt.Code = Limit
			attempt.State = "FAILED"
			attempt.ObservationID = ""
			attempt.Quality.Status = "limited"
			attempt.Quality.Code = Limit
			attempt.Quality.Reason = "storage"
			return store.FinishPPDiagnostic(persist, j.command.Scope.Profile, target, j.ppInput, j.view.ID, attempt, nil)
		}
		return code
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	if persisted != OK {
		attempt.Code = persisted
		attempt.State = "FAILED"
		attempt.ObservationID = ""
		if persisted == Unavailable {
			attempt.Code = "PERSISTENCE_FAILED"
		}
	}
	j.view.Code = attempt.Code
	j.view.State = attempt.State
	j.view.PPPoE = &attempt
	j.finished = s.now()
}
