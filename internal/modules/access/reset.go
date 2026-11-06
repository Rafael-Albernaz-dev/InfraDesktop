package access

import (
	"context"
	"time"
)

// SetResetProof is called at composition time before the service accepts work.
func (s *Service) SetResetProof(proof ResetProof) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resetProof = proof
}

func (s *Service) resetBegin(ctx context.Context, q Command) (View, Code) {
	s.resetOperationMu.Lock()
	defer s.resetOperationMu.Unlock()
	if q.Owner == "" {
		return View{}, Unauthorized
	}
	token, code := s.crypto.Token()
	if code != OK {
		return View{}, code
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resetProof == nil {
		return View{}, Unavailable
	}
	state, code := s.store.Read(ctx)
	if code != OK {
		return View{}, code
	}
	if state.Account == nil {
		return View{}, Unauthorized
	}
	for owner, challenge := range s.resetChallenges {
		if !s.now().Before(challenge.expires) {
			delete(s.resetChallenges, owner)
		}
	}
	if _, found := s.resetChallenges[q.Owner]; !found && len(s.resetChallenges) >= 32 {
		return View{}, Busy
	}
	s.resetChallenges[q.Owner] = resetChallenge{nonce: token, expires: s.now().Add(2 * time.Minute), epoch: state.Account.Epoch}
	return View{ResetNonce: token}, OK
}

func (s *Service) resetComplete(ctx context.Context, q Command) (View, Code) {
	s.resetOperationMu.Lock()
	defer s.resetOperationMu.Unlock()
	if !ValidName(q.Name) || !ValidPassword(q.Password) || len(q.ResetNonce) != 64 || q.Owner == "" {
		return View{}, Invalid
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		return View{}, Busy
	}
	password, code := s.crypto.Hash(q.Password)
	if code != OK {
		return View{}, code
	}
	recovery, code := s.crypto.Token()
	if code != OK {
		return View{}, code
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, found := s.resetChallenges[q.Owner]
	if !found || !s.now().Before(challenge.expires) || !s.crypto.Equal([]byte(challenge.nonce), []byte(q.ResetNonce)) {
		return View{}, Unauthorized
	}
	delete(s.resetChallenges, q.Owner)
	if s.resetProof == nil || !s.resetProof.Consume(q.ResetNonce) {
		return View{}, Unauthorized
	}
	code = s.store.Update(ctx, func(current *State) Code {
		return s.applyReset(current, challenge, q.Name, password, recovery)
	})
	if code != OK {
		return View{}, code
	}
	s.sessions = map[string]session{}
	s.pendingRecovery = map[string]bool{}
	s.resetChallenges = map[string]resetChallenge{}
	s.issued = map[string][]byte{q.Owner: s.crypto.Digest(recovery)}
	s.authorizationVersion++
	return View{Recovery: recovery, Theme: "light", Companies: []Company{}}, OK
}

func (s *Service) applyReset(current *State, challenge resetChallenge, name string, password Verifier, recovery string) Code {
	if current.Account == nil || current.Account.Epoch != challenge.epoch {
		return Unauthorized
	}
	current.Account.Name = name
	current.Account.Password = password
	current.Account.Recovery = s.crypto.Digest(recovery)
	current.Account.Epoch++
	current.Grants = nil
	current.Failures = 0
	current.NextAttempt = time.Time{}
	return OK
}
