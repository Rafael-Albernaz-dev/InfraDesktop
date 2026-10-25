package access

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// These units use the port's in-memory implementation; authorization and
// session allocation are the real service methods, with normal TTLs.
type sessionStore struct{ state State }

func (s *sessionStore) Read(context.Context) (State, Code) { return s.state, OK }
func (s *sessionStore) Update(_ context.Context, update func(*State) Code) Code {
	return update(&s.state)
}

type sessionCrypto struct {
	Crypto
	next int
}

func (c *sessionCrypto) Token() (string, Code) { c.next++; return fmt.Sprintf("%064d", c.next), OK }

func TestSessionOwnerAuthorization(t *testing.T) {
	now := time.Now()
	state := State{Account: &Account{Epoch: 3, Profile: "company", Theme: "light"}}
	service := New(&sessionStore{state}, &sessionCrypto{}, func() time.Time { return now }, 1000, "activation")
	service.sessions["session"] = session{Owner: "123:456", UID: 1000, Epoch: 3, Expires: now.Add(SessionTTL)}
	good := Command{Op: "access.snapshot", Session: "session", Owner: "123:456", Profile: "company", Activation: "activation"}
	for _, test := range []struct {
		name   string
		change func(*Command)
		uid    uint32
		want   Code
	}{
		{"authorized", func(*Command) {}, 1000, OK},
		{"different owner", func(q *Command) { q.Owner = "124:456" }, 1000, Unauthorized},
		{"reused PID", func(q *Command) { q.Owner = "123:457" }, 1000, Unauthorized},
		{"missing owner", func(q *Command) { q.Owner = "" }, 1000, Unauthorized},
		{"wrong session", func(q *Command) { q.Session = "other" }, 1000, Unauthorized},
		{"wrong UID", func(*Command) {}, 1001, Unauthorized},
		{"profile", func(q *Command) { q.Profile = "other" }, 1000, Changed},
		{"activation", func(q *Command) { q.Activation = "old" }, 1000, Changed},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := good
			test.change(&q)
			_, code := service.Handle(context.Background(), test.uid, q)
			if code != test.want {
				t.Fatalf("authorization: got %s want %s", code, test.want)
			}
		})
	}
	state.Account.Epoch++
	if _, code := service.Handle(context.Background(), 1000, good); code != Unauthorized {
		t.Fatal("old epoch accepted")
	}
	state.Account.Epoch--
	now = now.Add(SessionTTL)
	if _, code := service.Handle(context.Background(), 1000, good); code != Unauthorized {
		t.Fatal("expired session accepted")
	}
}

func TestSessionLimitAndOwnerReuse(t *testing.T) {
	now := time.Now()
	store := &sessionStore{State{Account: &Account{Epoch: 1, Theme: "light"}}}
	service := New(store, &sessionCrypto{}, func() time.Time { return now }, 1000, "activation")
	ctx := context.Background()
	for i := 0; i < 32; i++ {
		if _, code := service.open(ctx, 1000, fmt.Sprint(i), false, nil); code != OK {
			t.Fatalf("owner %d: %s", i, code)
		}
	}
	if _, code := service.open(ctx, 1000, "33rd", false, nil); code != Busy {
		t.Fatalf("33rd: %s", code)
	}
	old := service.ownerSession("0")
	view, code := service.open(ctx, 1000, "0", false, nil)
	if code != OK || view.Session != old || len(service.sessions) != 32 {
		t.Fatal("existing owner cannot reuse at limit")
	}
	service.Reap(func(owner string) bool { return owner != "1" })
	if len(service.sessions) != 31 {
		t.Fatal("dead owner not reaped")
	}
	if _, code := service.open(ctx, 1000, "33rd", false, nil); code != OK {
		t.Fatalf("reaped slot not reusable: %s", code)
	}
	// Grant-based resume has the same ceiling; grants and sessions are distinct.
	grant := "0000000000000000000000000000000000000000000000000000000000000000"
	store.state.Grants = []Grant{{Digest: []byte(grant), Epoch: 1, Expires: now.Add(GrantTTL)}}
	service.crypto = &resumeCrypto{sessionCrypto: sessionCrypto{next: 100}}
	if _, code := service.Handle(ctx, 1000, Command{Op: "access.resume", Grant: grant, Owner: "34th"}); code != Busy {
		t.Fatalf("resume at limit: %s", code)
	}
	if view, code := service.Handle(ctx, 1000, Command{Op: "access.resume", Grant: grant, Owner: "0"}); code != OK || view.Session != old {
		t.Fatalf("resume same owner: %s", code)
	}
}

type resumeCrypto struct{ sessionCrypto }

func (*resumeCrypto) Digest(value string) []byte { return []byte(value) }
func (*resumeCrypto) Equal(a, b []byte) bool     { return string(a) == string(b) }

func TestCompanyCreationRetryAtLimit(t *testing.T) {
	now := time.Now()
	store := &sessionStore{State{Account: &Account{Epoch: 1, Theme: "dark"}}}
	for i := 1; i < 100; i++ {
		store.state.Companies = append(store.state.Companies, Company{fmt.Sprintf("%08x-aaaa-4aaa-aaaa-aaaaaaaaaaaa", i), "Existing"})
	}
	service := New(store, &sessionCrypto{}, func() time.Time { return now }, 1000, "activation")
	service.sessions["session"] = session{Owner: "owner", UID: 1000, Epoch: 1, Expires: now.Add(SessionTTL)}
	q := Command{Op: "company.create", Session: "session", Owner: "owner", Activation: "activation", Operation: "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb", Name: "Hundredth"}
	ctx := context.Background()
	// Commit the 100th creation, then discard its response as a lost reply.
	if _, code := service.Handle(ctx, 1000, q); code != OK {
		t.Fatal("100th creation rejected", code)
	}
	view, code := service.Handle(ctx, 1000, q)
	if code != OK || len(view.Companies) != 100 || view.Companies[99] != (Company{q.Operation, q.Name}) || view.Theme != "dark" || !view.Authenticated {
		t.Fatal("retry at limit did not return the existing catalog", code)
	}
	q.Name = "Different payload"
	if _, code = service.Handle(ctx, 1000, q); code != Invalid {
		t.Fatal("UUID reused with changed name accepted", code)
	}
	q.Operation = "cccccccc-cccc-4ccc-cccc-cccccccccccc"
	q.Name = "Hundredth"
	if _, code = service.Handle(ctx, 1000, q); code != Invalid {
		t.Fatal("new operation exceeded ceiling even with same name", code)
	}
	if len(store.state.Companies) != 100 || store.state.Companies[99].Name != "Hundredth" {
		t.Fatal("rejected request mutated catalog")
	}
}
