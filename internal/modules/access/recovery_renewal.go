package access

import (
	"context"
	"time"
)

func (s *Service) renewRecovery(ctx context.Context, uid uint32, q Command) (View, Code) {
	st, c := s.renewalState(ctx, uid, q)
	if c != OK {
		return View{}, c
	}
	if !validExistingPassword(q.Password) {
		return View{ErrorReason: WrongPasswordReason}, Unauthorized
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		return View{}, Busy
	}
	if c = s.reserve(ctx); c != OK {
		return View{}, c
	}
	if !s.crypto.Verify(q.Password, st.Account.Password) {
		return View{ErrorReason: WrongPasswordReason}, Unauthorized
	}
	recovery, c := s.crypto.Token()
	if c != OK {
		return View{}, c
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c = s.store.Update(ctx, func(current *State) Code {
		if c := s.authorized(*current, uid, q); c != OK {
			return c
		}
		if current.Account.Epoch != st.Account.Epoch ||
			!s.crypto.Equal(current.Account.Password.Hash, st.Account.Password.Hash) ||
			!s.crypto.Equal(current.Account.Recovery, st.Account.Recovery) {
			return Unauthorized
		}
		current.Account.Recovery = s.crypto.Digest(recovery)
		current.Failures = 0
		current.NextAttempt = time.Time{}
		st = *current
		return OK
	})
	if c != OK {
		return View{}, c
	}
	s.authorizationVersion++
	s.pendingRecovery[q.Session] = true
	v := s.view(st)
	v.Authenticated = true
	v.Recovery = recovery
	return v, OK
}
