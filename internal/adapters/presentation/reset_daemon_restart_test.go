package presentation

import (
	"context"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestUncertainResetRequiresVerificationAfterSyntheticDaemonRestart(t *testing.T) {
	app, _, store, root, uid := uncertainResetFixture(t)
	if app.Register("old", "synthetic old password").Code != "OK" || app.AckRecovery().Code != "OK" {
		t.Fatal("fixture setup")
	}
	store.armed.Store(true)
	if r := app.ResetAccount("new", "synthetic new password"); r.Code != "RESET_OUTCOME_UNKNOWN" {
		t.Fatal(r.Code)
	}
	store.stop()
	db, err := storage.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	account := access.New(db, security.Crypto{}, time.Now, uid, "new-activation", db)
	server, err := ipc.ListenAccess(filepath.Join(root, "runtime"), uid, account, foundation.Status{Instance: "new"}, db.Close)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	reopened := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	reopened.resetUnixUser = func() (string, error) { return "synthetic", nil }
	called := 0
	reopened.resetAuthorize = func(context.Context, string, string) string { called++; return "OK" }
	if r := reopened.ResetAccount("again", "another synthetic password"); r.Code != "RESET_VERIFICATION_REQUIRED" || called != 0 {
		t.Fatal("restart bypassed verification", r.Code)
	}
	var stacks atomic.Value
	captured := make(chan struct{})
	timer := time.AfterFunc(2500*time.Millisecond, func() {
		defer close(captured)
		buf := make([]byte, 2<<20)
		stacks.Store(string(buf[:runtime.Stack(buf, true)]))
	})
	r := reopened.Login("new", "synthetic new password", false)
	if !timer.Stop() {
		<-captured
	}
	if r.Code != "OK" {
		if dump := stacks.Load(); dump != nil {
			t.Log(dump)
		}
		t.Fatal("verification after restart failed", r.Code)
	}
	if pending, err := reopened.client.ResetIntentPending(); err != nil || pending {
		t.Fatal("marker retained after verification", err)
	}
}
