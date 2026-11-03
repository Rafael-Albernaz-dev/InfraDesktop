package radio

import (
	"context"
	"time"
)

func (s *Service) runRadio(ctx context.Context, j *runningJob, link *RadioLink, revisions []uint64, key string) {
	if j.RadioInput.Mode != RadioOnce {
		s.runRadioPersistent(ctx, j, link, revisions, key)
		return
	}
	if link != nil {
		s.runRadioPreparedOnce(ctx, j, link, revisions)
		return
	}
	s.runStandaloneRadio(ctx, j, revisions)
}
func (s *Service) runStandaloneRadio(ctx context.Context, j *runningJob, revisions []uint64) {
	defer s.WG.Done()
	defer j.Cancel()
	defer func() { s.Mu.Lock(); j.Released = true; s.Mu.Unlock() }()
	code := Unavailable
	var failure *RadioFailure
	defer func() { s.finishOnceRadioJob(ctx, j, code, failure) }()
	defer s.watchRadioOnce(ctx, j)()
	view := RadioView{APCode: OK, CPECode: OK}
	store, ok := s.Store.(RadioStore)
	var r *RadioReading
	if ok {
		// Best effort presentation seed; the authorized read below decides the result.
		view.Reading, _, _ = store.RadioCurrent(ctx, j.Command.Scope.Profile, j.Device.ID)
		r, code = s.readStandaloneRadio(ctx, j, revisions[0])
	}
	view.APCode = code
	if code != OK && r != nil {
		failure = r.Failure
	}
	if code == OK || (code == RadioCodeUnsupported && r != nil && r.Family == RadioUnsupported) {
		view.Reading = r
	}
	code = standaloneRadioCode(view, r)
	s.publishOnceRadioView(j, view)
}
func (s *Service) readStandaloneRadio(ctx context.Context, j *runningJob, credential uint64) (*RadioReading, Code) {
	d := j.Device
	_, _, code := s.check(ctx, j, false)
	if code == OK {
		d, code = s.pinRadioTOFU(ctx, j.Command, d)
	}
	if code != OK {
		return nil, code
	}
	return s.readRadioSide(ctx, j, d, credential, j.RadioInput.Family)
}
func standaloneRadioCode(view RadioView, r *RadioReading) Code {
	code := OK
	if view.APCode != OK {
		code = Partial
		if view.Reading == nil && view.CPEReading == nil {
			code = view.APCode
		}
	}
	if view.APCode == RadioCodeUnsupported && r != nil && r.Family == RadioUnsupported {
		return RadioCodeUnsupported
	}
	return code
}
func (s *Service) publishOnceRadioView(j *runningJob, view RadioView) {
	s.Mu.Lock()
	snapshot := view
	j.Radio = &snapshot
	s.Mu.Unlock()
}
func (s *Service) watchRadioOnce(ctx context.Context, j *runningJob) func() {
	watchDone, watchExited := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watchExited)
		ticker := time.NewTicker(40 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watchDone:
				return
			case <-ticker.C:
				if c := s.authorized(ctx, j.Command, func(uint64) Code { _, c := s.radioRow(ctx, j.Command, j.RadioInput); return c }); c != OK {
					j.Cancel()
					return
				}
			}
		}
	}()
	return func() { close(watchDone); <-watchExited }
}
func (s *Service) finishOnceRadioJob(ctx context.Context, j *runningJob, code Code, failure *RadioFailure) {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if ctx.Err() != nil {
		code = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			code = RadioCodeWindowEnded
		}
	}
	if j.View.State == "CANCELLED" {
		code = Cancelled
	}
	j.View.Failure = failure
	j.View.Code, j.View.State = code, "SUCCEEDED"
	if code != OK && code != Cancelled && code != RadioCodeWindowEnded {
		j.View.State = "FAILED"
	}
	if code == Cancelled {
		j.View.State = "CANCELLED"
	}
	j.Finished = s.Now()
	j.Released = true
}
