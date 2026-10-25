// Package foundation owns the synthetic use case and ephemeral authorization policy.
package foundation

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
	OK              Code = "OK"
	Invalid         Code = "INVALID_REQUEST"
	Version         Code = "VERSION_INCOMPATIBLE"
	Unauthenticated Code = "UNAUTHENTICATED"
	Denied          Code = "PERMISSION_DENIED"
	ProfileChanged  Code = "PROFILE_CHANGED"
	Deadline        Code = "DEADLINE_EXCEEDED"
	Busy            Code = "BUSY"
	Unavailable     Code = "SERVICE_UNAVAILABLE"
	Profile              = "synthetic-l01"
	TTL                  = 15 * time.Minute
)

type Source interface {
	Read(context.Context, string) (int, Code)
}
type Session struct {
	ID      string
	UID     uint32
	Expires time.Time
}
type Scope struct {
	Session    string
	UID        uint32
	Profile    string
	Activation string
}
type Status struct {
	Instance     string
	Activation   string
	Heartbeat    int64
	Observations uint64
}
type Observation struct {
	Value int
	At    time.Time
}
type Service struct {
	mu                   sync.Mutex
	sessions             map[string]Session
	uid                  uint32
	instance, activation string
	started              time.Time
	now                  func() time.Time
	source               Source
	slots                chan struct{}
	observations         uint64
}

func New(uid uint32, instance, activation string, now func() time.Time, source Source) *Service {
	return &Service{uid: uid, instance: instance, activation: activation, started: now(), now: now, source: source, sessions: make(map[string]Session), slots: make(chan struct{}, 2)}
}
func (s *Service) Open(uid uint32, profile, id string) (Session, Code) {
	if uid != s.uid {
		return Session{}, Denied
	}
	if profile != Profile {
		return Session{}, ProfileChanged
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for k, v := range s.sessions {
		if !now.Before(v.Expires) {
			delete(s.sessions, k)
		}
	}
	if len(s.sessions) >= 32 {
		return Session{}, Busy
	}
	v := Session{ID: id, UID: uid, Expires: now.Add(TTL)}
	s.sessions[id] = v
	return v, OK
}
func (s *Service) authorize(x Scope) Code {
	if x.UID != s.uid {
		return Denied
	}
	s.mu.Lock()
	v, ok := s.sessions[x.Session]
	s.mu.Unlock()
	if !ok || v.UID != x.UID || !s.now().Before(v.Expires) {
		return Unauthenticated
	}
	if x.Profile != Profile || x.Activation != s.activation {
		return ProfileChanged
	}
	return OK
}
func (s *Service) Snapshot(x Scope) (Status, Code) {
	if c := s.authorize(x); c != OK {
		return Status{}, c
	}
	s.mu.Lock()
	n := s.observations
	s.mu.Unlock()
	return Status{Instance: s.instance, Activation: s.activation, Heartbeat: int64(s.now().Sub(s.started) / time.Second), Observations: n}, OK
}
func ValidLabel(label string) bool {
	n := utf8.RuneCountInString(label)
	if !utf8.ValidString(label) || n < 1 || n > 64 || strings.TrimSpace(label) != label {
		return false
	}
	for _, r := range label {
		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != ' ' && r != '_' && r != '-' {
			return false
		}
	}
	return true
}
func (s *Service) Observe(ctx context.Context, x Scope, label string) (Observation, Code) {
	if c := s.authorize(x); c != OK {
		return Observation{}, c
	}
	if !ValidLabel(label) {
		return Observation{}, Invalid
	}
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	default:
		return Observation{}, Busy
	}
	if ctx.Err() != nil {
		return Observation{}, Deadline
	}
	v, c := s.source.Read(ctx, label)
	if c != OK {
		return Observation{}, c
	}
	if ctx.Err() != nil {
		return Observation{}, Deadline
	}
	s.mu.Lock()
	s.observations++
	s.mu.Unlock()
	return Observation{Value: v, At: s.now().UTC()}, OK
}
