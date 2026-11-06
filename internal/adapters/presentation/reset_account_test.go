package presentation

import (
	"context"
	"infradesktop/internal/adapters/security"
	"testing"
)

func TestResetCancellationNeverSendsPasswordToAuthorization(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	if app.Register("technical", "isolated password").Code != "OK" || app.AckRecovery().Code != "OK" {
		t.Fatal("fixture setup")
	}
	app.resetUnixUser = func() (string, error) { return "synthetic", nil }
	called := false
	app.resetAuthorize = func(_ context.Context, user, nonce string) string {
		called = true
		if user != "synthetic" || len(nonce) != 64 || nonce == "new isolated password" {
			t.Fatal("wrong authorization arguments")
		}
		return "ADMIN_AUTH_CANCELLED"
	}
	if result := app.ResetAccount("new-user", "new isolated password"); result.Code != "ADMIN_AUTH_CANCELLED" || !called {
		t.Fatal("cancellation not reported", result.Code)
	}
	if result := app.Login("technical", "isolated password", false); result.Code != "OK" {
		t.Fatal("cancelled reset changed account", result.Code)
	}
}
func TestResetAuthorizationWithoutPkexecIsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	nonce := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if code := runResetAuthorization(context.Background(), "synthetic", nonce); code != "ADMIN_AUTH_UNAVAILABLE" {
		t.Fatal("missing pkexec not reported", code)
	}
}
