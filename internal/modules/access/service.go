// Package access owns local account, authorization and company policy.
package access

import (
	"context"
	"sync"
	"time"
)

type Code string

const (
	WrongPasswordReason      = "PASSWORD_INCORRECT"
	OK                  Code = "OK"
	Invalid             Code = "INVALID_REQUEST"
	Unauthorized        Code = "UNAUTHENTICATED"
	Changed             Code = "PROFILE_CHANGED"
	Busy                Code = "BUSY"
	Unavailable         Code = "SERVICE_UNAVAILABLE"
	Conflict            Code = "ACCOUNT_EXISTS"
	Limited             Code = "TRY_LATER"
	SessionTTL               = 12 * time.Hour
	GrantTTL                 = 30 * 24 * time.Hour
)

type Verifier struct {
	Algorithm          string
	Memory, Iterations uint32
	Parallelism        uint8
	Salt, Hash         []byte
}
type Account struct {
	Columns        []string
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

// ResetProof consumes a privileged, one-time proof without exposing filesystem details to the domain.
type ResetProof interface{ Consume(string) bool }
type resetChallenge struct {
	nonce   string
	expires time.Time
	epoch   uint64
}

type Command struct {
	Columns                                                                             *[]string
	Op, Session, Grant, Name, Password, Recovery, Profile, Activation, Theme, Operation string
	ExpectedProfile, ExpectedActivation, ResetNonce, CurrentPassword                    string
	Brand                                                                               Branding
	Remember, Replace                                                                   bool
	// Owner is a transport-authenticated opaque process identity, never a wire field.
	Owner string
}
type View struct {
	Columns                                                                             []string
	Setup, Authenticated                                                                bool
	Name, Theme, Profile, Activation, Session, Grant, Recovery, ResetNonce, ErrorReason string
	Companies                                                                           []Company
	Brand                                                                               *Branding
}
type session struct {
	Owner   string
	UID     uint32
	Epoch   uint64
	Expires time.Time
}
type Service struct {
	store                Store
	brands               BrandStore
	issued               map[string][]byte
	crypto               Crypto
	now                  func() time.Time
	uid                  uint32
	kdf                  chan struct{}
	mu                   sync.Mutex
	sessions             map[string]session
	activation           string
	authorizationVersion uint64
	pendingRecovery      map[string]bool
	resetOperationMu     sync.Mutex
	resetProof           ResetProof
	resetChallenges      map[string]resetChallenge
}

func New(store Store, crypto Crypto, now func() time.Time, uid uint32, activation string, brands ...BrandStore) *Service {
	var brandStore BrandStore
	if len(brands) > 0 {
		brandStore = brands[0]
	}
	return &Service{
		store: store, brands: brandStore, issued: map[string][]byte{}, crypto: crypto,
		now: now, uid: uid, kdf: make(chan struct{}, 1), sessions: map[string]session{},
		pendingRecovery: map[string]bool{}, resetChallenges: map[string]resetChallenge{},
		activation: activation, authorizationVersion: 1,
	}
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
		v.Columns = append([]string(nil), st.Account.Columns...)
		v.Profile = st.Account.Profile
		v.Activation = s.activation
		v.Companies = st.Companies
	}
	return v
}
func (s *Service) open(
	ctx context.Context, uid uint32, owner string, remember bool, verified *Account,
) (View, Code) {
	id, grant, c := s.issueOpenTokens(remember)
	if c != OK {
		return View{}, c
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredSessions()
	if len(s.sessions) >= 32 && s.ownerSession(owner) == "" {
		return View{}, Busy
	}
	var state State
	c = s.store.Update(ctx, func(st *State) Code {
		return s.commitOpenSession(st, remember, grant, verified, &state)
	})
	if c != OK {
		return View{}, c
	}
	s.pruneExpiredSessions()
	if len(s.sessions) >= 32 && s.ownerSession(owner) == "" {
		return View{}, Busy
	}
	if existing := s.ownerSession(owner); existing != "" {
		id = existing
	}
	s.sessions[id] = session{
		Owner:   owner,
		UID:     uid,
		Epoch:   state.Account.Epoch,
		Expires: s.now().Add(SessionTTL),
	}
	v := s.view(state)
	v.Authenticated = true
	v.Session = id
	v.Grant = grant
	return v, OK
}

// Reap releases abandoned process leases. Only the trusted transport supplies
// liveness; the domain has no process, Linux or filesystem dependencies.
func (s *Service) Reap(alive func(string) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for owner := range s.resetChallenges {
		if !alive(owner) {
			delete(s.resetChallenges, owner)
		}
	}
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
