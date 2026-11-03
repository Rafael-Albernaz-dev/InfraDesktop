package radio

import (
	"context"
	"time"
)

func (s *Service) readRadioSide(ctx context.Context,
	j *runningJob,
	d Device,
	credential uint64,
	family RadioFamily,
	read ...func() (*RadioReading,
		Code)) (*RadioReading,
	Code) {
	pathStarted := time.Now()
	ctx, cancel := context.WithTimeout(ctx, QueryBudget)
	defer cancel()
	copy := runningJob{Command: j.Command, Device: d, CredentialRevision: credential}
	copy.Cancel = cancel
	defer s.watchPP(ctx, &copy)()
	check := func() Code {
		if _, c := s.radioRow(ctx, j.Command, j.RadioInput); c != OK {
			return c
		}
		_, _, c := s.check(ctx, &copy, false)
		return c
	}
	if c := check(); c != OK {
		return nil, c
	}
	if !d.Active {
		return nil, Inactive
	}
	if d.Fingerprint == "" {
		return nil, TrustRequired
	}
	r, code := s.fetchRadioSide(ctx, &copy, d, family, check, read)
	return s.finishRadioSide(ctx, j, d, credential, r, code, check, pathStarted)
}
func (s *Service) fetchRadioSide(ctx context.Context, job *runningJob, d Device,
	family RadioFamily, check func() Code, read []func() (*RadioReading, Code)) (*RadioReading, Code) {
	if len(read) > 0 {
		return read[0]()
	}
	user, secret, code := s.check(ctx, job, true)
	if code != OK {
		return nil, code
	}
	remote, ok := s.Remote.(RadioRemote)
	if !ok {
		return nil, Unavailable
	}
	reading, code := remote.ReadRadio(ctx, d, user, secret, family, check)
	secret = ""
	return reading, code
}
func (s *Service) finishRadioSide(ctx context.Context, j *runningJob, d Device, credential uint64,
	r *RadioReading, c Code, check func() Code, pathStarted time.Time) (*RadioReading, Code) {
	readingCode := c
	unsupported := isUnsupportedRadioObservation(c, r, d)
	if (c != OK && !unsupported) || r == nil {
		if c == OK {
			c = Partial
		}
		if r != nil {
			r.Seconds = time.Since(pathStarted).Seconds()
		}
		return r, c
	}
	if c := check(); c != OK {
		return nil, c
	}
	r.ID, c = s.IDs.ID()
	if c != OK {
		return nil, c
	}
	StampRadio(r, d, credential, s.Now())

	r.Seconds = time.Since(pathStarted).Seconds()
	// An unidentified peer cannot replace a previously confirmed family or
	// stored reading. Return this observation only in the current job.
	if preserveUnknownStandalone(unsupported, j.RadioInput, r) {
		return r, readingCode
	}
	// A filtered link tick is ephemeral. It must never replace the complete
	// AP inventory (or re-date metrics that were not collected by this tick).
	if r.Scope == "link-tick" {
		return r, OK
	}
	c = s.saveRadioSide(ctx, j, d, credential, r)
	if c != OK {
		return nil, c
	}
	r.Seconds = time.Since(pathStarted).Seconds()
	return r, readingCode
}
func (s *Service) saveRadioSide(ctx context.Context, j *runningJob, d Device, credential uint64, r *RadioReading) Code {
	return s.authorized(ctx, j.Command, func(uint64) Code {
		if _, code := s.radioRow(ctx, j.Command, j.RadioInput); code != OK {
			return code
		}
		store, ok := s.Store.(RadioStore)
		if !ok {
			return Unavailable
		}
		return store.SaveRadio(ctx, j.Command.Scope.Profile, d, credential, r)
	})
}
func isUnsupportedRadioObservation(code Code, r *RadioReading, d Device) bool {
	return code == RadioCodeUnsupported && r != nil && r.Family == RadioUnsupported &&
		(d.Role == RoleCPE || d.Role == RoleAP || d.Role == RolePTP)
}
func preserveUnknownStandalone(unsupported bool, in RadioInput, r *RadioReading) bool {
	return unsupported && in.RowID == "" && (r.Manufacturer == "" || r.Model == "" || r.Version == "")
}
