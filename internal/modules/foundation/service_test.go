package foundation

import (
	"context"
	"sync"
	"testing"
	"time"
)

type controlledSource struct {
	entered chan struct{}
	release chan struct{}
}

func (s controlledSource) Read(ctx context.Context, _ string) (int, Code) {
	s.entered <- struct{}{}
	select {
	case <-s.release:
		return 173, OK
	case <-ctx.Done():
		return 0, Deadline
	}
}
func TestAuthorizationTTLAndSessionBound(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	s := New(1000, "i", "a", func() time.Time { return now }, nil)
	if _, c := s.Open(1001, Profile, "x"); c != Denied {
		t.Fatal(c)
	}
	if _, c := s.Open(1000, "other", "x"); c != ProfileChanged {
		t.Fatal(c)
	}
	v, c := s.Open(1000, Profile, "x")
	if c != OK || !v.Expires.Equal(now.Add(15*time.Minute)) {
		t.Fatal(v, c)
	}
	scope := Scope{"x", 1000, Profile, "a"}
	for _, tc := range []struct {
		x    Scope
		want Code
	}{{Scope{"", 1000, Profile, "a"}, Unauthenticated}, {Scope{"wrong", 1000, Profile, "a"}, Unauthenticated}, {Scope{"x", 1001, Profile, "a"}, Denied}, {Scope{"x", 1000, "other", "a"}, ProfileChanged}, {Scope{"x", 1000, Profile, "old"}, ProfileChanged}, {scope, OK}} {
		if _, got := s.Snapshot(tc.x); got != tc.want {
			t.Fatalf("%+v: %s", tc.x, got)
		}
	}
	for i := 1; i < 32; i++ {
		if _, c = s.Open(1000, Profile, time.Duration(i).String()); c != OK {
			t.Fatal(c)
		}
	}
	if _, c = s.Open(1000, Profile, "overflow"); c != Busy {
		t.Fatal(c)
	}
	now = now.Add(TTL)
	if _, c = s.Snapshot(scope); c != Unauthenticated {
		t.Fatal(c)
	}
	if _, c = s.Open(1000, Profile, "new"); c != OK {
		t.Fatal(c)
	}
}
func TestLabelRules(t *testing.T) {
	for _, tc := range []struct {
		s  string
		ok bool
	}{{"L01", true}, {"Rádio 2_A-B", true}, {"", false}, {"/tmp/x", false}, {"https://x", false}, {"a;ls", false}, {" a", false}, {string([]byte{255}), false}, {"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false}} {
		if ValidLabel(tc.s) != tc.ok {
			t.Fatal(tc.s)
		}
	}
}
func TestConcurrencyStatusAndCancellation(t *testing.T) {
	source := controlledSource{make(chan struct{}, 2), make(chan struct{})}
	s := New(1, "i", "a", time.Now, source)
	s.Open(1, Profile, "x")
	scope := Scope{"x", 1, Profile, "a"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	codes := make(chan Code, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, c := s.Observe(ctx, scope, "L01"); codes <- c }()
	}
	<-source.entered
	<-source.entered
	if _, c := s.Observe(ctx, scope, "L01"); c != Busy {
		t.Fatal(c)
	}
	status := make(chan Status, 1)
	go func() { v, _ := s.Snapshot(scope); status <- v }()
	select {
	case v := <-status:
		if v.Observations != 0 {
			t.Fatal(v)
		}
	case <-time.After(time.Second):
		t.Fatal("status blocked behind source")
	}
	cancel()
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != Deadline {
			t.Fatal(c)
		}
	}
}
