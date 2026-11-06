package presentation

import (
	"context"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type uncertainStore struct {
	access.Store
	armed     atomic.Bool
	committed chan struct{}
	stop      func()
}

func (s *uncertainStore) Update(ctx context.Context, fn func(*access.State) access.Code) access.Code {
	code := s.Store.Update(ctx, fn)
	if code == access.OK && s.armed.CompareAndSwap(true, false) {
		close(s.committed)
		<-ctx.Done()
	}
	return code
}
func uncertainResetFixture(t *testing.T) (*App, *bindingResetProof, *uncertainStore, string, uint32) {
	t.Helper()
	root, err := os.MkdirTemp("", "reset-uncertain-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	for _, name := range []string{"runtime", "state", "client"} {
		if err = os.Mkdir(filepath.Join(root, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	db, err := storage.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	store := &uncertainStore{Store: db, committed: make(chan struct{})}
	uid := uint32(os.Getuid())
	account := access.New(store, security.Crypto{}, time.Now, uid, "activation", db)
	server, err := ipc.ListenAccess(filepath.Join(root, "runtime"), uid, account, foundation.Status{Instance: "fixture"}, db.Close)
	if err != nil {
		t.Fatal(err)
	}
	proof := &bindingResetProof{}
	account.SetResetProof(proof)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	var stopOnce sync.Once
	store.stop = func() {
		stopOnce.Do(func() {
			cancel()
			if err := <-done; err != nil {
				t.Error(err)
			}
		})
	}
	t.Cleanup(store.stop)
	app := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	app.resetUnixUser = func() (string, error) { return "synthetic", nil }
	app.resetAuthorize = func(_ context.Context, _ string, nonce string) string { proof.nonce = nonce; return "OK" }
	return app, proof, store, root, uid
}
func TestUncertainResetRequiresExplicitLoginAcrossAppRestart(t *testing.T) {
	app, _, store, root, uid := uncertainResetFixture(t)
	if app.Register("old", "synthetic old password").Code != "OK" || app.AckRecovery().Code != "OK" {
		t.Fatal("fixture setup")
	}
	store.armed.Store(true)
	if r := app.ResetAccount("new", "synthetic new password"); r.Code != "RESET_OUTCOME_UNKNOWN" {
		t.Fatal("outcome not indeterminate", r.Code)
	}
	<-store.committed
	marker, err := os.ReadFile(filepath.Join(root, "client", "reset-verification-required"))
	if err != nil || string(marker) != "verification-required\n" {
		t.Fatal("marker contains unexpected data", err)
	}
	reopened := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	reopened.resetUnixUser = func() (string, error) { return "synthetic", nil }
	authCalls := 0
	reopened.resetAuthorize = func(context.Context, string, string) string { authCalls++; return "OK" }
	if r := reopened.ResetAccount("again", "another synthetic password"); r.Code != "RESET_VERIFICATION_REQUIRED" || authCalls != 0 {
		t.Fatal("retry not blocked before verification", r.Code)
	}
	if r := reopened.Login("new", "synthetic new password", false); r.Code != "OK" {
		t.Fatal("explicit verification failed", r.Code)
	}
	if pending, err := reopened.client.ResetIntentPending(); err != nil || pending {
		t.Fatal("verified marker retained", err)
	}
	if r := reopened.RenewRecovery("synthetic new password", false); r.Code != "OK" || r.Recovery == "" {
		t.Fatal("new code unavailable after verified login", r.Code)
	}
	if reopened.AckRecovery().Code != "OK" {
		t.Fatal("acknowledgement failed")
	}
}

func TestFailedExplicitLoginAllowsOnlyDeliberateNewAttempt(t *testing.T) {
	app, _, store, root, uid := uncertainResetFixture(t)
	if app.Register("old", "synthetic old password").Code != "OK" || app.AckRecovery().Code != "OK" {
		t.Fatal("fixture setup")
	}
	store.armed.Store(true)
	if r := app.ResetAccount("new", "synthetic new password"); r.Code != "RESET_OUTCOME_UNKNOWN" {
		t.Fatal(r.Code)
	}
	reopened := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	reopened.resetUnixUser = func() (string, error) { return "synthetic", nil }
	called := 0
	reopened.resetAuthorize = func(context.Context, string, string) string { called++; return "ADMIN_AUTH_CANCELLED" }
	if r := reopened.Login("old", "synthetic old password", false); r.Code != "UNAUTHENTICATED" {
		t.Fatal("expected observed failed login", r.Code)
	}
	if called != 0 {
		t.Fatal("verification retried reset automatically")
	}
	if r := reopened.ResetAccount("again", "another synthetic password"); r.Code != "ADMIN_AUTH_CANCELLED" || called != 1 {
		t.Fatal("deliberate retry not available after verification", r.Code)
	}
}
