// Package access owns local account, authorization and company policy.
package access

import (
	"context"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

type Code string

const (
	OK           Code = "OK"
	Invalid      Code = "INVALID_REQUEST"
	Unauthorized Code = "UNAUTHENTICATED"
	Changed      Code = "PROFILE_CHANGED"
	Busy         Code = "BUSY"
	Unavailable  Code = "SERVICE_UNAVAILABLE"
	Conflict     Code = "ACCOUNT_EXISTS"
	Limited      Code = "TRY_LATER"
	SessionTTL        = 12 * time.Hour
	GrantTTL          = 30 * 24 * time.Hour
)

type Verifier struct {
	Algorithm          string
	Memory, Iterations uint32
	Parallelism        uint8
	Salt, Hash         []byte
}
type Account struct {
	Name           string
	Password       Verifier
	Recovery       []byte
	Epoch          uint64
	Theme, Profile string
}
type Company struct{ ID, Name string }
type Grant struct {
	Digest  []byte
	Epoch   uint64
	Expires time.Time
}
type State struct {
	Account     *Account
	Companies   []Company
	Grants      []Grant
	Failures    uint32
	NextAttempt time.Time
}

// Store serializes short, atomic updates. A rejected callback never commits.
// Technology errors are translated to Unavailable by the adapter.
type Store interface {
	Read(context.Context) (State, Code)
	Update(context.Context, func(*State) Code) Code
}
type Crypto interface {
	Hash(string) (Verifier, Code)
	Verify(string, Verifier) bool
	Token() (string, Code)
	Digest(string) []byte
	Equal([]byte, []byte) bool
}
type Command struct {
	Op, Session, Grant, Name, Password, Recovery, Profile, Activation, Theme, Operation string
	ExpectedProfile, ExpectedActivation                                                 string
	Brand                                                                               Branding
	Remember, Replace                                                                   bool
	// Owner is a transport-authenticated opaque process identity, never a wire field.
	Owner string
}
type View struct {
	Setup, Authenticated                                       bool
	Name, Theme, Profile, Activation, Session, Grant, Recovery string
	Companies                                                  []Company
	Brand                                                      *Branding
}
type session struct {
	Owner   string
	UID     uint32
	Epoch   uint64
	Expires time.Time
}
type Service struct {
	store           Store
	brands          BrandStore
	issued          map[string][]byte
	crypto          Crypto
	now             func() time.Time
	uid             uint32
	kdf             chan struct{}
	mu              sync.Mutex
	sessions        map[string]session
	activation      string
	pendingRecovery map[string]bool
}

func New(store Store, crypto Crypto, now func() time.Time, uid uint32, activation string, brands ...BrandStore) *Service {
	var brandStore BrandStore
	if len(brands) > 0 {
		brandStore = brands[0]
	}
	return &Service{store: store, brands: brandStore, issued: map[string][]byte{}, crypto: crypto, now: now, uid: uid, kdf: make(chan struct{}, 1), sessions: map[string]session{}, pendingRecovery: map[string]bool{}, activation: activation}
}
func ValidPassword(p string) bool {
	n := utf8.RuneCountInString(p)
	return utf8.ValidString(p) && n >= 15 && n <= 128
}
func ValidName(n string) bool {
	if !utf8.ValidString(n) || strings.TrimSpace(n) != n || utf8.RuneCountInString(n) < 1 || utf8.RuneCountInString(n) > 80 {
		return false
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func Backoff(failures uint32) time.Duration {
	if failures > 5 {
		return 30 * time.Second
	}
	if failures == 0 {
		return 0
	}
	return time.Second * time.Duration(1<<(failures-1))
}
func (s *Service) reserve(ctx context.Context) Code {
	return s.store.Update(ctx, func(st *State) Code {
		if s.now().Before(st.NextAttempt) {
			return Limited
		}
		if st.Failures < 6 {
			st.Failures++
		}
		st.NextAttempt = s.now().Add(Backoff(st.Failures))
		return OK
	})
}
func (s *Service) authorized(st State, uid uint32, q Command) Code {
	v, ok := s.sessions[q.Session]
	if !ok || uid != s.uid || v.UID != uid || v.Owner != q.Owner || st.Account == nil || v.Epoch != st.Account.Epoch || !s.now().Before(v.Expires) {
		return Unauthorized
	}
	if q.Profile != st.Account.Profile || q.Activation != s.activation {
		return Changed
	}
	return OK
}
func (s *Service) view(st State) View {
	v := View{Setup: st.Account == nil, Theme: "light", Companies: []Company{}}
	if st.Account != nil {
		v.Name = st.Account.Name
		v.Theme = st.Account.Theme
		v.Profile = st.Account.Profile
		v.Activation = s.activation
		v.Companies = st.Companies
	}
	return v
}
func (s *Service) open(ctx context.Context, uid uint32, owner string, remember bool, verified *Account) (View, Code) {
	id, c := s.crypto.Token()
	if c != OK {
		return View{}, c
	}
	grant := ""
	if remember {
		grant, c = s.crypto.Token()
		if c != OK {
			return View{}, c
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.sessions {
		if !s.now().Before(v.Expires) {
			delete(s.sessions, k)
		}
	}
	if len(s.sessions) >= 32 && s.ownerSession(owner) == "" {
		return View{}, Busy
	}
	var state State
	c = s.store.Update(ctx, func(st *State) Code {
		if st.Account == nil {
			return Unauthorized
		}
		if verified != nil && (st.Account.Epoch != verified.Epoch || !s.crypto.Equal(st.Account.Password.Hash, verified.Password.Hash) || !s.crypto.Equal(st.Account.Password.Salt, verified.Password.Salt)) {
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
		}
		if remember {
			if len(st.Grants) >= 32 {
				return Busy
			}
			st.Grants = append(st.Grants, Grant{s.crypto.Digest(grant), st.Account.Epoch, s.now().Add(GrantTTL)})
		}
		state = *st
		return OK
	})
	if c != OK {
		return View{}, c
	}
	for k, v := range s.sessions {
		if !s.now().Before(v.Expires) {
			delete(s.sessions, k)
		}
	}
	if len(s.sessions) >= 32 && s.ownerSession(owner) == "" {
		return View{}, Busy
	}
	if existing := s.ownerSession(owner); existing != "" {
		id = existing
	}
	s.sessions[id] = session{Owner: owner, UID: uid, Epoch: state.Account.Epoch, Expires: s.now().Add(SessionTTL)}
	v := s.view(state)
	v.Authenticated = true
	v.Session = id
	v.Grant = grant
	return v, OK
}
func (s *Service) Handle(ctx context.Context, uid uint32, q Command) (View, Code) {
	if uid != s.uid {
		return View{}, Unauthorized
	}
	if q.Op == "access.bootstrap" {
		st, c := s.store.Read(ctx)
		if c != OK {
			return View{}, c
		}
		return View{Setup: st.Account == nil, Theme: "light", Companies: []Company{}}, OK
	}
	if q.Op == "access.register" || q.Op == "access.login" || q.Op == "access.recover" {
		if !ValidName(q.Name) || !ValidPassword(q.Password) {
			if q.Op == "access.login" {
				return View{}, Unauthorized
			}
			return View{}, Invalid
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
			if st.Account == nil {
				return View{}, Unauthorized
			}
			valid := s.crypto.Verify(q.Password, st.Account.Password)
			if !valid || q.Name != st.Account.Name {
				return View{}, Unauthorized
			}
			return s.open(ctx, uid, q.Owner, q.Remember, st.Account)
		}
		if q.Op == "access.register" && st.Account != nil {
			return View{}, Conflict
		}
		if q.Op == "access.recover" && (st.Account == nil || q.Name != st.Account.Name || !s.crypto.Equal(s.crypto.Digest(q.Recovery), st.Account.Recovery)) {
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
			if q.Op == "access.register" {
				if current.Account != nil {
					return Conflict
				}
				current.Account = &Account{Name: q.Name, Epoch: 1, Theme: "light"}
			}
			if q.Op == "access.recover" {
				if current.Account == nil || current.Account.Epoch != st.Account.Epoch || !s.crypto.Equal(s.crypto.Digest(q.Recovery), current.Account.Recovery) {
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
		})
		if c != OK {
			return View{}, c
		}
		s.sessions = map[string]session{}
		s.pendingRecovery = map[string]bool{}
		s.issued = map[string][]byte{q.Owner: s.crypto.Digest(recovery)}
		return View{Recovery: recovery, Theme: "light", Companies: []Company{}}, OK
	}
	if q.Op == "access.check-issued" || q.Op == "access.ack-issued" {
		s.mu.Lock()
		defer s.mu.Unlock()
		st, c := s.store.Read(ctx)
		if c != OK {
			return View{}, c
		}
		digest, ok := s.issued[q.Owner]
		if !ok || st.Account == nil || !s.crypto.Equal(digest, st.Account.Recovery) || len(q.Recovery) != 64 || !s.crypto.Equal(s.crypto.Digest(q.Recovery), digest) {
			return View{}, Unauthorized
		}
		if q.Op == "access.ack-issued" {
			delete(s.issued, q.Owner)
		}
		return View{Theme: "light", Companies: []Company{}}, OK
	}
	if q.Op == "access.renew-recovery" {
		return s.renewRecovery(ctx, uid, q)
	}
	if q.Op == "access.resume" {
		if len(q.Grant) != 64 {
			return View{}, Unauthorized
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		st, c := s.store.Read(ctx)
		if c != OK {
			return View{}, c
		}
		if st.Account == nil {
			return View{}, Unauthorized
		}
		found := false
		digest := s.crypto.Digest(q.Grant)
		for _, g := range st.Grants {
			if s.crypto.Equal(digest, g.Digest) && g.Epoch == st.Account.Epoch && s.now().Before(g.Expires) {
				found = true
			}
		}
		if !found {
			return View{}, Unauthorized
		}
		for k, v := range s.sessions {
			if !s.now().Before(v.Expires) {
				delete(s.sessions, k)
			}
		}
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
		s.sessions[id] = session{Owner: q.Owner, UID: uid, Epoch: st.Account.Epoch, Expires: s.now().Add(SessionTTL)}
		v := s.view(st)
		v.Session = id
		v.Authenticated = true
		return v, OK
	}
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
	if len(s.pendingRecovery) > 0 && (q.Op == "company.select" || q.Op == "company.create" || q.Op == "access.logout") {
		return View{}, Busy
	}
	id := ""
	if q.Op == "company.create" {
		if !ValidName(q.Name) {
			return View{}, Invalid
		}
		id = q.Operation
		if !ValidCompanyID(id) {
			return View{}, Invalid
		}
		if c != OK {
			return View{}, c
		}
	}
	activation := s.activation
	if q.Op == "company.select" {
		activation, c = s.crypto.Token()
		if c != OK {
			return View{}, c
		}
	}
	c = s.store.Update(ctx, func(current *State) Code {
		if c := s.authorized(*current, uid, q); c != OK {
			return c
		}
		switch q.Op {
		case "access.ack-recovery":
			// Cleared only after commit succeeds below.
		case "access.logout":
			current.Account.Epoch++
			current.Grants = nil
		case "company.create":
			for _, existing := range current.Companies {
				if existing.ID == id {
					if existing.Name != q.Name {
						return Invalid
					}
					st = *current
					return OK
				}
			}
			if len(current.Companies) >= 100 {
				return Invalid
			}
			current.Companies = append(current.Companies, Company{id, q.Name})
		case "company.select":
			found := false
			for _, v := range current.Companies {
				if v.ID == q.Name {
					found = true
				}
			}
			if !found {
				return Invalid
			}
			current.Account.Profile = q.Name
		case "preferences.theme":
			if q.Theme != "light" && q.Theme != "dark" {
				return Invalid
			}
			current.Account.Theme = q.Theme
		default:
			return Invalid
		}
		st = *current
		return OK
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

// Reap releases abandoned process leases. Only the trusted transport supplies
// liveness; the domain has no process, Linux or filesystem dependencies.
func (s *Service) Reap(alive func(string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for owner := range s.issued {
		if !alive(owner) {
			delete(s.issued, owner)
		}
	}
	for id, v := range s.sessions {
		if !s.now().Before(v.Expires) || !alive(v.Owner) {
			delete(s.sessions, id)
			delete(s.pendingRecovery, id)
		}
	}
}

// One reusable session per authenticated owner supports repeated client objects
// without evicting other live windows. The overall bound remains 32 owners.
func (s *Service) ownerSession(owner string) string {
	for id, v := range s.sessions {
		if v.Owner == owner {
			return id
		}
	}
	return ""
}

func ValidCompanyID(id string) bool {
	if len(id) != 36 {
		return false
	}
	for i, r := range id {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if r != '-' {
				return false
			}
		} else if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

func (s *Service) renewRecovery(ctx context.Context, uid uint32, q Command) (View, Code) {
	if !ValidPassword(q.Password) {
		return View{}, Invalid
	}
	select {
	case s.kdf <- struct{}{}:
		defer func() { <-s.kdf }()
	default:
		return View{}, Busy
	}
	s.mu.Lock()
	st, c := s.store.Read(ctx)
	if c == OK {
		c = s.authorized(st, uid, q)
	}
	// Any unacknowledged issuance blocks other sessions. The issuing session may
	// explicitly reauthenticate to replace an uncertain/lost response.
	if c == OK {
		for owner := range s.pendingRecovery {
			if owner != q.Session || !q.Replace {
				c = Busy
			}
		}
	}
	s.mu.Unlock()
	if c != OK {
		return View{}, c
	}
	if c = s.reserve(ctx); c != OK {
		return View{}, c
	}
	if !s.crypto.Verify(q.Password, st.Account.Password) {
		return View{}, Unauthorized
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
		if current.Account.Epoch != st.Account.Epoch || !s.crypto.Equal(current.Account.Password.Hash, st.Account.Password.Hash) || !s.crypto.Equal(current.Account.Recovery, st.Account.Recovery) {
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
	s.pendingRecovery[q.Session] = true
	v := s.view(st)
	v.Authenticated = true
	v.Recovery = recovery
	return v, OK
}
