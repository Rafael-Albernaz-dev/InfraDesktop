package equipment

import (
	"context"
	"fmt"
	"strings"

	"infradesktop/internal/modules/access"
	"time"
)

func ppStamp(kind string, in PPInput) string {
	if kind == "pppoe.diagnostic-start" {
		return fmt.Sprintf("ppdiag:%q", []string{kind, in.DeviceID, fmt.Sprint(in.Revision), in.Scope, in.ObservationID, in.RowID})
	}
	return fmt.Sprintf("ppref:%q", []string{kind, in.DeviceID, fmt.Sprint(in.Revision), in.ObservationID, in.ReferenceID})
}
func (s *Service) checkPP(ctx context.Context, j *runningJob, reveal bool, user, secret *string) Code {
	store, ok := s.store.(PPStore)
	if !ok {
		return Unavailable
	}
	t, c := store.ReadPPTarget(ctx, j.command.Scope.Profile, j.device.ID, reveal)
	if c != OK {
		return c
	}
	if t.Device.Revision != j.device.Revision || PPConnection(t.Device) != PPConnection(j.device) || t.Credential.Metadata.Revision != j.credentialRevision {
		return Revision
	}
	if !t.Device.Active {
		return Inactive
	}
	if !s.vault.Ready(t.Installation, t.Company, t.Wrapped) {
		return VaultUnavailable
	}
	if reveal {
		*user = t.Credential.Metadata.User
		var c Code
		*secret, c = s.vault.Open(t.Installation, t.Company, t.Credential.Metadata.ID, t.Credential.Metadata.Revision, t.Credential.Envelope, t.Wrapped)
		return c
	}
	return OK
}
func (s *Service) HandlePP(ctx context.Context, q Command, in PPInput) (PPView, Code) {
	v := PPView{}
	c := s.authorized(ctx, q, func(version uint64) Code {
		q.Authority = version
		store, ok := s.store.(PPStore)
		if !ok {
			return Unavailable
		}
		if !PPValidQuery(in.Query) || in.Offset < 0 || in.Offset > 2*PPMaxRows {
			return Invalid
		}
		if strings.HasPrefix(q.Op, "pppoe.diagnostic-") {
			return s.handlePD(ctx, q, in, &v)
		}
		if q.Op == "pppoe.collect-status" || q.Op == "pppoe.collect-cancel" {
			q.JobID = in.JobID
			if q.Op == "pppoe.collect-cancel" {
				q.Op = "device.query-cancel"
			} else {
				q.Op = "device.query-status"
			}
			var c Code
			v.Job, c = s.statusKind(ctx, q, "pppoe.collect-start")
			if v.Job != nil && v.Job.Kind != "pppoe.collect-start" {
				return Invalid
			}
			return c
		}
		if !access.ValidCompanyID(in.DeviceID) {
			return Invalid
		}
		if q.Op == "pppoe.collect-start" {
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
		}
		if q.Op == "pppoe.export-page" && in.ExportPart == "comparison" {
			q.Op = "pppoe.compare"
		}
		switch q.Op {
		case "pppoe.current":
			var c Code
			v, c = store.PPCurrent(ctx, q.Scope.Profile, in.DeviceID)
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
		case "pppoe.observation-get":
			var c Code
			v.Current, c = store.PPObservation(ctx, q.Scope.Profile, in.DeviceID, in.ObservationID)
			return c
		case "pppoe.sessions-list":
			var c Code
			v, c = store.PPPage(ctx, q.Scope.Profile, in)
			return c
		case "pppoe.session-get":
			var c Code
			v, c = store.PPSession(ctx, q.Scope.Profile, in)
			return c
		case "pppoe.reference-list":
			var c Code
			v, c = store.PPReferences(ctx, q.Scope.Profile, in)
			return c
		case "pppoe.reference-save":
			if !access.ValidCompanyID(in.Operation) || !access.ValidCompanyID(in.ReferenceID) || !access.ValidCompanyID(in.ObservationID) {
				return Invalid
			}
			var c Code
			in.SavedAt = PPTime(s.now())
			v, c = store.SavePPReference(ctx, q.Scope.Profile, in, ppStamp(q.Op, in))
			return c
		case "pppoe.reference-delete":
			if !access.ValidCompanyID(in.Operation) || !access.ValidCompanyID(in.ReferenceID) {
				return Invalid
			}
			var c Code
			v, c = store.DeletePPReference(ctx, q.Scope.Profile, in, ppStamp(q.Op, in))
			return c
		case "pppoe.export-page":

			target, c := store.PPObservation(ctx, q.Scope.Profile, in.DeviceID, in.TargetID)
			if c != OK {
				return c
			}
			baseCount := 0
			if in.BaseID != "" {
				base, c := store.PPObservation(ctx, q.Scope.Profile, in.DeviceID, in.BaseID)
				if c != OK {
					return c
				}
				if !PPCompatible(base, target) {
					return ObservationIncompatible
				}
				baseCount = base.Count
			}
			if in.ExportPart == "diagnostics" {
				diagnosticStore, ok := s.store.(PDDiagnosticStore)
				if !ok || !PDValidScope(in.Scope) || !access.ValidCompanyID(in.DiagnosticID) {
					return Invalid
				}
				page, code := diagnosticStore.PPDiagnosticPage(ctx, q.Scope.Profile, in)
				if code != OK {
					return code
				}
				if !PDCompatibleExport(target, page.Diagnostic) {
					return ObservationIncompatible
				}
				v = page
				return OK
			}
			total := baseCount + target.Count
			if in.Offset > total {
				return CursorInvalid
			}
			page := in
			page.Query = &PPQuery{Sort: "row"}
			page.ObservationID = in.TargetID
			page.Offset = in.Offset - baseCount
			if in.Offset < baseCount {
				page.ObservationID = in.BaseID
				page.Offset = in.Offset
			}
			if in.Offset == total {
				v.Total = total
				return OK
			}
			var code Code
			v, code = store.PPPage(ctx, q.Scope.Profile, page)
			if code != OK {
				return code
			}
			v.Total = total
			v.NextOffset = in.Offset + len(v.Rows)
			v.More = v.NextOffset < total
			return OK
		case "pppoe.compare":
			if in.Query != nil && (in.Query.Search != "" || in.Query.Duplicates || in.Query.Sort != "" || in.Query.Desc) {
				return Invalid
			}
			a, c := s.ppLoad(ctx, store, q.Scope.Profile, in.DeviceID, in.BaseID)
			if c != OK {
				return c
			}
			b, c := s.ppLoad(ctx, store, q.Scope.Profile, in.DeviceID, in.TargetID)
			if c != OK {
				return c
			}
			ds, c := ComparePP(a, b)
			if c != OK {
				return c
			}
			v.Counts = map[string]int{}
			filtered := make([]PPDelta, 0, len(ds))
			for _, d := range ds {
				v.Counts[d.Category]++
				if in.Query == nil || in.Query.Category == "" || in.Query.Category == d.Category {
					filtered = append(filtered, d)
				}
			}
			v.Total = len(filtered)
			if in.Offset > v.Total {
				return CursorInvalid
			}
			end := in.Offset + PageSize
			if end > v.Total {
				end = v.Total
			}
			v.Deltas = filtered[in.Offset:end]
			v.More = end < v.Total
			v.NextOffset = end
			return OK
		default:
			return Invalid
		}
	})
	return v, c
}
func (s *Service) ppLoad(ctx context.Context, store PPStore, company, device, id string) (*PPObservation, Code) {
	o, c := store.PPObservation(ctx, company, device, id)
	if c != OK {
		return nil, c
	}
	in := PPInput{DeviceID: device, ObservationID: id, Query: &PPQuery{Sort: "row"}}
	for {
		v, c := store.PPPage(ctx, company, in)
		if c != OK {
			return nil, c
		}
		if len(o.Rows)+len(v.Rows) > PPMaxRows {
			return nil, Limit
		}
		o.Rows = append(o.Rows, v.Rows...)
		if !v.More {
			break
		}
		if v.NextOffset <= in.Offset {
			return nil, Invalid
		}
		in.Offset = v.NextOffset
	}
	if len(o.Rows) != o.Count {
		return nil, Unavailable
	}
	return o, OK
}
func (s *Service) startPP(ctx context.Context, q Command, in PPInput) (*Job, Code) {
	if !access.ValidCompanyID(in.Operation) || in.Revision == 0 {
		return nil, Invalid
	}
	store, ok := s.store.(PPStore)
	if !ok {
		return nil, Unavailable
	}
	diagnostic := q.Op == "pppoe.diagnostic-start"
	reserve := store.ReservePPCollection
	if diagnostic {
		if _, ok := s.remote.(PDDiagnosticRemote); !ok {
			return nil, Unavailable
		}
		diagnosticStore, ok := s.store.(PDDiagnosticStore)
		if !ok {
			return nil, Unavailable
		}
		reserve = diagnosticStore.ReservePPDiagnostic
	} else if _, ok := s.remote.(PPRemote); !ok {
		return nil, Unavailable
	}
	t, c := store.ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, true)
	if c != OK {
		return nil, c
	}
	d := t.Device
	if d.Revision != in.Revision {
		return nil, Revision
	}
	if !d.Active {
		return nil, Inactive
	}
	if d.Fingerprint == "" {
		return nil, TrustRequired
	}
	if !s.vault.Ready(t.Installation, t.Company, t.Wrapped) {
		return nil, VaultUnavailable
	}
	q.ID, q.Operation, q.Revision = in.DeviceID, in.Operation, in.Revision
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if s.closed {
		return nil, Unavailable
	}
	for _, j := range s.jobs {
		if sameScope(q, j.command) && j.command.Operation == q.Operation {
			if j.command.Op != q.Op || j.device.ID != d.ID || j.device.Revision != d.Revision || (diagnostic && ppStamp(q.Op, j.ppInput) != ppStamp(q.Op, in)) {
				return nil, Revision
			}
			return copyJob(j), OK
		}
	}
	// Check replay without committing an effect before checking admission. The store
	// rechecks the same receipt/quota atomically when it reserves a new job.
	in.SavedAt = PPTime(s.now())
	in.WorkerEpoch = s.epoch
	stamp := ppStamp(q.Op, in)
	// An empty job ID is a read-only preflight of the same ledger (not a second ledger).
	entity, known, c := reserve(ctx, q.Scope.Profile, in, stamp, "")
	if c != OK {
		return nil, c
	}
	if known {
		return &Job{ID: entity}, "JOB_INTERRUPTED"
	}
	active := 0
	for _, j := range s.jobs {
		if !j.released {
			active++
			if Endpoint(j.device) == Endpoint(d) {
				return nil, access.Busy
			}
		}
	}
	if active >= MaxConcurrent {
		return nil, access.Busy
	}
	if len(s.jobs) >= MaxJobs {
		return nil, "pp_jobs_full"
	}
	var ping *PPPingTarget
	if diagnostic && in.Scope == PDPing {
		ping, c = s.preparePPPing(ctx, q, in, t)
		if c != OK {
			return nil, c
		}
	}
	id, c := s.ids.ID()
	if c != OK {
		return nil, c
	}
	entity, known, c = reserve(ctx, q.Scope.Profile, in, stamp, id)
	if c != OK {
		return nil, c
	}
	if known {
		return &Job{ID: entity}, "JOB_INTERRUPTED"
	}
	jobctx, cancel := context.WithTimeout(context.Background(), QueryBudget)
	j := &runningJob{view: Job{ID: id, DeviceID: d.ID, Kind: q.Op, State: "RUNNING", Code: OK}, command: q, device: d, credentialRevision: t.Credential.Metadata.Revision, cancel: cancel, ppInput: in, ping: ping}
	s.jobs[id] = j
	s.wg.Add(1)
	go s.run(jobctx, j)
	return copyJob(j), OK
}
func (s *Service) runPP(ctx context.Context, j *runningJob) {
	defer s.wg.Done()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	defer j.cancel()
	defer s.watchPP(ctx, j)()
	user, secret, c := s.check(ctx, j, true)
	var o *PPObservation
	started := s.now()
	startMono := time.Since(s.origin).Nanoseconds()
	if c == OK {
		o, c = s.remote.(PPRemote).CollectPPPoE(ctx, j.device, user, secret, func() Code { _, _, c := s.check(ctx, j, false); return c })
	}
	secret = ""
	a := PPAttempt{Epoch: s.epoch, JobID: j.view.ID, At: PPTime(s.now()), Code: c, State: "FAILED"}
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
			c = "DEADLINE_EXCEEDED"
		}
	}
	if c == OK && o != nil && o.Quality.Status == "complete" {
		o.ID, c = s.ids.ID()
		o.DeviceID = j.device.ID
		o.Scope = PPScope
		o.Provenance.Epoch = s.epoch
		o.Provenance.Started = PPTime(started)
		o.Provenance.Ended = PPTime(s.now())
		o.Provenance.Sample = PPInterval{Min: startMono, Max: time.Since(s.origin).Nanoseconds(), Known: true}
		o.Provenance.Connection = PPConnection(j.device)
		o.Provenance.Revision = j.device.Revision
		o.Provenance.CredentialRevision = j.credentialRevision
		for i := range o.Rows {
			if c != OK {
				break
			}
			o.Rows[i].ID, c = s.ids.ID()
			o.Rows[i].Sample = o.Provenance.Sample
		}

	} else {
		o = nil
	}
	if c != OK {
		o = nil
	}
	a.Code = c
	if c == OK {
		a.State = "SUCCEEDED"
	}
	if c == Cancelled {
		a.State = "CANCELLED"
	}
	if o != nil {
		a.ObservationID = o.ID
	}
	persistCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	persisted := s.authorized(persistCtx, j.command, func(uint64) Code {
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() != nil || j.view.State == "CANCELLED" {
			o = nil
			a.Code = Cancelled
			a.State = "CANCELLED"
			a.ObservationID = ""
		}
		store := s.store.(PPStore)
		t, code := store.ReadPPTarget(persistCtx, j.command.Scope.Profile, j.device.ID, false)
		if code != OK {
			return code
		}
		if t.Device.Revision != j.device.Revision || t.Credential.Metadata.Revision != j.credentialRevision {
			return Revision
		}
		if !s.vault.Ready(t.Installation, t.Company, t.Wrapped) {
			a.Code = VaultUnavailable
			a.State = "FAILED"
			a.ObservationID = ""
			a.Quality.Status = "failed"
			a.Quality.Code = VaultUnavailable
			return store.FinishPPCollection(persistCtx, j.command.Scope.Profile, t, j.view.ID, a, nil)
		}
		if o != nil {
			o.Provenance.Installation = t.Installation
			o.Provenance.Company = t.Company
		}
		code = store.FinishPPCollection(persistCtx, j.command.Scope.Profile, t, j.view.ID, a, o)
		if code == Limit && o != nil {
			a.Code = Limit
			a.State = "FAILED"
			a.ObservationID = ""
			a.Quality.Status = "limited"
			a.Quality.Code = Limit
			a.Quality.Reason = "storage"
			return store.FinishPPCollection(persistCtx, j.command.Scope.Profile, t, j.view.ID, a, nil)
		}
		return code
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	if persisted != OK {
		a.Code = persisted
		a.State = "FAILED"
		a.ObservationID = ""
		if persisted == Unavailable {
			a.Code = "PERSISTENCE_FAILED"
		}
	}
	j.view.Code = a.Code
	j.view.State = a.State
	j.view.PPPoE = &a
	j.finished = s.now()
}

func (s *Service) watchPP(ctx context.Context, j *runningJob) func() {
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
				if _, _, c := s.check(ctx, j, false); c != OK {
					j.cancel()
					return
				}
			}
		}
	}()
	return func() { close(watcherDone); <-watcherExit }
}
