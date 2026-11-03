package radio

import (
	"context"
	"infradesktop/internal/modules/access"
)

func (s *Service) runRadioPreparedOnce(ctx context.Context, j *runningJob, link *RadioLink, revisions []uint64) {
	defer s.WG.Done()
	defer j.Cancel()
	defer func() { s.Mu.Lock(); j.Released = true; s.Mu.Unlock() }()
	store, ok := s.Store.(RadioStore)
	view := RadioView{Link: link, APCode: OK, CPECode: OK}
	if !ok {
		s.finishPreparedRadio(ctx, j, link, view, Unavailable, nil)
		return
	}
	// Saved readings are presentation seeds; each side is revalidated below.
	view.Reading, _, _ = store.RadioCurrent(ctx, j.Command.Scope.Profile, link.AP)
	view.CPEReading, _, _ = store.RadioCurrent(ctx, j.Command.Scope.Profile, link.CPE)
	code := OK
	var failure *RadioFailure
	for i, old := range j.RadioDevices {
		if _, c := s.radioRow(ctx, j.Command, j.RadioInput); c != OK {
			code = c
			break
		}
		reading, c := s.readPreparedRadioSide(ctx, j, link, i, old, revisions[i], store)
		recordPreparedRadioSide(&view, &failure, i, c, reading)
		// Endpoint failures leave the independent side available.
		if c == access.Unauthorized || c == Changed || c == Cancelled || ctx.Err() != nil {
			code = c
			break
		}
	}
	if code == OK && (view.APCode != OK || view.CPECode != OK) {
		code = Partial
		if view.Reading == nil && view.CPEReading == nil {
			code = view.APCode
		}
	}
	s.finishPreparedRadio(ctx, j, link, view, code, failure)
}
func (s *Service) readPreparedRadioSide(ctx context.Context, j *runningJob, link *RadioLink, i int, old Device, revision uint64,
	store RadioStore) (*RadioReading, Code) {
	d, code := s.pinRadioTOFU(ctx, j.Command, old)
	if code != OK {
		return nil, code
	}
	observed, profile, code := store.RadioCurrent(ctx, j.Command.Scope.Profile, d.ID)
	family := preparedRadioFamily(d, profile)
	// A revoked revision of the same association is not a new explicit choice.
	if code == OK && observed != nil && observed.Connection == PPConnection(d) && observed.CredentialRevision != revision {
		code = RadioCodeFamilyConfirmationRequired
	}
	var reading *RadioReading
	if code == OK {
		reading, code = s.readRadioSide(ctx, j, d, revision, family)
	}
	if code == OK && !preparedRadioIdentityMatches(i, link, reading) {
		code = RadioCodeLinkUnconfirmed
	}
	if code == OK {
		code = s.confirmPreparedRadioFamily(ctx, j, d, revision, reading, profile, store)
	}
	return reading, code
}
func preparedRadioFamily(d Device, profile *RadioProfile) RadioFamily {
	if d.Role != RoleCPE {
		return RadioRouterOSWirelessV6
	}
	if profile != nil && IsRadioFamily(profile.Family) && profile.Family != RadioUnsupported {
		return profile.Family
	}
	return RadioAuto
}
func preparedRadioIdentityMatches(i int, link *RadioLink, r *RadioReading) bool {
	if i == 0 {
		return RadioLinkPresent(*link, r)
	}
	if i == 1 {
		return r.APBSSID == link.BSSID && r.WirelessMAC == link.MAC
	}
	return true
}
func (s *Service) confirmPreparedRadioFamily(ctx context.Context, j *runningJob, d Device, revision uint64, r *RadioReading,
	profile *RadioProfile, store RadioStore) Code {
	return s.authorized(ctx, j.Command, func(uint64) Code {
		warning, critical := 15, 20
		if profile != nil && profile.Warning > 0 && profile.Critical > profile.Warning {
			warning, critical = profile.Warning, profile.Critical
		}
		return store.ConfirmRadioFamily(ctx, j.Command.Scope.Profile, d, revision, r.Family, warning, critical)
	})
}
func recordPreparedRadioSide(view *RadioView, failure **RadioFailure, i int, code Code, r *RadioReading) {
	if i == 0 {
		view.APCode = code
	} else {
		view.CPECode = code
	}
	if r != nil && r.Failure != nil && *failure == nil {
		*failure = r.Failure
	}
	if code == OK || (code == RadioCodeUnsupported && r != nil && r.Family == RadioUnsupported) {
		if i == 0 {
			view.Reading = r
		} else {
			view.CPEReading = r
		}
	}
}
func (s *Service) finishPreparedRadio(ctx context.Context, j *runningJob, link *RadioLink, view RadioView,
	code Code, failure *RadioFailure) {
	copy := *link
	copy.ConfirmedNow = view.APCode == OK
	view.Link = &copy
	s.Mu.Lock()
	defer s.Mu.Unlock()
	j.Radio = &view
	j.View.Code = code
	j.View.Failure = failure
	j.View.State = "SUCCEEDED"
	if code != OK {
		j.View.State = "FAILED"
	}
	if ctx.Err() != nil || j.View.State == "CANCELLED" || code == Cancelled {
		j.View.State = "CANCELLED"
		j.View.Code = Cancelled
	}
	j.Finished = s.Now()
	j.Released = true
}
