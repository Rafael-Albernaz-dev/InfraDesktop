package access

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type resetCrypto struct{ next int }

func (c *resetCrypto) Token() (string, Code)        { c.next++; return fmt.Sprintf("%064x", c.next), OK }
func (*resetCrypto) Hash(p string) (Verifier, Code) { return Verifier{Hash: []byte(p)}, OK }
func (*resetCrypto) Verify(p string, v Verifier) bool {
	return p == string(v.Hash)
}
func (*resetCrypto) Digest(v string) []byte { return []byte(v) }
func (*resetCrypto) Equal(a, b []byte) bool { return string(a) == string(b) }

type resetProofFixture struct {
	approved string
	calls    int
}

func (p *resetProofFixture) Consume(v string) bool {
	p.calls++
	return v == p.approved
}

type resetStore struct {
	state State
	fail  bool
}

func (s *resetStore) Read(context.Context) (State, Code) { return s.state, OK }
func (s *resetStore) Update(_ context.Context, f func(*State) Code) Code {
	if s.fail {
		return Unavailable
	}
	state := s.state
	if state.Account != nil {
		copy := *state.Account
		state.Account = &copy
	}
	if c := f(&state); c != OK {
		return c
	}
	s.state = state
	return OK
}
func resetFixture() (*Service, *resetStore, *resetProofFixture, *time.Time) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	st := &resetStore{state: State{
		Account: &Account{Name: "old", Password: Verifier{Hash: []byte("old fixture")},
			Recovery: []byte("old code"), Epoch: 4, Profile: "company", Theme: "dark"},
		Companies: []Company{{ID: "company", Name: "Fixture"}},
		Grants:    []Grant{{Digest: []byte("fixture-grant"), Epoch: 4, Expires: now.Add(time.Hour)}},
	}}
	svc := New(st, &resetCrypto{}, func() time.Time { return now }, 1000, "activation")
	proof := &resetProofFixture{}
	svc.SetResetProof(proof)
	svc.sessions["session"] = session{Owner: "window", UID: 1000, Epoch: 4, Expires: now.Add(time.Hour)}
	return svc, st, proof, &now
}
func beginReset(t *testing.T, s *Service, p *resetProofFixture) string {
	t.Helper()
	v, c := s.Handle(context.Background(), 1000, Command{Op: "access.reset-begin", Owner: "requester"})
	if c != OK || len(v.ResetNonce) != 64 {
		t.Fatalf("begin: %s", c)
	}
	p.approved = v.ResetNonce
	return v.ResetNonce
}
func completeReset(s *Service, owner, nonce string) (View, Code) {
	return s.Handle(context.Background(), 1000, Command{
		Op: "access.reset-complete", Owner: owner, Name: "new", Password: "new fixture", ResetNonce: nonce,
	})
}
func TestResetRequiresLiveOwnerNonceAndConsumesIt(t *testing.T) {
	for _, scenario := range []string{"wrong", "expired", "owner", "proof", "store"} {
		t.Run(scenario, func(t *testing.T) {
			svc, store, proof, now := resetFixture()
			nonce := beginReset(t, svc, proof)
			originalNonce := nonce
			owner := "requester"
			switch scenario {
			case "wrong":
				nonce = fmt.Sprintf("%064x", 999)
				proof.approved = nonce
			case "expired":
				*now = now.Add(2 * time.Minute)
			case "owner":
				owner = "other"
			case "proof":
				proof.approved = "not-approved"
			case "store":
				store.fail = true
			}
			_, code := completeReset(svc, owner, nonce)
			if code == OK {
				t.Fatal("reset unexpectedly accepted")
			}
			if store.state.Account.Name != "old" || store.state.Account.Epoch != 4 || string(store.state.Account.Recovery) != "old code" {
				t.Fatal("failed reset changed account")
			}
			if scenario == "proof" || scenario == "store" {
				proof.approved = originalNonce
				store.fail = false
				if _, c := completeReset(svc, "requester", originalNonce); c == OK {
					t.Fatal("consumed nonce reused")
				}
			}
		})
	}
}
func TestResetPreservesAccountScopeAndRevokesSessions(t *testing.T) {
	svc, store, proof, _ := resetFixture()
	nonce := beginReset(t, svc, proof)
	v, c := completeReset(svc, "requester", nonce)
	if c != OK || v.Recovery == "" || v.Recovery == "old code" {
		t.Fatalf("reset result %s", c)
	}
	a := store.state.Account
	if a.Name != "new" || a.Profile != "company" || a.Theme != "dark" || a.Epoch != 5 ||
		!svc.crypto.Verify("new fixture", a.Password) || len(store.state.Companies) != 1 {
		t.Fatal("reset did not preserve account scope")
	}
	if string(a.Recovery) == "old code" || len(store.state.Grants) != 0 {
		t.Fatal("old recovery or grants retained")
	}
	if _, c = svc.Handle(context.Background(), 1000, Command{
		Op: "access.snapshot", Owner: "window", Session: "session", Profile: "company", Activation: "activation",
	}); c != Unauthorized {
		t.Fatal("old session survived", c)
	}
	if _, c = completeReset(svc, "requester", nonce); c != Unauthorized {
		t.Fatal("replay accepted", c)
	}
	if _, c = svc.Handle(context.Background(), 1000, Command{
		Op: "access.recover", Owner: "requester", Name: "new", Password: "new fixture", Recovery: "old code",
	}); c == OK {
		t.Fatal("old recovery accepted")
	}
}
func TestResetConcurrentCompletionHasOneWinner(t *testing.T) {
	svc, _, proof, _ := resetFixture()
	nonce := beginReset(t, svc, proof)
	var wg sync.WaitGroup
	results := make(chan Code, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, c := completeReset(svc, "requester", nonce); results <- c }()
	}
	wg.Wait()
	close(results)
	success := 0
	for c := range results {
		if c == OK {
			success++
		}
	}
	if success != 1 || proof.calls != 1 {
		t.Fatalf("winners=%d proof calls=%d", success, proof.calls)
	}
}

func TestResetRejectsInvalidNewAccountBeforeConsumingProof(t *testing.T) {
	for _, tc := range []struct{ name, password string }{{"", "valid fixture"}, {"new", "short"}} {
		svc, store, proof, _ := resetFixture()
		nonce := beginReset(t, svc, proof)
		_, code := svc.Handle(context.Background(), 1000, Command{
			Op: "access.reset-complete", Owner: "requester", Name: tc.name, Password: tc.password, ResetNonce: nonce,
		})
		if code != Invalid || proof.calls != 0 || store.state.Account.Name != "old" {
			t.Fatal("invalid account accepted or proof consumed", code)
		}
	}
}

func TestResetBeginReplacesChallengeAndBoundsOwners(t *testing.T) {
	svc, _, proof, _ := resetFixture()
	first := beginReset(t, svc, proof)
	second := beginReset(t, svc, proof)
	if first == second {
		t.Fatal("challenge was reused")
	}
	proof.approved = first
	if _, code := completeReset(svc, "requester", first); code == OK {
		t.Fatal("superseded challenge accepted")
	}
	for i := 0; i < 31; i++ {
		_, code := svc.Handle(context.Background(), 1000, Command{Op: "access.reset-begin", Owner: fmt.Sprintf("owner-%d", i)})
		if code != OK {
			t.Fatal("bounded owner rejected", code)
		}
	}
	if _, code := svc.Handle(context.Background(), 1000, Command{Op: "access.reset-begin", Owner: "overflow"}); code != Busy {
		t.Fatal("challenge limit not enforced", code)
	}
	if _, code := svc.Handle(context.Background(), 1000, Command{Op: "access.reset-begin"}); code != Unauthorized {
		t.Fatal("missing owner accepted", code)
	}
}

func TestResetBeginRequiresProofAndExistingAccount(t *testing.T) {
	svc, store, _, _ := resetFixture()
	svc.SetResetProof(nil)
	if _, code := svc.Handle(context.Background(), 1000, Command{Op: "access.reset-begin", Owner: "requester"}); code != Unavailable {
		t.Fatal("proof unavailable was accepted", code)
	}
	svc.SetResetProof(&resetProofFixture{})
	store.state.Account = nil
	if _, code := svc.Handle(context.Background(), 1000, Command{Op: "access.reset-begin", Owner: "requester"}); code != Unauthorized {
		t.Fatal("missing account was accepted", code)
	}
}

func TestResetRejectsChallengeIssuedForOlderAccountEpoch(t *testing.T) {
	svc, store, proof, _ := resetFixture()
	nonce := beginReset(t, svc, proof)
	store.state.Account.Epoch++
	store.state.Account.Name = "changed-by-another-recovery"
	if _, code := completeReset(svc, "requester", nonce); code != Unauthorized {
		t.Fatal("stale authorization changed a newer account", code)
	}
	if store.state.Account.Name != "changed-by-another-recovery" || store.state.Account.Epoch != 5 {
		t.Fatal("newer account overwritten")
	}
}
