package access

import (
	"context"
	"time"
)

func (s *Service) Handle(ctx context.Context, uid uint32, q Command) (View, Code) {
	if uid != s.uid {
		return View{}, Unauthorized
	}
	switch q.Op {
	case "access.bootstrap":
		return s.handleBootstrap(ctx)
	case "access.register", "access.login", "access.recover":
		return s.handleCredentials(ctx, uid, q)
	case "access.check-issued", "access.ack-issued":
		return s.handleIssuedRecovery(ctx, q)
	case "access.renew-recovery":
		return s.renewRecovery(ctx, uid, q)
	case "access.resume":
		return s.handleResume(ctx, uid, q)
	default:
		return s.handleAuthenticated(ctx, uid, q)
	}
}
func (s *Service) handleBootstrap(ctx context.Context) (View, Code) {
	st, c := s.store.Read(ctx)
	if c != OK {
		return View{}, c
	}
	return View{Setup: st.Account == nil, Theme: "light", Companies: []Company{}}, OK
}
func (s *Service) issueOpenTokens(remember bool) (string, string, Code) {
	id, c := s.crypto.Token()
	if c != OK {
		return "", "", c
	}
	grant := ""
	if remember {
		grant, c = s.crypto.Token()
		if c != OK {
			return "", "", c
		}
	}
	return id, grant, OK
}

func (s *Service) commitOpenSession(
	st *State, remember bool, grant string, verified *Account, out *State,
) Code {
	if st.Account == nil {
		return Unauthorized
	}
	if verified != nil && (st.Account.Epoch != verified.Epoch ||
		!s.crypto.Equal(st.Account.Password.Hash, verified.Password.Hash) ||
		!s.crypto.Equal(st.Account.Password.Salt, verified.Password.Salt)) {
		return Unauthorized
	}
	st.Failures = 0
	st.NextAttempt = time.Time{}
	live := st.Grants[:0]
	for _, g := range st.Grants {
		if s.now().Before(g.Expires) && g.Epoch == st.Account.Epoch {
			live = append(live, g)
		}
	}
	st.Grants = live
	if !remember {
		st.Grants = nil
	} else {
		if len(st.Grants) >= 32 {
			return Busy
		}
		st.Grants = append(
			st.Grants,
			Grant{s.crypto.Digest(grant), st.Account.Epoch, s.now().Add(GrantTTL)},
		)
	}
	*out = *st
	return OK
}

func admitCredentialInput(q Command) Code {
	if ValidName(q.Name) && passwordForOperation(q.Op, q.Password) {
		return OK
	}
	if q.Op == "access.login" {
		return Unauthorized
	}
	return Invalid
}

func (s *Service) handleCredentials(ctx context.Context, uid uint32, q Command) (View, Code) {
	if c := admitCredentialInput(q); c != OK {
		return View{}, c
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		return View{}, Busy
	}
	if c := s.reserve(ctx); c != OK {
		return View{}, c
	}
	st, c := s.store.Read(ctx)
	if c != OK {
		return View{}, c
	}
	if q.Op == "access.login" {
		return s.handleLogin(ctx, uid, q, st)
	}
	return s.handleRegisterOrRecover(ctx, q, st)
}

func (s *Service) handleLogin(ctx context.Context, uid uint32, q Command, st State) (View, Code) {
	if st.Account == nil {
		return View{}, Unauthorized
	}
	valid := s.crypto.Verify(q.Password, st.Account.Password)
	if !valid || q.Name != st.Account.Name {
		return View{}, Unauthorized
	}
	return s.open(ctx, uid, q.Owner, q.Remember, st.Account)
}

func (s *Service) validRecoveryRequest(q Command, st State) bool {
	if st.Account == nil || q.Name != st.Account.Name {
		return false
	}
	return s.crypto.Equal(s.crypto.Digest(q.Recovery), st.Account.Recovery)
}

func (s *Service) handleRegisterOrRecover(ctx context.Context, q Command, st State) (View, Code) {
	if q.Op == "access.register" && st.Account != nil {
		return View{}, Conflict
	}
	if q.Op == "access.recover" && !s.validRecoveryRequest(q, st) {
		return View{}, Unauthorized
	}
	password, c := s.crypto.Hash(q.Password)
	if c != OK {
		return View{}, c
	}
	recovery, c := s.crypto.Token()
	if c != OK {
		return View{}, c
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c = s.store.Update(ctx, func(current *State) Code {
		return s.applyRegisterOrRecover(current, st, q, password, recovery)
	})
	if c != OK {
		return View{}, c
	}
	s.sessions = map[string]session{}
	s.pendingRecovery = map[string]bool{}
	s.issued = map[string][]byte{q.Owner: s.crypto.Digest(recovery)}
	return View{Recovery: recovery, Theme: "light", Companies: []Company{}}, OK
}

func (s *Service) applyRegisterOrRecover(
	current *State, st State, q Command, password Verifier, recovery string,
) Code {
	if q.Op == "access.register" {
		if current.Account != nil {
			return Conflict
		}
		current.Account = &Account{Name: q.Name, Epoch: 1, Theme: "light"}
	}
	if q.Op == "access.recover" {
		if current.Account == nil || current.Account.Epoch != st.Account.Epoch ||
			!s.crypto.Equal(s.crypto.Digest(q.Recovery), current.Account.Recovery) {
			return Unauthorized
		}
		current.Account.Epoch++
	}
	current.Account.Password = password
	current.Account.Recovery = s.crypto.Digest(recovery)
	current.Grants = nil
	current.Failures = 0
	current.NextAttempt = time.Time{}
	return OK
}

func (s *Service) handleIssuedRecovery(ctx context.Context, q Command) (View, Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, c := s.store.Read(ctx)
	if c != OK {
		return View{}, c
	}
	digest, ok := s.issued[q.Owner]
	if !ok || st.Account == nil || !s.crypto.Equal(digest, st.Account.Recovery) ||
		len(q.Recovery) != 64 || !s.crypto.Equal(s.crypto.Digest(q.Recovery), digest) {
		return View{}, Unauthorized
	}
	if q.Op == "access.ack-issued" {
		delete(s.issued, q.Owner)
	}
	return View{Theme: "light", Companies: []Company{}}, OK
}

func (s *Service) handleResume(ctx context.Context, uid uint32, q Command) (View, Code) {
	if len(q.Grant) != 64 {
		return View{}, Unauthorized
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, c := s.store.Read(ctx)
	if c != OK {
		return View{}, c
	}
	if st.Account == nil || !s.validResumeGrant(st, q.Grant) {
		return View{}, Unauthorized
	}
	s.pruneExpiredSessions()
	if len(s.sessions) >= 32 && s.ownerSession(q.Owner) == "" {
		return View{}, Busy
	}
	id, c := s.crypto.Token()
	if c != OK {
		return View{}, c
	}
	if existing := s.ownerSession(q.Owner); existing != "" {
		id = existing
	}
	s.sessions[id] = session{
		Owner:   q.Owner,
		UID:     uid,
		Epoch:   st.Account.Epoch,
		Expires: s.now().Add(SessionTTL),
	}
	v := s.view(st)
	v.Session = id
	v.Authenticated = true
	return v, OK
}

func (s *Service) validResumeGrant(st State, grant string) bool {
	digest := s.crypto.Digest(grant)
	for _, g := range st.Grants {
		if s.crypto.Equal(digest, g.Digest) &&
			g.Epoch == st.Account.Epoch &&
			s.now().Before(g.Expires) {
			return true
		}
	}
	return false
}

func (s *Service) pruneExpiredSessions() {
	for k, v := range s.sessions {
		if !s.now().Before(v.Expires) {
			delete(s.sessions, k)
		}
	}
}

func (s *Service) handleAuthenticated(ctx context.Context, uid uint32, q Command) (View, Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, c := s.store.Read(ctx)
	if c != OK {
		return View{}, c
	}
	if c = s.authorized(st, uid, q); c != OK {
		return View{}, c
	}
	if q.Op == "company.brand-get" || q.Op == "company.brand-set" {
		return s.brand(ctx, st, q)
	}
	if q.Op == "access.snapshot" {
		v := s.view(st)
		v.Authenticated = true
		return v, OK
	}
	if s.blockedByPendingRecovery(q.Op) {
		return View{}, Busy
	}
	return s.mutateAuthenticatedState(ctx, uid, q, st)
}

func (s *Service) blockedByPendingRecovery(op string) bool {
	if len(s.pendingRecovery) == 0 {
		return false
	}
	return op == "company.select" || op == "company.create" || op == "access.logout"
}

func (s *Service) mutateAuthenticatedState(
	ctx context.Context, uid uint32, q Command, st State,
) (View, Code) {
	id, activation, c := s.prepareAuthenticatedMutation(q)
	if c != OK {
		return View{}, c
	}
	c = s.store.Update(ctx, func(current *State) Code {
		if code := s.authorized(*current, uid, q); code != OK {
			return code
		}
		code := applyAuthenticatedOp(current, q, id)
		if code == OK {
			st = *current
		}
		return code
	})
	if c != OK {
		return View{}, c
	}
	if q.Op == "access.logout" {
		s.sessions = map[string]session{}
		s.pendingRecovery = map[string]bool{}
		return View{Theme: "light", Companies: []Company{}}, OK
	}
	if q.Op == "access.ack-recovery" {
		delete(s.pendingRecovery, q.Session)
	}
	s.activation = activation
	v := s.view(st)
	v.Authenticated = true
	return v, OK
}

func (s *Service) prepareAuthenticatedMutation(q Command) (string, string, Code) {
	id := ""
	if q.Op == "company.create" {
		if !ValidName(q.Name) || !ValidCompanyID(q.Operation) {
			return "", "", Invalid
		}
		id = q.Operation
	}
	activation := s.activation
	if q.Op == "company.select" {
		var c Code
		activation, c = s.crypto.Token()
		if c != OK {
			return "", "", c
		}
	}
	return id, activation, OK
}

func applyAuthenticatedOp(current *State, q Command, id string) Code {
	switch q.Op {
	case "access.ack-recovery":
		return OK
	case "access.logout":
		current.Account.Epoch++
		current.Grants = nil
		return OK
	case "company.create":
		return applyCompanyCreate(current, id, q.Name)
	case "company.select":
		return applyCompanySelect(current, q.Name)
	case "preferences.theme":
		if q.Theme != "light" && q.Theme != "dark" {
			return Invalid
		}
		current.Account.Theme = q.Theme
		return OK
	case "preferences.columns":
		return applyInventoryColumns(current, q)
	default:
		return Invalid
	}
}

func applyCompanyCreate(current *State, id, name string) Code {
	for _, existing := range current.Companies {
		if existing.ID == id {
			if existing.Name != name {
				return Invalid
			}
			return OK
		}
	}
	if len(current.Companies) >= 100 {
		return Invalid
	}
	current.Companies = append(current.Companies, Company{id, name})
	return OK
}

func applyCompanySelect(current *State, companyID string) Code {
	for _, v := range current.Companies {
		if v.ID == companyID {
			current.Account.Profile = companyID
			return OK
		}
	}
	return Invalid
}
