package inventory

import (
	"context"
	"time"
)

// A finished result is not thrown away because the disk was slow for a moment.
// Each try keeps its short budget, so the service lock is never held longer than
// before; on Unavailable (database busy or slow) the lock is released, the result
// waits, and the write is tried again, up to persistRetryWindow in total. Only
// then does the job end as PERSISTENCE_FAILED. Zabbix and The Dude likewise keep
// collection apart from the database write (docs/reference/ferramentas-semelhantes).
const (
	persistRetryWindow = 15 * time.Second
	persistRetryPause  = 250 * time.Millisecond
)

// PersistWithRetry runs try with a fresh context of budget until it stops answering
// Unavailable, the window ends or the service closes. It must be called without Mu.
func (s *Service) PersistWithRetry(budget time.Duration, try func(context.Context) Code) Code {
	window, pause := persistRetryWindow, persistRetryPause
	if s.PersistRetry.Window > 0 {
		window, pause = s.PersistRetry.Window, s.PersistRetry.Pause
	}
	started := time.Now()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		code := try(ctx)
		cancel()
		if code != Unavailable || time.Since(started)+pause >= window || s.closing() {
			return code
		}
		time.Sleep(pause)
	}
}

// PersistRetryLimits overrides the retry window in tests; zero keeps the defaults.
type PersistRetryLimits struct {
	Window, Pause time.Duration
}

func (s *Service) closing() bool {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	return s.Closed
}
