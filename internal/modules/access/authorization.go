package access

import "context"

// WithAuthorization protects a short local operation and publication against
// logout/reset/company switches. Its callback must never perform network IO.
func (s *Service) WithAuthorization(ctx context.Context, uid uint32, q Command, fn func(uint64) Code) Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, c := s.store.Read(ctx)
	if c != OK {
		return c
	}
	if c = s.authorized(st, uid, q); c != OK {
		return c
	}
	if q.Profile == "" || q.ExpectedProfile != q.Profile || q.ExpectedActivation != q.Activation {
		return Changed
	}
	if len(s.pendingRecovery) > 0 {
		return Busy
	}
	return fn(s.authorizationVersion)
}
