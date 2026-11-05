package access

import (
	"context"
	"unicode/utf8"
)

func validExistingPassword(p string) bool {
	return p != "" && utf8.ValidString(p) && utf8.RuneCountInString(p) <= 128
}

func passwordForOperation(op, password string) bool {
	if op == "access.login" {
		return validExistingPassword(password)
	}
	return ValidPassword(password)
}

func (s *Service) renewalState(ctx context.Context, uid uint32, q Command) (State, Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, c := s.store.Read(ctx)
	if c == OK {
		c = s.authorized(st, uid, q)
	}
	// A pending issuance may only be replaced by its owner explicitly.
	if c == OK {
		for owner := range s.pendingRecovery {
			if owner != q.Session || !q.Replace {
				c = Busy
			}
		}
	}
	return st, c
}
