package pppoe

import (
	"context"
	"fmt"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"strings"
)

func ppStamp(kind string, in PPInput) string {
	if kind == "pppoe.diagnostic-start" {
		parts := []string{kind, in.DeviceID, fmt.Sprint(in.Revision), in.Scope, in.ObservationID, in.RowID}
		return fmt.Sprintf("ppdiag:%q", parts)
	}
	parts := []string{kind, in.DeviceID, fmt.Sprint(in.Revision), in.ObservationID, in.ReferenceID}
	return fmt.Sprintf("ppref:%q", parts)
}
func (s *Service) HandlePP(ctx context.Context, q Command, in PPInput) (PPView, Code) {
	v := PPView{}
	c := s.inv.Authorized(ctx, q, func(version uint64) Code {
		q.Authority = version
		return s.dispatchAuthorizedPP(ctx, q, in, &v)
	})
	return v, c
}

func (s *Service) dispatchAuthorizedPP(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	store, ok := s.inv.Store.(PPStore)
	if !ok {
		return Unavailable
	}
	if !PPValidQuery(in.Query) || in.Offset < 0 || in.Offset > 2*PPMaxRows {
		return Invalid
	}
	if strings.HasPrefix(q.Op, "pppoe.read-") {
		return s.handlePPRead(ctx, q, in, v)
	}
	if strings.HasPrefix(q.Op, "pppoe.diagnostic-") {
		return s.handlePD(ctx, q, in, v)
	}
	if q.Op == "pppoe.collect-status" || q.Op == "pppoe.collect-cancel" {
		return s.handleCollectJobStatus(ctx, q, in, v)
	}
	if !access.ValidCompanyID(in.DeviceID) {
		return Invalid
	}
	if q.Op == "pppoe.collect-start" {
		return s.handleCollectStart(ctx, q, in, v)
	}
	if q.Op == "pppoe.export-page" && in.ExportPart == "comparison" {
		q.Op = "pppoe.compare"
	}
	return s.dispatchStorePPOp(ctx, store, q, in, v)
}

func (s *Service) handleCollectJobStatus(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	q.JobID = in.JobID
	if q.Op == "pppoe.collect-cancel" {
		q.Op = "device.query-cancel"
	} else {
		q.Op = "device.query-status"
	}
	var c Code
	v.Job, c = s.inv.StatusKind(ctx, q, "pppoe.collect-start")
	if v.Job != nil && v.Job.Kind != "pppoe.collect-start" {
		return Invalid
	}
	return c
}

func (s *Service) handleCollectStart(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	var c Code
	v.Job, c = s.startPP(ctx, q, in)
	if c == CodeJobsFull {
		v.Reason = "job_slots"
		return Limit
	}
	if c == Limit {
		v.Reason = "operation_receipts"
	}
	if c == CodeJobInterrupted && v.Job != nil {
		v.Entity = v.Job.ID
		v.Job = nil
	}
	return c
}

func (s *Service) dispatchStorePPOp(
	ctx context.Context, store PPStore, q Command, in PPInput, v *PPView,
) Code {
	switch q.Op {
	case "pppoe.monitor-current":
		if s.mon != nil {
			res, c := s.mon.Handle(ctx, q, in)
			*v = res
			return c
		}
		monStore, ok := s.inv.Store.(DeviceMonitoringStore)
		if !ok {
			return Unavailable
		}
		var c Code
		*v, c = monStore.DeviceMonitoring(ctx, q.Scope.Profile, in)
		return c
	case "pppoe.current":
		return s.handleCurrent(ctx, store, q, in, v)
	case "pppoe.observation-get":
		var c Code
		v.Current, c = store.PPObservation(ctx, q.Scope.Profile, in.DeviceID, in.ObservationID)
		return c
	case "pppoe.context-counts":
		return s.handleContextCounts(ctx, q, in, v)
	case "pppoe.sessions-list":
		var c Code
		*v, c = store.PPPage(ctx, q.Scope.Profile, in)
		return c
	case "pppoe.session-get":
		var c Code
		*v, c = store.PPSession(ctx, q.Scope.Profile, in)
		return c
	default:
		return s.dispatchReferenceOrExportOp(ctx, store, q, in, v)
	}
}

func (s *Service) dispatchReferenceOrExportOp(
	ctx context.Context, store PPStore, q Command, in PPInput, v *PPView,
) Code {
	switch q.Op {
	case "pppoe.reference-list":
		var c Code
		*v, c = store.PPReferences(ctx, q.Scope.Profile, in)
		return c
	case "pppoe.reference-save":
		if !access.ValidCompanyID(in.Operation) ||
			!access.ValidCompanyID(in.ReferenceID) ||
			!access.ValidCompanyID(in.ObservationID) {
			return Invalid
		}
		var c Code
		in.SavedAt = PPTime(s.inv.Now())
		*v, c = store.SavePPReference(ctx, q.Scope.Profile, in, ppStamp(q.Op, in))
		return c
	case "pppoe.reference-delete":
		if !access.ValidCompanyID(in.Operation) || !access.ValidCompanyID(in.ReferenceID) {
			return Invalid
		}
		var c Code
		*v, c = store.DeletePPReference(ctx, q.Scope.Profile, in, ppStamp(q.Op, in))
		return c
	case "pppoe.export-page":
		return s.handleExportPage(ctx, store, q, in, v)
	case "pppoe.compare":
		return s.handleCompare(ctx, store, q, in, v)
	default:
		return Invalid
	}
}

func (s *Service) handleCurrent(
	ctx context.Context, store PPStore, q Command, in PPInput, v *PPView,
) Code {
	var c Code
	*v, c = store.PPCurrent(ctx, q.Scope.Profile, in.DeviceID)
	if c == OK && v.Attempt != nil && v.Attempt.State == "RUNNING" {
		s.inv.Mu.Lock()
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
		s.inv.Mu.Unlock()
	}
	return c
}

func (s *Service) handleContextCounts(ctx context.Context, q Command, in PPInput, v *PPView) Code {
	groupStore, ok := s.inv.Store.(interface {
		PPContextCounts(context.Context, string, PPInput) (PPView, Code)
	})
	if !ok {
		return Unavailable
	}
	var c Code
	*v, c = groupStore.PPContextCounts(ctx, q.Scope.Profile, in)
	return c
}

func (s *Service) handleExportPage(
	ctx context.Context, store PPStore, q Command, in PPInput, v *PPView,
) Code {
	target, c := store.PPObservation(ctx, q.Scope.Profile, in.DeviceID, in.TargetID)
	if c != OK {
		return c
	}
	baseCount, c := s.exportBaseCount(ctx, store, q.Scope.Profile, in, target)
	if c != OK {
		return c
	}
	if in.ExportPart == "diagnostics" {
		return s.handleDiagnosticExportPage(ctx, q.Scope.Profile, in, target, v)
	}
	total := baseCount + target.Count
	if in.Offset > total {
		return CursorInvalid
	}
	if in.Offset == total {
		v.Total = total
		return OK
	}
	page := in
	page.Query = &PPQuery{Sort: "row"}
	page.ObservationID = in.TargetID
	page.Offset = in.Offset - baseCount
	if in.Offset < baseCount {
		page.ObservationID = in.BaseID
		page.Offset = in.Offset
	}
	var code Code
	*v, code = store.PPPage(ctx, q.Scope.Profile, page)
	if code != OK {
		return code
	}
	v.Total = total
	v.NextOffset = in.Offset + len(v.Rows)
	v.More = v.NextOffset < total
	return OK
}

func (s *Service) exportBaseCount(
	ctx context.Context, store PPStore, company string, in PPInput, target *PPObservation,
) (int, Code) {
	if in.BaseID == "" {
		return 0, OK
	}
	base, c := store.PPObservation(ctx, company, in.DeviceID, in.BaseID)
	if c != OK {
		return 0, c
	}
	if !PPCompatible(base, target) {
		return 0, ObservationIncompatible
	}
	return base.Count, OK
}

func (s *Service) handleDiagnosticExportPage(
	ctx context.Context, company string, in PPInput, target *PPObservation, v *PPView,
) Code {
	diagnosticStore, ok := s.inv.Store.(PDDiagnosticStore)
	if !ok || !PDValidScope(in.Scope) || !access.ValidCompanyID(in.DiagnosticID) {
		return Invalid
	}
	page, code := diagnosticStore.PPDiagnosticPage(ctx, company, in)
	if code != OK {
		return code
	}
	if !PDCompatibleExport(target, page.Diagnostic) {
		return ObservationIncompatible
	}
	*v = page
	return OK
}

func (s *Service) handleCompare(
	ctx context.Context, store PPStore, q Command, in PPInput, v *PPView,
) Code {
	if in.Query != nil &&
		(in.Query.Search != "" || in.Query.Duplicates || in.Query.Sort != "" || in.Query.Desc) {
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
		if equipment.PPDeltaMatchesQuery(d, in.Query) {
			filtered = append(filtered, d)
		}
	}
	v.Total = len(filtered)
	if in.Offset > v.Total {
		return CursorInvalid
	}
	end := min(in.Offset+PageSize, v.Total)
	v.Deltas = filtered[in.Offset:end]
	v.More = end < v.Total
	v.NextOffset = end
	return OK
}

func (s *Service) ppLoad(
	ctx context.Context, store PPStore, company, device, id string,
) (*PPObservation, Code) {
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
