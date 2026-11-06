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
	"testing"
	"time"
)

type bindingResetProof struct{ nonce string }

func (p *bindingResetProof) Consume(v string) bool { return p.nonce != "" && p.nonce == v }

func resetBindingFixture(t *testing.T) (*App, *bindingResetProof, *time.Time) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"runtime", "state", "client"} {
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
	server, err := ipc.ListenAccess(filepath.Join(root, "runtime"), uid, account, foundation.Status{Instance: "fixture"}, store.Close)
	if err != nil {
		t.Fatal(err)
	}
	proof := &bindingResetProof{}
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
	app := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	app.resetUnixUser = func() (string, error) { return "synthetic", nil }
	return app, proof, &now
}
func TestResetBindingDeliversCodeAndProtectsItUntilAcknowledged(t *testing.T) {
	app, proof, clock := resetBindingFixture(t)
	first := app.Register("technical", "synthetic old password")
	if first.Code != "OK" || app.AckRecovery().Code != "OK" {
		t.Fatal("fixture setup")
	}
	app.resetAuthorize = func(_ context.Context, user, nonce string) string {
		if user != "synthetic" || len(nonce) != 64 {
			t.Fatal("unexpected helper arguments")
		}
		proof.nonce = nonce
		return "OK"
	}
	result := app.ResetAccount("new-user", "synthetic new password")
	if result.Code != "OK" || result.Recovery == "" || result.Recovery == first.Recovery {
		t.Fatal("reset result", result.Code)
	}
	if !PreventClose(app) {
		t.Fatal("issued recovery was not protected")
	}
	var copied string
	app.clipboard = func(_ context.Context, value string) error { copied = value; return nil }
	if app.CopyRecovery() != "OK" || copied != result.Recovery {
		t.Fatal("issued recovery not available for copy")
	}
	if app.AckRecovery().Code != "OK" || PreventClose(app) {
		t.Fatal("ack did not release code")
	}
	*clock = clock.Add(31 * time.Second)
	if app.Login("technical", "synthetic old password", false).Code != "UNAUTHENTICATED" {
		t.Fatal("old password accepted")
	}
	*clock = clock.Add(31 * time.Second)
	if app.Login("new-user", "synthetic new password", false).Code != "OK" {
		t.Fatal("new password refused")
	}
}
