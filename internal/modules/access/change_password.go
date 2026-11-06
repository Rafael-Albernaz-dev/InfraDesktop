package access

import (
	"context"
	"time"
)

func (s *Service) changePassword(ctx context.Context, uid uint32, q Command) (View, Code) {
	st, code := s.renewalState(ctx, uid, q)
	if code != OK {
		return View{}, code
	}
	if !ValidPassword(q.Password) {
		return View{}, Invalid
	}
	if !validExistingPassword(q.CurrentPassword) {
		return View{ErrorReason: WrongPasswordReason}, Unauthorized
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		return View{}, Busy
	}
	if code = s.reserve(ctx); code != OK {
		return View{}, code
	}
	if !s.crypto.Verify(q.CurrentPassword, st.Account.Password) {
		return View{ErrorReason: WrongPasswordReason}, Unauthorized
	}
	verifier, code := s.crypto.Hash(q.Password)
	if code != OK {
		return View{}, code
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	code = s.store.Update(ctx, func(current *State) Code {
		if c := s.authorized(*current, uid, q); c != OK {
			return c
		}
		if current.Account.Epoch != st.Account.Epoch || !s.crypto.Equal(current.Account.Password.Hash, st.Account.Password.Hash) ||
			!s.crypto.Equal(current.Account.Password.Salt, st.Account.Password.Salt) {
			return Unauthorized
		}
		current.Account.Password = verifier
		current.Account.Epoch++
		current.Grants = nil
		current.Failures = 0
		current.NextAttempt = time.Time{}
		st = *current
		return OK
	})
	if code != OK {
		return View{}, code
	}
	currentSession := s.sessions[q.Session]
	currentSession.Epoch = st.Account.Epoch
	s.sessions = map[string]session{q.Session: currentSession}
	s.authorizationVersion++
	v := s.view(st)
	v.Authenticated = true
	return v, OK
}
