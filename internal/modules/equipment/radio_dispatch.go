package equipment

import (
	"context"
	"infradesktop/internal/modules/access"
)

func (s *Service) handleRadioJob(ctx context.Context, q Command, in RadioInput, store RadioStore, v *RadioView) Code {
	if q.Op == "radio.cancel" && in.JobID == "" {
		if !access.ValidCompanyID(in.DeviceID) || !access.ValidCompanyID(in.Operation) {
			return Invalid
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, j := range s.jobs {
			if j.command.Op == "radio.start" &&
				sameScope(q,
					j.command) &&
				j.radioInput.DeviceID == in.DeviceID &&
				j.command.Operation == in.Operation {
				j.cancel()
				j.view.State, j.view.Code = "CANCELLED", Cancelled
				v.Job = copyJob(j)
			}
		}
		return OK
	}
	if !access.ValidCompanyID(in.JobID) {
		return Invalid
	}
	check := q
	check.JobID = in.JobID
	check.Op = "device.query-status"
	if q.Op == "radio.cancel" {
		check.Op = "device.query-cancel"
	}
	job, c := s.statusKind(ctx, check, "radio.start")
	if c != OK {
		return c
	}
	s.mu.Lock()
	j := s.jobs[in.JobID]
	if j != nil && j.radio != nil {
		*v = *j.radio
	}
	s.mu.Unlock()
	v.Job = job
	var code Code
	*v, code = radioPage(*v, in)
	return code
}
func (s *Service) handleRadioCurrent(ctx context.Context, q Command, in RadioInput, store RadioStore, v *RadioView) Code {
	if !access.ValidCompanyID(in.DeviceID) {
		return Invalid
	}
	var c Code
	v.Reading, v.Profile, c = store.RadioCurrent(ctx, q.Scope.Profile, in.DeviceID)
	if c != OK {
		return c
	}
	if q.Op == "radio.confirm-family" {
		targetStore, ok := s.store.(PPStore)
		if !ok {
			return Unavailable
		}
		t, c := targetStore.ReadPPTarget(ctx, q.Scope.Profile, in.DeviceID, false)
		if c != OK {
			return c
		}
		if t.Device.Revision != in.Revision {
			return Revision
		}
		c = store.ConfirmRadioFamily(ctx, q.Scope.Profile, t.Device, t.Credential.Metadata.Revision, in.Family, in.Warning, in.Critical)
		if c != OK {
			return c
		}
		v.Reading, v.Profile, c = store.RadioCurrent(ctx, q.Scope.Profile, in.DeviceID)
		if c != OK {
			return c
		}
	}
	*v, c = radioPage(*v, in)
	return c
}
