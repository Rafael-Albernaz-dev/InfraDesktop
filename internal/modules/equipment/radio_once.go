package equipment

import (
	"context"
	"time"
)

func (s *Service) runRadio(ctx context.Context, j *runningJob, link *RadioLink, revisions []uint64, key string) {
	if j.radioInput.Mode != RadioOnce {
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
	defer s.wg.Done()
	defer j.cancel()
	defer func() { s.mu.Lock(); j.released = true; s.mu.Unlock() }()
	code := Unavailable
	var failure *RadioFailure
	defer func() { s.finishOnceRadioJob(ctx, j, code, failure) }()
	defer s.watchRadioOnce(ctx, j)()
	view := RadioView{APCode: OK, CPECode: OK}
	store, ok := s.store.(RadioStore)
	var r *RadioReading
	if ok {
		// Best effort presentation seed; the authorized read below decides the result.
		view.Reading, _, _ = store.RadioCurrent(ctx, j.command.Scope.Profile, j.device.ID)
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
	d := j.device
	_, _, code := s.check(ctx, j, false)
	if code == OK {
		d, code = s.pinRadioTOFU(ctx, j.command, d)
	}
	if code != OK {
		return nil, code
	}
	return s.readRadioSide(ctx, j, d, credential, j.radioInput.Family)
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
	s.mu.Lock()
	snapshot := view
	j.radio = &snapshot
	s.mu.Unlock()
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
				if c := s.authorized(ctx, j.command, func(uint64) Code { _, c := s.radioRow(ctx, j.command, j.radioInput); return c }); c != OK {
					j.cancel()
					return
				}
			}
		}
	}()
	return func() { close(watchDone); <-watchExited }
}
func (s *Service) finishOnceRadioJob(ctx context.Context, j *runningJob, code Code, failure *RadioFailure) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		code = Cancelled
		if ctx.Err() == context.DeadlineExceeded {
			code = RadioCodeWindowEnded
		}
	}
	if j.view.State == "CANCELLED" {
		code = Cancelled
	}
	j.view.Failure = failure
	j.view.Code, j.view.State = code, "SUCCEEDED"
	if code != OK && code != Cancelled && code != RadioCodeWindowEnded {
		j.view.State = "FAILED"
	}
	if code == Cancelled {
		j.view.State = "CANCELLED"
	}
	j.finished = s.now()
	j.released = true
}
