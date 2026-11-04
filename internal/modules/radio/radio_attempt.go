package radio

import "context"

func (s *Service) runRadioRecorded(ctx context.Context, j *runningJob, link *RadioLink, revisions []uint64, key string) {
	s.WG.Add(1)
	defer s.WG.Done()
	defer s.recordRadioAttempts(ctx, j)
	s.runRadio(ctx, j, link, revisions, key)
}

func (s *Service) recordRadioAttempts(ctx context.Context, j *runningJob) {
	s.Mu.Lock()
	overall, id := j.View.Code, j.View.ID
	sides := []Code{OK, OK}
	if j.Radio != nil {
		sides = []Code{j.Radio.APCode, j.Radio.CPECode}
	}
	s.Mu.Unlock()
	devices := j.RadioDevices
	if len(devices) == 0 {
		devices = []Device{j.Device}
	}
	for i, device := range devices {
		code, known := radioAttemptOutcome(overall, sides[min(i, 1)], len(devices) == 1)
		if known {
			s.RecordAttempt(ctx, j.Command.Scope.Profile, device, id, code)
		}
	}
}

// A side that never connected keeps its previous attempt: only its own failure,
// or a run that finished well, says something about that device's credential.
func radioAttemptOutcome(overall, side Code, single bool) (Code, bool) {
	switch {
	case overall == Cancelled:
		return "", false
	case side != "" && side != OK && side != RadioCodeUnsupported:
		return side, true
	case overall == OK || overall == RadioCodeWindowEnded:
		return OK, true
	case single:
		return overall, true
	}
	return "", false
}
