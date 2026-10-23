package presentation

import (
	"context"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
	"sync"
	"testing"
)

// Gate only the KDF port; binding, IPC, service and SQLite remain real.
type lifecycleCrypto struct {
	security.Crypto
	started, release       chan struct{}
	blockHash, blockVerify bool
}

func (c *lifecycleCrypto) wait() {
	close(c.started)
	<-c.release
}
func (c *lifecycleCrypto) Hash(p string) (access.Verifier, access.Code) {
	if c.blockHash {
		c.wait()
	}
	return c.Crypto.Hash(p)
}
func (c *lifecycleCrypto) Verify(p string, v access.Verifier) bool {
	if c.blockVerify {
		c.wait()
	}
	return c.Crypto.Verify(p, v)
}

func TestNativeCloseVetoForEveryRecoveryEmissionAndAck(t *testing.T) {
	for _, op := range []string{"register", "recover", "renew"} {
		t.Run(op, func(t *testing.T) {
			crypto := &lifecycleCrypto{}
			app := newRecoveryApp(t, crypto)
			const password = "isolated lifecycle password"
			var original string
			if op != "register" {
				r := app.Register("technical", password)
				if r.Code != "OK" || app.AckRecovery().Code != "OK" {
					t.Fatal("registration setup failed", r.Code)
				}
				original = r.Recovery
			}
			if op == "renew" {
				if r := app.Login("technical", password, false); r.Code != "OK" {
					t.Fatal("login setup failed", r.Code)
				}
			}
			crypto.started, crypto.release = make(chan struct{}), make(chan struct{})
			crypto.blockHash, crypto.blockVerify = op != "renew", op == "renew"
			release := sync.OnceFunc(func() { close(crypto.release) })
			defer release()
			done := make(chan Result, 1)
			go func() {
				switch op {
				case "register":
					done <- app.Register("technical", password)
				case "recover":
					done <- app.Recover("technical", original, "new isolated lifecycle password")
				default:
					done <- app.RenewRecovery(password, false)
				}
			}()
			<-crypto.started
			// Callback must veto immediately while IPC holds app.mu.
			if !PreventClose(app) {
				t.Fatal("native close allowed during emission")
			}
			release()
			r := <-done
			if r.Code != "OK" || len(r.Recovery) != 64 {
				t.Fatal("issuance failed", r.Code)
			}
			if !PreventClose(app) {
				t.Fatal("native close discarded unacknowledged code")
			}
			if status := app.Status(); status.Code != "OK" || status.Recovery != "" || !PreventClose(app) {
				t.Fatal("polling exposed or released code", status.Code)
			}
			if op == "renew" {
				if theme := app.SetTheme("dark"); theme.Code != "OK" || !PreventClose(app) {
					t.Fatal("theme released pending code", theme.Code)
				}
			}
			if ack := app.AckRecovery(); ack.Code != "OK" {
				t.Fatal("ack failed", ack.Code)
			}
			if PreventClose(app) {
				t.Fatal("native close veto persisted after ack")
			}
			if late := app.Register("late", password); late.Code != "BUSY" {
				t.Fatal("late issuance started after close won", late.Code)
			}
		})
	}
}

func TestNativeCloseAfterFailedEmission(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	if r := app.Register("technical", "short"); r.Code != "INVALID_REQUEST" {
		t.Fatal("invalid registration accepted", r.Code)
	}
	if PreventClose(app) {
		t.Fatal("failed emission left a native close veto")
	}
}

func TestNativeCloseKeepsCodeAfterFailedAck(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	const password = "isolated lifecycle password"
	if app.Register("technical", password).Code != "OK" || app.AckRecovery().Code != "OK" || app.Login("technical", password, false).Code != "OK" {
		t.Fatal("account setup failed")
	}
	if r := app.RenewRecovery(password, false); r.Code != "OK" {
		t.Fatal("issuance failed", r.Code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	AttachContext(app, ctx)
	if ack := app.AckRecovery(); ack.Code == "OK" || !PreventClose(app) {
		t.Fatal("failed ack released native veto", ack.Code)
	}
	AttachContext(app, context.Background())
	if app.AckRecovery().Code != "OK" || PreventClose(app) {
		t.Fatal("successful ack did not release native veto")
	}
}

func TestNativeCloseRequiresObservedRevocationToReleaseDisplayedCode(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	const password = "isolated lifecycle password"
	if app.Register("technical", password).Code != "OK" || app.AckRecovery().Code != "OK" || app.Login("technical", password, false).Code != "OK" {
		t.Fatal("account setup failed")
	}
	r := app.RenewRecovery(password, false)
	if r.Code != "OK" {
		t.Fatal("issuance failed", r.Code)
	}
	// Simulate a reset outside the presentation's current surface. The real
	// daemon revokes the session before the UI has observed the new snapshot.
	reset := app.client.Call(context.Background(), ipc.Request{Op: "access.recover", Name: "technical", Recovery: r.Recovery, Password: "new isolated lifecycle password"})
	if reset.Code != "OK" {
		t.Fatal("reset failed", reset.Code)
	}
	if app.CopyRecovery() != "UNAUTHENTICATED" || app.recovery != "" || !PreventClose(app) {
		t.Fatal("clipboard invalidation acknowledged the displayed code")
	}
	if status := app.Status(); status.Code != "OK" || status.Authenticated || PreventClose(app) {
		t.Fatal("observed revocation retained native veto", status.Code)
	}
}
