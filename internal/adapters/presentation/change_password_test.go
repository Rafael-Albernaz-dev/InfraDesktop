package presentation

import (
	"infradesktop/internal/adapters/security"
	"testing"
)

func TestPasswordChangeMapsWrongPasswordAndKeepsSession(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	if app.Register("technical", "synthetic old password").Code != "OK" || app.AckRecovery().Code != "OK" ||
		app.Login("technical", "synthetic old password", false).Code != "OK" {
		t.Fatal("fixture setup")
	}
	wrong := app.ChangePassword("wrong synthetic password", "synthetic new password")
	if wrong.Code != "UNAUTHENTICATED" || wrong.ErrorReason != "PASSWORD_INCORRECT" {
		t.Fatal("wrong password reason lost", wrong.Code, wrong.ErrorReason)
	}
}
func TestExpiredRecoverySessionHasNoIncorrectPasswordReason(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	if app.Register("technical", "synthetic old password").Code != "OK" || app.AckRecovery().Code != "OK" ||
		app.Login("technical", "synthetic old password", false).Code != "OK" || app.Logout().Code != "OK" {
		t.Fatal("fixture setup")
	}
	results := []Result{app.ChangePassword("wrong synthetic password", "synthetic new password"),
		app.RenewRecovery("wrong synthetic password", false)}
	for _, r := range results {
		if r.Code != "UNAUTHENTICATED" || r.ErrorReason != "" {
			t.Fatal("expired session reported as incorrect password", r.Code, r.ErrorReason)
		}
	}
}
func TestPasswordChangeSuccessRetainsCurrentSession(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	if app.Register("technical", "synthetic old password").Code != "OK" || app.AckRecovery().Code != "OK" ||
		app.Login("technical", "synthetic old password", false).Code != "OK" {
		t.Fatal("fixture setup")
	}
	if r := app.ChangePassword("synthetic old password", "synthetic new password"); r.Code != "OK" || !r.Authenticated {
		t.Fatal("change failed", r.Code)
	}
	if r := app.Status(); r.Code != "OK" || !r.Authenticated {
		t.Fatal("current session lost", r.Code)
	}
}
