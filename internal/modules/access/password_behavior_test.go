package access

import (
	"context"
	"strings"
	"testing"
	"time"
)

type passwordCrypto struct {
	sessionCrypto
	verifies int
}

func passwordPolicyCases() []struct {
	v  string
	ok bool
} {
	return []struct {
		v  string
		ok bool
	}{
		{strings.Repeat("á", 8), true}, {strings.Repeat("界", 128), true},
		{strings.Repeat("a", 7), false}, {strings.Repeat("a", 129), false},
		{"  espaços valem  ", true}, {string([]byte{0xff}) + strings.Repeat("a", 20), false},
	}
}

func (c *passwordCrypto) Hash(p string) (Verifier, Code)   { return Verifier{Hash: []byte(p)}, OK }
func (c *passwordCrypto) Verify(p string, v Verifier) bool { c.verifies++; return p == string(v.Hash) }
func (*passwordCrypto) Digest(s string) []byte             { return []byte(s) }
func (*passwordCrypto) Equal(a, b []byte) bool             { return string(a) == string(b) }

func TestPasswordBoundariesThroughAccountOperations(t *testing.T) {
	for _, op := range []string{"access.register", "access.recover"} {
		for _, tc := range []struct {
			password string
			want     Code
		}{{"12345678", OK}, {"1234567", Invalid}} {
			t.Run(op+"/"+tc.password, func(t *testing.T) {
				store := &sessionStore{}
				if op == "access.recover" {
					store.state.Account = &Account{Name: "user", Recovery: []byte("recovery"), Epoch: 1}
				}
				s := New(store, &passwordCrypto{}, time.Now, 1000, "activation")
				q := Command{Op: op, Name: "user", Password: tc.password, Recovery: "recovery", Owner: "owner"}
				_, got := s.Handle(context.Background(), 1000, q)
				if got != tc.want {
					t.Fatalf("got %s want %s", got, tc.want)
				}
			})
		}
	}
}

func TestExistingShortPasswordLoginAndRenewal(t *testing.T) {
	for _, tc := range []struct {
		name, password string
		want           Code
		verify         bool
	}{
		{"wrong short", "bad", Unauthorized, true}, {"wrong long", strings.Repeat("a", 129), Unauthorized, false},
		{"empty", "", Unauthorized, false},
		{"invalid utf8", string([]byte{0xff}), Unauthorized, false}, {"correct existing", "old", OK, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			crypto := &passwordCrypto{}
			store := &sessionStore{State{Account: &Account{Name: "user", Password: Verifier{Hash: []byte("old")}, Epoch: 1}}}
			s := New(store, crypto, func() time.Time { return now }, 1000, "activation")
			s.sessions["session"] = session{Owner: "owner", UID: 1000, Epoch: 1, Expires: now.Add(SessionTTL)}
			q := Command{Op: "access.renew-recovery", Password: tc.password, Session: "session", Owner: "owner", Activation: "activation"}
			_, got := s.Handle(context.Background(), 1000, q)
			if got != tc.want || (crypto.verifies > 0) != tc.verify {
				t.Fatalf("got %s verify=%d", got, crypto.verifies)
			}
		})
	}
	t.Run("login", func(t *testing.T) {
		store := &sessionStore{State{Account: &Account{Name: "user", Password: Verifier{Hash: []byte("old")}, Epoch: 1}}}
		s := New(store, &passwordCrypto{}, time.Now, 1000, "activation")
		_, got := s.Handle(context.Background(), 1000, Command{Op: "access.login", Name: "user", Password: "old", Owner: "owner"})
		if got != OK {
			t.Fatalf("existing password login: %s", got)
		}
	})
}
