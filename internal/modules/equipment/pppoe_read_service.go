package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
	"strings"
	"time"
)

func (s *Service) handlePPRead(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	if q.Op == "pppoe.read-start" {
		var c Code
		v.Job, c = s.startPPRead(ctx, q, in)
		return c
	}
	if q.Op == "pppoe.read-cancel" && in.JobID == "" {
		if !access.ValidCompanyID(in.DeviceID) || !access.ValidCompanyID(in.Operation) {
			return Invalid
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, j := range s.jobs {
			if j.command.Op == "pppoe.read-start" && sameScope(q, j.command) && j.device.ID == in.DeviceID && j.command.Operation == in.Operation {
				j.read = nil
				if j.finished.IsZero() {
					j.cancel()
					j.view.State = "CANCELLED"
					j.view.Code = Cancelled
					v.Job = copyJob(j)
				}
			}
		}
		return OK
	}
	if !access.ValidCompanyID(in.JobID) {
		return Invalid
	}
	if q.Op == "pppoe.read-stop" {
		s.mu.Lock()
		j := s.jobs[in.JobID]
		if j == nil || !sameScope(q, j.command) || j.ppInput.Scope != "traffic" {
			s.mu.Unlock()
			return Invalid
		}
		j.retainTraffic = true
		s.mu.Unlock()
	}
	q.JobID = in.JobID
	op := q.Op
	q.Op = "device.query-status"
	if op == "pppoe.read-cancel" || op == "pppoe.read-stop" {
		q.Op = "device.query-cancel"
	}
	job, c := s.statusKind(ctx, q, "pppoe.read-start")
	if c != OK {
		return c
	}
	v.Job = job
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[in.JobID]
	if j == nil || !sameScope(q, j.command) || j.command.Authority != q.Authority {
		return Changed
	}
	if op == "pppoe.read-cancel" {
		j.read = nil
		return OK
	}
	var user, secret string
	if c := s.checkPP(ctx, j, false, &user, &secret); c != OK {
		j.read = nil
		j.cancel()
		return c
	}
	if j.read == nil || (j.view.State == "CANCELLED" && !j.retainTraffic) {
		return OK
	}
	source := j.read
	v.Total = len(source.Rows)
	if in.Offset > v.Total {
		return CursorInvalid
	}
	page := *source
	end := min(in.Offset+PageSize, v.Total)
	page.Rows = append([]PPReadRow{}, source.Rows[in.Offset:end]...)
	v.Read = &page
	v.NextOffset = end
	v.More = end < v.Total
	return OK
}

func (s *Service) preparePPRead(ctx context.Context, q Command, in PPInput, t PPTarget) (*PPReadTarget, Code) {
	if in.Scope == "top" || in.Scope == "health" || in.Scope == "operations" {
		if in.ObservationID != "" || len(in.RowIDs) != 0 {
			return nil, Invalid
		}
		return &PPReadTarget{Prepared: time.Now()}, OK
	}
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
	prepared := time.Now()
	now := prepared.Sub(s.origin).Nanoseconds()
	if p.Sample.Min < 0 || p.Sample.Max < p.Sample.Min || p.Epoch != s.epoch || p.Connection != PPConnection(t.Device) || p.Revision != t.Device.Revision || p.CredentialRevision != t.Credential.Metadata.Revision || !p.Sample.Known || now < p.Sample.Max {
		return nil, ObservationIncompatible
	}
	if len(in.RowIDs) == 0 || len(in.RowIDs) > PageSize || ((in.Scope == "log" || in.Scope == "secret") && len(in.RowIDs) != 1) {
		return nil, Invalid
	}
	target := &PPReadTarget{Observation: *o, Prepared: prepared, Age: PPInterval{Known: true, Min: now - p.Sample.Max, Max: now - p.Sample.Min}}
	seen := map[string]bool{}
	for _, id := range in.RowIDs {
		if !access.ValidCompanyID(id) || seen[id] {
			return nil, Invalid
		}
		seen[id] = true
		rowInput := in
		rowInput.RowID = id
		detail, c := store.PPSession(ctx, q.Scope.Profile, rowInput)
		if c != OK {
			return nil, c
		}
		if len(detail.Rows) != 1 {
			return nil, ObservationMissing
		}
		row := detail.Rows[0]
		if in.Scope == "log" || in.Scope == "secret" {
			if row.Login == "" {
				return nil, ObservationIncompatible
			}
		} else if row.State == "OFFLINE" || row.SessionID == "" || row.RemoteID == "" || !row.Uptime.Known {
			return nil, ObservationIncompatible
		}
		target.Rows = append(target.Rows, row)
	}
	return target, OK
}

func (s *Service) startPPRead(ctx context.Context, q Command, in PPInput) (*Job, Code) {
	if !access.ValidCompanyID(in.DeviceID) || !access.ValidCompanyID(in.Operation) || in.Revision == 0 || !PPReadScope(in.Scope) {
		return nil, Invalid
	}
	remote, ok := s.remote.(PPReadRemote)
	if !ok {
		return nil, Unavailable
	}
	_ = remote
	store, ok := s.store.(PPStore)
	if !ok {
		return nil, Unavailable
	}
	target, c := store.ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, true)
	if c != OK {
		return nil, c
	}
	d := target.Device
	if d.Revision != in.Revision {
		return nil, Revision
	}
	if !d.Active {
		return nil, Inactive
	}
	if d.Fingerprint == "" {
		return nil, TrustRequired
	}
	if !s.vault.Ready(target.Installation, target.Company, target.Wrapped) {
		return nil, VaultUnavailable
	}
	q.ID, q.Operation, q.Revision = in.DeviceID, in.Operation, in.Revision
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prune()
	if s.closed {
		return nil, Unavailable
	}
	active := 0
	for _, j := range s.jobs {
		if j.view.State == "RUNNING" && sameScope(q, j.command) && j.command.Operation == in.Operation {
			if j.command.Op != q.Op || j.device.ID != d.ID || j.device.Revision != d.Revision || j.ppInput.Scope != in.Scope || j.ppInput.ObservationID != in.ObservationID || strings.Join(j.ppInput.RowIDs, ",") != strings.Join(in.RowIDs, ",") {
				return nil, Revision
			}
			return copyJob(j), OK
		}
		if !j.released {
			active++
			if jobUsesEndpoint(j, d) {
				return nil, access.Busy
			}
		}
	}
	if active >= MaxConcurrent {
		return nil, access.Busy
	}
	if len(s.jobs) >= MaxJobs {
		return nil, Limit
	}
	reading, c := s.preparePPRead(ctx, q, in, target)
	if c != OK {
		return nil, c
	}
	id, c := s.ids.ID()
	if c != OK {
		return nil, c
	}
	jobctx, cancel := context.WithTimeout(context.Background(), PPReadBudget(in.Scope))
	j := &runningJob{view: Job{ID: id, DeviceID: d.ID, Kind: q.Op, State: "RUNNING", Code: OK}, command: q, device: d, credentialRevision: target.Credential.Metadata.Revision, cancel: cancel, ppInput: in, readTarget: reading}
	s.jobs[id] = j
	s.wg.Add(1)
	go s.runPPRead(jobctx, j)
	return copyJob(j), OK
}

func (s *Service) publishPPRead(ctx context.Context, j *runningJob, result *PPReadResult) Code {
	if !PPReadWithinBudget(result) {
		return Limit
	}
	_, _, c := s.check(ctx, j, false)
	if c != OK {
		return c
	}
	return s.authorized(ctx, j.command, func(uint64) Code {
		if j.ppInput.Scope == "operations" {
			store, ok := s.store.(DeviceMonitoringStore)
			if !ok {
				return Unavailable
			}
			if c := store.SaveDeviceMonitoring(ctx, j.command.Scope.Profile, j.device, j.credentialRevision, result); c != OK {
				return c
			}
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		if ctx.Err() != nil || j.view.State != "RUNNING" {
			return Cancelled
		}
		copy := *result
		copy.Rows = append([]PPReadRow(nil), result.Rows...)
		j.read = &copy
		return OK
	})
}
func (s *Service) runPPRead(ctx context.Context, j *runningJob) {
	defer s.wg.Done()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	defer j.cancel()
	defer s.watchPP(ctx, j)()
	user, secret, c := s.check(ctx, j, true)
	var result *PPReadResult
	if c == OK {
		result, c = s.remote.(PPReadRemote).ReadPP(ctx, j.device, user, secret, j.ppInput.Scope, j.readTarget, func(v *PPReadResult) Code { return s.publishPPRead(ctx, j, v) }, func() Code { _, _, c := s.check(ctx, j, false); return c })
	}
	secret = ""
	if c == OK && result == nil {
		c = Partial
	}
	if c == OK && result != nil {
		c = s.publishPPRead(ctx, j, result)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		c = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			c = "DEADLINE_EXCEEDED"
		}
	}
	if j.view.State == "CANCELLED" {
		c = Cancelled
	}
	j.view.State = "SUCCEEDED"
	if c != OK {
		j.view.State = "FAILED"
		if !(j.ppInput.Scope == "traffic" && ((c == Cancelled && j.retainTraffic) || c == Partial || c == "DEADLINE_EXCEEDED" || c == "SSH_CONNECTION_LOST" || c == "SSH_NEGOTIATION_FAILED" || c == "CONNECT_FAILED" || c == Limit || c == "REMOTE_PERMISSION_DENIED")) {
			j.read = nil
		}
	}
	if c == Cancelled {
		j.view.State = "CANCELLED"
	}
	if result != nil && ValidCollectorReason(c, result.FailureReason) {
		j.view.PPPoE = &PPAttempt{Epoch: s.epoch, JobID: j.view.ID, State: j.view.State, Code: c, At: PPTime(s.now()), Operation: j.command.Operation, Scope: j.ppInput.Scope, Quality: PPQuality{Status: "partial", Code: c, Reason: result.FailureReason}}
	}
	j.view.Code = c
	j.finished = s.now()
}
