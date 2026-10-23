package presentation

import (
	"context"
	"errors"
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

func newRecoveryApp(t *testing.T, crypto access.Crypto) *App {
	t.Helper()
	root, e := os.MkdirTemp("", "id-copy-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	for _, n := range []string{"state", "runtime", "client"} {
		if e = os.Mkdir(filepath.Join(root, n), 0700); e != nil {
			t.Fatal(e)
		}
	}
	st, e := storage.Open(filepath.Join(root, "state"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	svc := access.New(st, crypto, time.Now, uint32(os.Getuid()), "a", st)
	server, e := ipc.ListenAccess(filepath.Join(root, "runtime"), uint32(os.Getuid()), svc, foundation.Status{Instance: "test"}, st.Close)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if e = <-done; e != nil {
			t.Error(e)
		}
	})
	return New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uint32(os.Getuid())))
}

func TestRecoveryCacheClipboardAndAckRealIPC(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	if app.CopyRecovery() != "INVALID_REQUEST" {
		t.Fatal("copy without issuance accepted")
	}
	if r := app.Register("technical", "isolated clipboard password"); r.Code != "OK" {
		t.Fatal(r.Code)
	}
	if !PreventClose(app) || app.AckRecovery().Code != "OK" {
		t.Fatal("registration code must be acknowledged before leaving")
	}
	if r := app.Login("technical", "isolated clipboard password", true); r.Code != "OK" {
		t.Fatal(r.Code)
	}
	r := app.RenewRecovery("isolated clipboard password", false)
	if r.Code != "OK" || len(r.Recovery) != 64 {
		t.Fatal("issuance failed", r.Code)
	}
	var copied string
	app.clipboard = func(context.Context, string) error { return errors.New("injected clipboard failure") }
	if app.CopyRecovery() != "CLIPBOARD_UNAVAILABLE" {
		t.Fatal("clipboard error swallowed")
	}
	app.clipboard = func(_ context.Context, value string) error { copied = value; return nil }
	if app.CopyRecovery() != "OK" || copied != r.Recovery {
		t.Fatal("copy differs from issued secret")
	}
	if status := app.Status(); status.Code != "OK" || status.Recovery != "" {
		t.Fatal("snapshot leaked secret")
	}
	if ack := app.AckRecovery(); ack.Code != "OK" {
		t.Fatal(ack.Code)
	}
	if app.recovery != "" || app.CopyRecovery() != "INVALID_REQUEST" {
		t.Fatal("ack retained cache")
	}
	second := app.RenewRecovery("isolated clipboard password", false)
	if second.Code != "OK" {
		t.Fatal(second.Code)
	}
	// Password reset revokes the session even when the UI has an unacked code.
	if reset := app.Recover("technical", second.Recovery, "another isolated password"); reset.Code != "OK" {
		t.Fatal(reset.Code)
	}
	if app.CopyRecovery() != "OK" || copied == second.Recovery {
		t.Fatal("reset must copy its new code, never the revoked one")
	}
}
