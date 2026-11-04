package pppoe

import "context"

// The summary reads the device attempt, so a refused collection must reach it.
func (s *Service) runPPRecorded(ctx context.Context, j *RunningJob) {
	s.inv.WG.Add(1)
	defer s.inv.WG.Done()
	s.runPP(ctx, j)
	s.inv.Mu.Lock()
	code := j.View.Code
	s.inv.Mu.Unlock()
	if code != Cancelled {
		s.inv.RecordAttempt(ctx, j.Command.Scope.Profile, j.Device, j.View.ID, code)
	}
}
