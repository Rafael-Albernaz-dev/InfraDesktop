package ipc

import (
	"context"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type ipcResetFixture struct{ nonce string }

func (p *ipcResetFixture) Consume(nonce string) bool { return p.nonce != "" && p.nonce == nonce }

func TestResetJourneyThroughSyntheticIPC(t *testing.T) {
	root, uid, proof, clock := newResetIPCHarness(t)
	client := NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid)
	call := func(c *AccountClient, q Request, want foundation.Code) Response {
		t.Helper()
		r := c.Call(context.Background(), q)
		if r.Code != want {
			t.Fatalf("%s: got %s want %s", q.Op, r.Code, want)
		}
		return r
	}
	reg := call(client, Request{Op: "access.register", Name: "technical", Password: "synthetic old password"}, foundation.OK)
	call(client, Request{Op: "access.ack-issued", Recovery: reg.Data.Recovery}, foundation.OK)
	*clock = clock.Add(31 * time.Second)
	call(client, Request{Op: "access.login", Name: "technical", Password: "synthetic old password", Remember: true}, foundation.OK)
	company := call(client, Request{Op: "company.create", Name: "Synthetic company"}, foundation.OK).Data.Companies[0].ID
	call(client, Request{Op: "company.select", Name: company}, foundation.OK)
	second := NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "other"), uid)
	*clock = clock.Add(31 * time.Second)
	call(second, Request{Op: "access.login", Name: "technical", Password: "synthetic old password"}, foundation.OK)
	first := call(client, Request{Op: "access.reset-begin"}, foundation.OK).Data.ResetNonce
	call(client, resetIPCRequest(first), foundation.Unauthenticated)
	begin := call(client, Request{Op: "access.reset-begin"}, foundation.OK)
	proof.nonce = begin.Data.ResetNonce
	issued := call(client, resetIPCRequest(proof.nonce), foundation.OK)
	if issued.Data.Recovery == "" || issued.Data.Recovery == reg.Data.Recovery {
		t.Fatal("new code missing")
	}
	call(second, Request{Op: "access.snapshot"}, foundation.Unauthenticated)
	call(client, resetIPCRequest(proof.nonce), foundation.Unauthenticated)
	call(client, Request{Op: "access.check-issued", Recovery: issued.Data.Recovery}, foundation.OK)
	call(client, Request{Op: "access.ack-issued", Recovery: issued.Data.Recovery}, foundation.OK)
	*clock = clock.Add(31 * time.Second)
	login := call(client, Request{Op: "access.login", Name: "new-user", Password: "synthetic new password"}, foundation.OK)
	if login.Data.Profile != company || len(login.Data.Companies) != 1 {
		t.Fatal("company scope lost")
	}
	*clock = clock.Add(31 * time.Second)
	call(second, Request{Op: "access.login", Name: "technical", Password: "synthetic old password"}, foundation.Unauthenticated)
	*clock = clock.Add(31 * time.Second)
	call(second, Request{
		Op: "access.recover", Name: "new-user", Password: "another synthetic password", Recovery: reg.Data.Recovery,
	}, foundation.Unauthenticated)
}

func resetIPCRequest(nonce string) Request {
	return Request{Op: "access.reset-complete", Name: "new-user", Password: "synthetic new password", ResetNonce: nonce}
}
func newResetIPCHarness(t *testing.T) (string, uint32, *ipcResetFixture, *time.Time) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runtime", "state", "client", "other"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	store, err := storage.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	uid := uint32(os.Getuid())
	account := access.New(store, security.Crypto{}, func() time.Time { return now }, uid, "activation", store)
	server, err := ListenAccess(filepath.Join(root, "runtime"), uid, account, foundation.Status{Instance: "synthetic"}, store.Close)
	if err != nil {
		t.Fatal(err)
	}
	proof := &ipcResetFixture{}
	account.SetResetProof(proof)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return root, uid, proof, &now
}
