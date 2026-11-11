package pppoe

import (
	"context"
	"infradesktop/internal/modules/access"
	"sort"
	"strconv"
	"strings"
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
		v.Job, c = s.inv.StatusKind(ctx, q, "pppoe.diagnostic-start")
		return c
	}
	if !access.ValidCompanyID(in.DeviceID) || !PDValidScope(in.Scope) {
		return Invalid
	}
	store, ok := s.inv.Store.(PDDiagnosticStore)
	if !ok {
		return Unavailable
	}
	return s.dispatchStorePDOp(ctx, store, q, in, v)
}

func (s *Service) dispatchStorePDOp(
	ctx context.Context, store PDDiagnosticStore, q Command, in PPInput, v *PPView,
) Code {
	switch q.Op {
	case "pppoe.diagnostic-start":
		if !validDiagnosticStartScope(in) {
			return Invalid
		}
		return s.handleCollectStart(ctx, q, in, v)
	case "pppoe.diagnostic-current":
		result, c := store.PPDiagnosticCurrent(ctx, q.Scope.Profile, in)
		*v = result
		s.reconcileRunningAttempt(v)
		return c
	case "pppoe.diagnostic-list":
		if !access.ValidCompanyID(in.DiagnosticID) {
			return Invalid
		}
		result, c := store.PPDiagnosticPage(ctx, q.Scope.Profile, in)
		*v = result
		return c
	default:
		return Invalid
	}
}

func validDiagnosticStartScope(in PPInput) bool {
	if in.Scope == PDPing {
		return access.ValidCompanyID(in.ObservationID) && access.ValidCompanyID(in.RowID)
	}
	return in.ObservationID == "" && in.RowID == ""
}

func (s *Service) reconcileRunningAttempt(v *PPView) {
	if v.Attempt == nil || v.Attempt.State != "RUNNING" {
		return
	}
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	j := s.inv.Jobs[v.Attempt.JobID]
	if j != nil && j.View.PPPoE != nil {
		a := *j.View.PPPoE
		v.Attempt = &a
	} else if j == nil || v.Attempt.Epoch != s.inv.Epoch {
		v.Attempt.State = "INTERRUPTED"
		v.Attempt.Code = CodeJobInterrupted
		v.Attempt.Quality.Status = "failed"
		v.Attempt.Quality.Code = CodeJobInterrupted
	}
}

func (s *Service) preparePPPing(
	ctx context.Context, q Command, in PPInput, target PPTarget,
) (*PPPingTarget, Code) {
	store := s.inv.Store.(PPStore)
	current, c := store.PPCurrent(ctx, q.Scope.Profile, in.DeviceID)
	if c != OK {
		return nil, c
	}
	o := current.Current
	if o == nil || o.ID != in.ObservationID {
		return nil, ObservationMissing
	}
	p := o.Provenance
	now := time.Since(s.inv.Origin).Nanoseconds()
	if !s.validReadProvenance(p, target, now) || !p.RouterUptime.Known {
		return nil, CodePingReadingStale
	}
	detail, c := store.PPSession(ctx, q.Scope.Profile, in)
	if c != OK {
		return nil, c
	}
	if len(detail.Rows) != 1 {
		return nil, ObservationMissing
	}
	row := detail.Rows[0]
	if row.State == "OFFLINE" {
		return nil, CodePingOffline
	}
	address, ok := PPPingAddress(row)
	if !ok || !row.Uptime.Known || row.RemoteID == "" || row.SessionID == "" {
		return nil, ObservationIncompatible
	}
	row.Address = address
	age := PPInterval{Min: now - p.Sample.Max, Max: now - p.Sample.Min, Known: true}
	return &PPPingTarget{Observation: *o, Row: row, Age: age, Prepared: time.Now()}, OK
}

func (s *Service) runPD(ctx context.Context, j *RunningJob) {
	defer s.inv.WG.Done()
	defer func() { s.inv.Mu.Lock(); j.Released = true; s.inv.Mu.Unlock() }()
	defer j.Cancel()
	defer s.inv.Watch(ctx, j)()
	o, attempt := s.executePDCollect(ctx, j)
	s.persistPDCollect(ctx, j, o, &attempt)
}

func (s *Service) executePDCollect(ctx context.Context, j *RunningJob) (*PDDiagnostic, PPAttempt) {
	user, secret, c := s.inv.Check(ctx, j, true)
	started := s.inv.Now()
	startMono := time.Since(s.inv.Origin).Nanoseconds()
	var o *PDDiagnostic
	if c == OK {
		chkFn := func() Code { _, _, code := s.inv.Check(ctx, j, false); return code }
		req := PDDiagnosticRequest{
			Device: j.Device, User: user, Secret: secret, Scope: j.PPInput.Scope, Ping: j.Ping, Check: chkFn,
		}
		o, c = s.inv.Remote.(PDDiagnosticRemote).CollectPPDiagnostic(ctx, req)
		req.Secret = ""
	}
	secret = ""
	attempt := PPAttempt{
		Epoch: s.inv.Epoch, JobID: j.View.ID, At: PPTime(s.inv.Now()), Code: c,
		State: "FAILED", Operation: j.PPInput.Operation, Scope: j.PPInput.Scope,
		SourceObservation: j.PPInput.ObservationID, SourceRow: j.PPInput.RowID,
	}
	o, c = s.evaluatePDOutcome(ctx, j, o, c, started, startMono, &attempt)
	attempt.Code = c
	if c == OK {
		attempt.State = "SUCCEEDED"
	} else if c == Cancelled {
		attempt.State = "CANCELLED"
	}
	if o != nil {
		attempt.ObservationID = o.ID
	}
	return o, attempt
}

func (s *Service) evaluatePDOutcome(
	ctx context.Context, j *RunningJob, o *PDDiagnostic, c Code,
	started time.Time, startMono int64, attempt *PPAttempt,
) (*PDDiagnostic, Code) {
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
			c = CodeDeadlineExceeded
		}
	}
	if c == OK && o != nil {
		c = s.stampDiagnosticProvenance(j, o, started, startMono)
	}
	if c != OK {
		o = nil
		applyFailedPDQuality(attempt, c)
	}
	return o, c
}

func (s *Service) stampDiagnosticProvenance(
	j *RunningJob, o *PDDiagnostic, started time.Time, startMono int64,
) Code {
	var c Code
	o.ID, c = s.inv.IDs.ID()
	o.DeviceID = j.Device.ID
	o.Scope = j.PPInput.Scope
	p := &o.Provenance
	p.Epoch = s.inv.Epoch
	p.Started = PPTime(started)
	p.Ended = PPTime(s.inv.Now())
	p.Sample = PPInterval{
		Min: startMono, Max: time.Since(s.inv.Origin).Nanoseconds(), Known: true,
	}
	p.Connection = PPConnection(j.Device)
	p.Revision = j.Device.Revision
	p.CredentialRevision = j.CredentialRevision
	if j.Ping != nil {
		o.SourceObservation = j.Ping.Observation.ID
		o.SourceRow = j.Ping.Row.ID
	}
	for i := range o.Rows {
		if c != OK {
			break
		}
		o.Rows[i].ID, c = s.inv.IDs.ID()
	}
	ResolvePDRelations(o.Rows)
	return c
}

func applyFailedPDQuality(attempt *PPAttempt, c Code) {
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

func (s *Service) persistPDCollect(
	ctx context.Context, j *RunningJob, o *PDDiagnostic, attempt *PPAttempt,
) {
	persist, cancel := context.WithTimeout(context.Background(), PPPersistenceBudget)
	defer cancel()
	persisted := s.inv.Authorized(persist, j.Command, func(uint64) Code {
		return s.commitPDDiagnostic(persist, ctx, j, o, attempt)
	})
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	if persisted != OK {
		attempt.Code = persisted
		attempt.State = "FAILED"
		attempt.ObservationID = ""
		if persisted == Unavailable {
			attempt.Code = CodePersistenceFailed
		}
	}
	copyAttempt := *attempt
	j.View.Code = attempt.Code
	j.View.State = attempt.State
	j.View.PPPoE = &copyAttempt
	j.Finished = s.inv.Now()
}

func (s *Service) commitPDDiagnostic(
	persist, jobCtx context.Context, j *RunningJob, o *PDDiagnostic, attempt *PPAttempt,
) Code {
	s.inv.Mu.Lock()
	defer s.inv.Mu.Unlock()
	if jobCtx.Err() != nil || j.View.State == "CANCELLED" {
		o = nil
		attempt.Code = Cancelled
		attempt.State = "CANCELLED"
		attempt.ObservationID = ""
	}
	target, code := s.inv.Store.(PPStore).ReadPPTarget(
		persist, j.Command.Scope.Profile, j.Device.ID, false,
	)
	if code != OK {
		return code
	}
	if target.Device.Revision != j.Device.Revision ||
		target.Credential.Metadata.Revision != j.CredentialRevision {
		return Revision
	}
	if !s.inv.Vault.Ready(target.Installation, target.Company, target.Wrapped) {
		o = nil
		attempt.Code = VaultUnavailable
		attempt.State = "FAILED"
		attempt.ObservationID = ""
	}
	if o != nil {
		o.Provenance.Installation = target.Installation
		o.Provenance.Company = target.Company
	}
	store := s.inv.Store.(PDDiagnosticStore)
	s.decoratePingLocked(j, o)
	code = store.FinishPPDiagnostic(
		persist, j.Command.Scope.Profile, target, j.PPInput, j.View.ID, *attempt, o,
	)
	if code == OK {
		s.rememberPingLocked(j, o)
	}
	if code == Limit && o != nil {
		attempt.Code = Limit
		attempt.State = "FAILED"
		attempt.ObservationID = ""
		attempt.Quality.Status = "limited"
		attempt.Quality.Code = Limit
		attempt.Quality.Reason = "storage"
		return store.FinishPPDiagnostic(
			persist, j.Command.Scope.Profile, target, j.PPInput, j.View.ID, *attempt, nil,
		)
	}
	return code
}

func PingJitter(p *PDPingResult) *int64 {
	if p == nil || p.Sent != 3 || len(p.Probes) != 3 {
		return nil
	}
	var total uint64
	var pairs int64
	for i := 1; i < len(p.Probes); i++ {
		a, b := p.Probes[i-1], p.Probes[i]
		if a.RTT == nil || b.RTT == nil || *a.RTT < 0 || *b.RTT < 0 || b.Sequence != a.Sequence+1 {
			continue
		}
		if *a.RTT > *b.RTT {
			total += uint64(*a.RTT - *b.RTT)
		} else {
			total += uint64(*b.RTT - *a.RTT)
		}
		pairs++
	}
	if pairs == 0 {
		return nil
	}
	v := int64(total / uint64(pairs))
	return &v
}

func PingMedianTen(history []int64) *int64 {
	if len(history) < 10 {
		return nil
	}
	v := append([]int64(nil), history[len(history)-10:]...)
	for _, n := range v {
		if n < 0 {
			return nil
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i] < v[j] })
	m := v[4]/2 + v[5]/2 + (v[4]%2+v[5]%2)/2
	return &m
}

func (s *Service) pingStatsKey(j *RunningJob) string {
	if j.Ping == nil {
		return ""
	}
	parts := []string{
		j.Command.Scope.Profile,
		j.Command.Scope.Activation,
		strconv.FormatUint(j.Command.Authority, 10),
		j.Device.ID,
		PPConnection(j.Device),
		strconv.FormatUint(j.CredentialRevision, 10),
		j.Ping.Row.Address,
		j.Ping.Row.RemoteID,
		j.Ping.Row.SessionID,
		j.Ping.Row.Login,
		j.Ping.Row.Caller,
	}
	return strings.Join(parts, "\x00")
}

// Caller holds Service.inv.Mu under current authorization during publication.
func (s *Service) decoratePingLocked(j *RunningJob, o *PDDiagnostic) {
	if o == nil || o.Scope != PDPing {
		return
	}
	key := s.pingStatsKey(j)
	history := append([]int64(nil), s.pingHistory[key]...)
	for i := range o.Rows {
		p := o.Rows[i].Ping
		if p != nil {
			p.Jitter = PingJitter(p)
			p.Baseline = PingMedianTen(history)
			p.BaselineSamples = len(history)
		}
	}
}

func (s *Service) rememberPingLocked(j *RunningJob, o *PDDiagnostic) {
	if o == nil || o.Scope != PDPing {
		return
	}
	key := s.pingStatsKey(j)
	if key == "" {
		return
	}
	for _, r := range o.Rows {
		p := r.Ping
		if p == nil || p.Sent != 3 || p.Received != 3 || p.Avg == nil || *p.Avg < 0 {
			continue
		}
		if s.pingHistory == nil {
			s.pingHistory = map[string][]int64{}
		}
		if _, exists := s.pingHistory[key]; !exists {
			s.pingOrder = append(s.pingOrder, key)
			if len(s.pingOrder) > MaxDevices {
				delete(s.pingHistory, s.pingOrder[0])
				s.pingOrder = s.pingOrder[1:]
			}
		}
		h := append(s.pingHistory[key], *p.Avg)
		if len(h) > 10 {
			h = h[len(h)-10:]
		}
		s.pingHistory[key] = h
	}
}
