package ipc

import (
	"context"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type postCommitDelay struct {
	access.Store
	armed     atomic.Bool
	committed chan struct{}
}

func (s *postCommitDelay) Update(ctx context.Context, fn func(*access.State) access.Code) access.Code {
	code := s.Store.Update(ctx, fn)
	if code == access.OK && s.armed.CompareAndSwap(true, false) {
		close(s.committed)
		<-ctx.Done()
	}
	return code
}

func TestResetDeadlineAfterCommitIsIndeterminate(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runtime", "state", "client"} {
		if err := os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	db, err := storage.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	store := &postCommitDelay{Store: db, committed: make(chan struct{})}
	uid := uint32(os.Getuid())
	account := access.New(store, security.Crypto{}, time.Now, uid, "activation", db)
	server, err := ListenAccess(filepath.Join(root, "runtime"), uid, account, foundation.Status{Instance: "fixture"}, db.Close)
	if err != nil {
		t.Fatal(err)
	}
	proof := &ipcResetFixture{}
	account.SetResetProof(proof)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	client := NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid)
	registration := client.Call(context.Background(), Request{Op: "access.register", Name: "old", Password: "synthetic old password"})
	if registration.Code != foundation.OK {
		t.Fatal("fixture registration", registration.Code)
	}
	challenge := client.Call(context.Background(), Request{Op: "access.reset-begin"})
	if challenge.Code != foundation.OK {
		t.Fatal("fixture challenge", challenge.Code)
	}
	proof.nonce = challenge.Data.ResetNonce
	store.armed.Store(true)
	result := client.Call(context.Background(), Request{
		Op: "access.reset-complete", Name: "new", Password: "synthetic new password", ResetNonce: proof.nonce,
	})
	<-store.committed
	state, code := db.Read(context.Background())
	if code != access.OK {
		t.Fatal("fixture read", code)
	}
	if result.Code != foundation.Deadline || state.Account.Name != "new" {
		t.Fatalf("expected indeterminate committed reset, got %s and %q", result.Code, state.Account.Name)
	}
}
