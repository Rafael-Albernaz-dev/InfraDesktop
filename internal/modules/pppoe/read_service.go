package pppoe

import (
	"context"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/inventory"
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
		return s.cancelPPReadByOperation(q, in, v)
	}
	if !access.ValidCompanyID(in.JobID) {
		return Invalid
	}
	if q.Op == "pppoe.read-stop" {
		if c := s.markTrafficRetained(q, in.JobID); c != OK {
			return c
		}
	}
	return s.readJobPageOrCancel(ctx, q, in, v)
}

func (s *Service) cancelPPReadByOperation(q Command, in PPInput, v *PPView) Code {
	if !access.ValidCompanyID(in.DeviceID) || !access.ValidCompanyID(in.Operation) {
		return Invalid
	}
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	for _, j := range s.inv.Jobs {
		if j.Command.Op == "pppoe.read-start" &&
			inventory.SameScope(q, j.Command) &&
			j.Device.ID == in.DeviceID &&
			j.Command.Operation == in.Operation {
			j.Read = nil
			if j.Finished.IsZero() {
				j.Cancel()
				j.View.State = "CANCELLED"
				j.View.Code = Cancelled
				v.Job = inventory.CopyJob(j)
			}
		}
	}
	return OK
}

func (s *Service) markTrafficRetained(q Command, jobID string) Code {
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	j := s.inv.Jobs[jobID]
	if j == nil || !inventory.SameScope(q, j.Command) || j.PPInput.Scope != "traffic" {
		return Invalid
	}
	j.RetainTraffic = true
	return OK
}

func (s *Service) readJobPageOrCancel(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	q.JobID = in.JobID
	op := q.Op
	q.Op = "device.query-status"
	if op == "pppoe.read-cancel" || op == "pppoe.read-stop" {
		q.Op = "device.query-cancel"
	}
	job, c := s.inv.StatusKind(ctx, q, "pppoe.read-start")
	if c != OK {
		return c
	}
	v.Job = job
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	j := s.inv.Jobs[in.JobID]
	if j == nil || !inventory.SameScope(q, j.Command) || j.Command.Authority != q.Authority {
		return Changed
	}
	if op == "pppoe.read-cancel" {
		j.Read = nil
		return OK
	}
	var user, secret string
	if c := s.inv.CheckPP(ctx, j, false, &user, &secret); c != OK {
		j.Read = nil
		j.Cancel()
		return c
	}
	return paginateReadResult(j, in.Offset, v)
}

func paginateReadResult(j *RunningJob, offset int, v *PPView) Code {
	if j.Read == nil || (j.View.State == "CANCELLED" && !j.RetainTraffic) {
		return OK
	}
	source := j.Read
	v.Total = len(source.Rows)
	if offset > v.Total {
		return CursorInvalid
	}
	page := *source
	end := min(offset+PageSize, v.Total)
	page.Rows = append([]PPReadRow{}, source.Rows[offset:end]...)
	v.Read = &page
	v.NextOffset = end
	v.More = end < v.Total
	return OK
}

func (s *Service) preparePPRead(
	ctx context.Context, q Command, in PPInput, t PPTarget,
) (*PPReadTarget, Code) {
	if in.Scope == "top" || in.Scope == "health" || in.Scope == "operations" {
		if in.ObservationID != "" || len(in.RowIDs) != 0 {
			return nil, Invalid
		}
		return &PPReadTarget{Prepared: time.Now()}, OK
	}
	store := s.inv.Store.(PPStore)
	o, prepared, now, c := s.loadFreshReadObservation(ctx, store, q.Scope.Profile, in, t)
	if c != OK {
		return nil, c
	}
	p := o.Provenance
	target := &PPReadTarget{
		Observation: *o,
		Prepared:    prepared,
		Age:         PPInterval{Known: true, Min: now - p.Sample.Max, Max: now - p.Sample.Min},
	}
	if c := s.loadReadTargetRows(ctx, store, q.Scope.Profile, in, target); c != OK {
		return nil, c
	}
	return target, OK
}

func (s *Service) loadFreshReadObservation(
	ctx context.Context, store PPStore, company string, in PPInput, t PPTarget,
) (*PPObservation, time.Time, int64, Code) {
	current, c := store.PPCurrent(ctx, company, in.DeviceID)
	if c != OK {
		return nil, time.Time{}, 0, c
	}
	o := current.Current
	if o == nil || o.ID != in.ObservationID {
		return nil, time.Time{}, 0, ObservationMissing
	}
	p := o.Provenance
	prepared := time.Now()
	now := prepared.Sub(s.inv.Origin).Nanoseconds()
	if !s.validReadProvenance(p, t, now) {
		return nil, time.Time{}, 0, ObservationIncompatible
	}
	if !validReadRowIDList(in.Scope, in.RowIDs) {
		return nil, time.Time{}, 0, Invalid
	}
	return o, prepared, now, OK
}

func (s *Service) validReadProvenance(p PPProvenance, t PPTarget, now int64) bool {
	return p.Sample.Min >= 0 &&
		p.Sample.Max >= p.Sample.Min &&
		p.Epoch == s.inv.Epoch &&
		p.Connection == PPConnection(t.Device) &&
		p.Revision == t.Device.Revision &&
		p.CredentialRevision == t.Credential.Metadata.Revision &&
		p.Sample.Known &&
		now >= p.Sample.Max
}

func validReadRowIDList(scope string, rowIDs []string) bool {
	if len(rowIDs) == 0 || len(rowIDs) > PageSize {
		return false
	}
	if (scope == "log" || scope == "secret") && len(rowIDs) != 1 {
		return false
	}
	return true
}

func (s *Service) loadReadTargetRows(
	ctx context.Context, store PPStore, company string, in PPInput, target *PPReadTarget,
) Code {
	seen := map[string]bool{}
	for _, id := range in.RowIDs {
		if !access.ValidCompanyID(id) || seen[id] {
			return Invalid
		}
		seen[id] = true
		rowInput := in
		rowInput.RowID = id
		detail, c := store.PPSession(ctx, company, rowInput)
		if c != OK {
			return c
		}
		if len(detail.Rows) != 1 {
			return ObservationMissing
		}
		row := detail.Rows[0]
		if !compatibleReadRow(in.Scope, row) {
			return ObservationIncompatible
		}
		target.Rows = append(target.Rows, row)
	}
	return OK
}

func compatibleReadRow(scope string, row PPRow) bool {
	if scope == "log" || scope == "secret" {
		return row.Login != ""
	}
	return row.State != "OFFLINE" && row.SessionID != "" && row.RemoteID != "" && row.Uptime.Known
}

func (s *Service) startPPRead(ctx context.Context, q Command, in PPInput) (*Job, Code) {
	target, c := s.admitStartPPRead(ctx, q, in)
	if c != OK {
		return nil, c
	}
	d := target.Device
	q.ID, q.Operation, q.Revision = in.DeviceID, in.Operation, in.Revision
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	s.inv.PruneLocked()
	if s.inv.Closed {
		return nil, Unavailable
	}
	if existing, code := s.findOrAdmitPPReadJobLocked(q, in, d); existing != nil || code != OK {
		return existing, code
	}
	reading, c := s.preparePPRead(ctx, q, in, target)
	if c != OK {
		return nil, c
	}
	id, c := s.inv.IDs.ID()
	if c != OK {
		return nil, c
	}
	jobctx, cancel := context.WithTimeout(context.Background(), PPReadBudget(in.Scope))
	j := &RunningJob{
		View:               Job{ID: id, DeviceID: d.ID, Kind: q.Op, State: "RUNNING", Code: OK},
		Command:            q,
		Device:             d,
		CredentialRevision: target.Credential.Metadata.Revision,
		Cancel:             cancel,
		PPInput:            in,
		ReadTarget:         reading,
	}
	s.inv.Jobs[id] = j
	s.inv.WG.Add(1)
	go s.runPPRead(jobctx, j)
	return inventory.CopyJob(j), OK
}

func (s *Service) admitStartPPRead(ctx context.Context, q Command, in PPInput) (PPTarget, Code) {
	if !access.ValidCompanyID(in.DeviceID) ||
		!access.ValidCompanyID(in.Operation) ||
		in.Revision == 0 ||
		!PPReadScope(in.Scope) {
		return PPTarget{}, Invalid
	}
	if _, ok := s.inv.Remote.(PPReadRemote); !ok {
		return PPTarget{}, Unavailable
	}
	store, ok := s.inv.Store.(PPStore)
	if !ok {
		return PPTarget{}, Unavailable
	}
	target, c := store.ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, true)
	if c != OK {
		return PPTarget{}, c
	}
	if c := s.validatePPTargetDevice(target, in.Revision); c != OK {
		return PPTarget{}, c
	}
	return target, OK
}

func (s *Service) findOrAdmitPPReadJobLocked(q Command, in PPInput, d Device) (*Job, Code) {
	active := 0
	for _, j := range s.inv.Jobs {
		if j.View.State == "RUNNING" &&
			inventory.SameScope(q, j.Command) &&
			j.Command.Operation == in.Operation {
			if !matchingPPReadIdempotency(j, q, in, d) {
				return nil, Revision
			}
			return inventory.CopyJob(j), OK
		}
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
		return nil, Limit
	}
	return nil, OK
}

func matchingPPReadIdempotency(j *RunningJob, q Command, in PPInput, d Device) bool {
	return j.Command.Op == q.Op &&
		j.Device.ID == d.ID &&
		j.Device.Revision == d.Revision &&
		j.PPInput.Scope == in.Scope &&
		j.PPInput.ObservationID == in.ObservationID &&
		strings.Join(j.PPInput.RowIDs, ",") == strings.Join(in.RowIDs, ",")
}

func (s *Service) publishPPRead(ctx context.Context, j *RunningJob, result *PPReadResult) Code {
	if !PPReadWithinBudget(result) {
		return Limit
	}
	if _, _, c := s.inv.Check(ctx, j, false); c != OK {
		return c
	}
	return s.inv.Authorized(ctx, j.Command, func(uint64) Code {
		if j.PPInput.Scope == "operations" {
			store, ok := s.inv.Store.(DeviceMonitoringStore)
			if !ok {
				return Unavailable
			}
			c := store.SaveDeviceMonitoring(
				ctx, j.Command.Scope.Profile, j.Device, j.CredentialRevision, result,
			)
			if c != OK {
				return c
			}
		}
		s.inv.Mu.Lock()
		defer s.inv.Mu.Unlock()
		if ctx.Err() != nil || j.View.State != "RUNNING" {
			return Cancelled
		}
		copyRes := *result
		copyRes.Rows = append([]PPReadRow(nil), result.Rows...)
		j.Read = &copyRes
		return OK
	})
}

func (s *Service) runPPRead(ctx context.Context, j *RunningJob) {
	defer s.inv.WG.Done()
	defer func() { s.inv.Mu.Lock(); j.Released = true; s.inv.Mu.Unlock() }()
	defer j.Cancel()
	defer s.inv.Watch(ctx, j)()
	user, secret, c := s.inv.Check(ctx, j, true)
	var result *PPReadResult
	if c == OK {
		pubFn := func(v *PPReadResult) Code { return s.publishPPRead(ctx, j, v) }
		chkFn := func() Code { _, _, code := s.inv.Check(ctx, j, false); return code }
		req := PPReadRequest{
			Device: j.Device, User: user, Secret: secret, Scope: j.PPInput.Scope,
			Target: j.ReadTarget, Publish: pubFn, Check: chkFn,
		}
		result, c = s.inv.Remote.(PPReadRemote).ReadPP(ctx, req)
		req.Secret = ""
	}
	secret = ""
	if c == OK && result == nil {
		c = Partial
	}
	if c == OK && result != nil {
		c = s.publishPPRead(ctx, j, result)
	}
	s.finishPPReadJob(ctx, j, result, c)
}

func (s *Service) finishPPReadJob(
	ctx context.Context, j *RunningJob, result *PPReadResult, c Code,
) {
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	if ctx.Err() != nil {
		c = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			c = CodeDeadlineExceeded
		}
	}
	if j.View.State == "CANCELLED" {
		c = Cancelled
	}
	j.View.State = "SUCCEEDED"
	if c != OK {
		j.View.State = "FAILED"
		if !retainTrafficOnError(j, c) {
			j.Read = nil
		}
	}
	if c == Cancelled {
		j.View.State = "CANCELLED"
	}
	if result != nil && ValidCollectorReason(c, result.FailureReason) {
		j.View.PPPoE = &PPAttempt{
			Epoch: s.inv.Epoch, JobID: j.View.ID, State: j.View.State, Code: c,
			At: PPTime(s.inv.Now()), Operation: j.Command.Operation, Scope: j.PPInput.Scope,
			Quality: PPQuality{Status: "partial", Code: c, Reason: result.FailureReason},
		}
	}
	j.View.Code = c
	j.Finished = s.inv.Now()
}

func retainTrafficOnError(j *RunningJob, c Code) bool {
	if j.PPInput.Scope != "traffic" {
		return false
	}
	if c == Cancelled && j.RetainTraffic {
		return true
	}
	switch c {
	case Partial, CodeDeadlineExceeded, "SSH_CONNECTION_LOST",
		"SSH_NEGOTIATION_FAILED", "CONNECT_FAILED", Limit, "REMOTE_PERMISSION_DENIED":
		return true
	default:
		return false
	}
}
