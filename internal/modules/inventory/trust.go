package inventory

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Service) status(ctx context.Context, q Command) (*Job, Code) {
	return s.StatusKind(ctx, q, "legacy")
}

func (s *Service) StatusKind(ctx context.Context, q Command, kind string) (*Job, Code) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	s.PruneLocked()
	j := s.Jobs[q.JobID]
	if j == nil || !SameScope(q, j.Command) {
		return nil, Changed
	}
	if !matchingJobKind(j.View.Kind, kind) {
		return nil, Invalid
	}
	if q.Op == "device.query-cancel" && (j.Finished.IsZero() || j.Command.Op == "trust.inspect") {
		if code := s.cancelJobLocked(ctx, q, j); code != OK {
			return nil, code
		}
	}
	return CopyJob(j), OK
}

func matchingJobKind(actual, expected string) bool {
	if expected == "legacy" {
		return actual == "device.query-start" || actual == "trust.inspect"
	}
	return actual == expected
}

func (s *Service) cancelJobLocked(ctx context.Context, q Command, j *RunningJob) Code {
	j.Cancel()
	j.View.Result = nil
	j.View.Fingerprint = ""
	j.View.Code = Cancelled
	j.View.State = "CANCELLED"
	if j.Command.Op != "device.query-start" {
		return OK
	}
	code := s.Store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		d := DeviceByID(st, j.Device.ID)
		if d == nil || d.Revision != j.Device.Revision {
			return Revision
		}
		if d.LastAttempt != nil && d.LastAttempt.JobID == j.View.ID {
			at := s.Now().UTC().Format(time.RFC3339Nano)
			d.LastAttempt = &Attempt{JobID: j.View.ID, State: "CANCELLED", Code: Cancelled, At: at}
		}
		return OK
	})
	if code != OK {
		return CodePersistenceFailed
	}
	return OK
}

func (s *Service) confirm(ctx context.Context, q Command, v *View) Code {
	if !ValidID(q.Operation) {
		return Invalid
	}
	stamp := fmt.Sprintf("first-acceptance:%q:%q:%d:%q", q.ID, q.JobID, q.Revision, q.Fingerprint)
	st, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return c
	}
	if replayed, code := checkConfirmReplay(&st, q, stamp, v); replayed {
		return code
	}
	if len(st.Operations) >= MaxOperations {
		return Limit
	}
	candidate, c := s.validateConfirmJob(q)
	if c != OK {
		return c
	}
	c = s.Store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		return s.applyTrustConfirmation(st, q, stamp, candidate, v)
	})
	if c == OK {
		s.Invalidate(q.Scope.Profile, q.ID, false)
	}
	return c
}

func checkConfirmReplay(st *State, q Command, stamp string, v *View) (bool, Code) {
	for _, op := range st.Operations {
		if op.ID != q.Operation {
			continue
		}
		if !strings.HasPrefix(op.Stamp, stamp+";accepted:") {
			return true, Revision
		}
		v.Device = DeviceByID(st, q.ID)
		if v.Device == nil || v.Device.Fingerprint != q.Fingerprint {
			return true, Revision
		}
		return true, OK
	}
	return false, OK
}

func (s *Service) validateConfirmJob(q Command) (string, Code) {
	s.Mu.Lock()
	j := s.Jobs[q.JobID]
	if !admissibleConfirmJob(j, q, s.Now()) {
		s.Mu.Unlock()
		return "", TrustRequired
	}
	candidate := j.View.Fingerprint
	s.Mu.Unlock()
	if !strings.HasPrefix(candidate, "SHA256:") {
		return "", Invalid
	}
	return candidate, OK
}

func admissibleConfirmJob(j *RunningJob, q Command, now time.Time) bool {
	if j == nil || !SameScope(q, j.Command) || j.Command.Op != "trust.inspect" {
		return false
	}
	if j.View.State != "SUCCEEDED" || now.Sub(j.Finished) > JobRetention {
		return false
	}
	return j.Device.ID == q.ID &&
		j.Device.Revision == q.Revision &&
		j.View.Fingerprint == q.Fingerprint
}

func (s *Service) applyTrustConfirmation(st *State, q Command, stamp, candidate string, v *View) Code {
	d := DeviceByID(st, q.ID)
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
		return Revision
	}
	d.Fingerprint = candidate
	d.TrustAcceptance = &TrustAcceptance{
		Mode: "explicit-first-acceptance", At: s.Now().UTC().Format(time.RFC3339Nano),
		UID: q.UID, Company: q.Scope.Profile, DeviceID: d.ID,
		Address: d.Address, Port: d.Port, Revision: d.Revision, Fingerprint: candidate,
	}
	d.Revision++
	v.Device = d
	receipt := stamp + fmt.Sprintf(";accepted:%q", fmt.Sprint(*d.TrustAcceptance))
	st.Operations = append(st.Operations, Operation{ID: q.Operation, Stamp: receipt, Entity: q.ID})
	return OK
}

func (s *Service) Invalidate(profile, id string, cred bool) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	for _, j := range s.Jobs {
		if j.Command.Scope.Profile == profile && jobMatchesTarget(j, id, cred) {
			j.Cancel()
			if j.Radio != nil {
				j.Radio = invalidateRadioView(*j.Radio, j.RadioDevices, id, cred)
			}
			j.View.Result = nil
			j.View.Fingerprint = ""
			j.View.Code = Cancelled
			j.View.State = "CANCELLED"
		}
	}
}

func jobMatchesTarget(j *RunningJob, id string, cred bool) bool {
	if j.Device.ID == id || (cred && j.Device.CredentialID == id) {
		return true
	}
	for _, d := range j.RadioDevices {
		if d.ID == id || (cred && d.CredentialID == id) {
			return true
		}
	}
	return false
}

func invalidateRadioView(view RadioView, devices []Device, id string, cred bool) *RadioView {
	matches := func(deviceID string) bool {
		for _, d := range devices {
			if d.ID == deviceID && (d.ID == id || (cred && d.CredentialID == id)) {
				return true
			}
		}
		return false
	}
	if view.Reading != nil && matches(view.Reading.DeviceID) {
		copy := *view.Reading
		copy.Compatible = false
		view.Reading = &copy
		view.APCode = Cancelled
	}
	if view.CPEReading != nil && matches(view.CPEReading.DeviceID) {
		copy := *view.CPEReading
		copy.Compatible = false
		view.CPEReading = &copy
		view.CPECode = Cancelled
	}
	if view.Cost != nil {
		copy := *view.Cost
		copy.Enabled = false
		copy.Reason = "O cadastro ou a credencial mudou. Meça novamente."
		view.Cost = &copy
	}
	return &view
}
