package presentation

import (
	"context"
	"errors"
	"os/exec"
	"os/user"
	"regexp"
	"time"

	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
)

var unixName = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

func currentUnixUser() (string, error) {
	current, err := user.Current()
	if err != nil || current.Uid == "0" || !unixName.MatchString(current.Username) {
		return "", errors.New("invalid technician")
	}
	return current.Username, nil
}

func runResetAuthorization(ctx context.Context, username, nonce string) string {
	path, err := exec.LookPath("pkexec")
	if err != nil {
		return "ADMIN_AUTH_UNAVAILABLE"
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	err = exec.CommandContext(ctx, path, "/usr/lib/infradesktop/reset-account", username, nonce).Run()
	if err == nil {
		return "OK"
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 126 {
		return "ADMIN_AUTH_CANCELLED"
	}
	return "ADMIN_AUTH_UNAVAILABLE"
}

// ResetAccount never passes the new password to the privileged helper.
func (a *App) ResetAccount(name, password string) Result {
	if !access.ValidName(name) || !access.ValidPassword(password) {
		return Result{Code: "INVALID_REQUEST", Companies: []ipc.Company{}}
	}
	if !a.lifecycle.beginEmission() {
		return Result{Code: "BUSY", Companies: []ipc.Company{}}
	}
	defer a.lifecycle.endEmission()
	a.mu.Lock()
	defer a.mu.Unlock()
	defer a.syncRecoveryPending()
	return a.requestReset(name, password)
}

func (a *App) requestReset(name, password string) Result {
	pending, err := a.client.ResetIntentPending()
	if err != nil {
		return Result{Code: "SERVICE_UNAVAILABLE", Companies: []ipc.Company{}}
	}
	if pending {
		return Result{Code: "RESET_VERIFICATION_REQUIRED", Companies: []ipc.Company{}}
	}
	username, err := a.resetUnixUser()
	if err != nil {
		return Result{Code: "ADMIN_AUTH_UNAVAILABLE", Companies: []ipc.Company{}}
	}
	begin := a.client.Call(a.ctx, ipc.Request{Op: "access.reset-begin"})
	if begin.Code != "OK" {
		return accountResult(begin)
	}
	if begin.Data == nil || len(begin.Data.ResetNonce) != 64 {
		return Result{Code: "SERVICE_UNAVAILABLE", Companies: []ipc.Company{}}
	}
	code := a.resetAuthorize(a.ctx, username, begin.Data.ResetNonce)
	if code != "OK" {
		return Result{Code: code, Companies: []ipc.Company{}}
	}
	intent, err := a.client.BeginResetIntent()
	if errors.Is(err, ipc.ErrResetVerification) {
		return Result{Code: "RESET_VERIFICATION_REQUIRED", Companies: []ipc.Company{}}
	}
	if err != nil {
		return Result{Code: "SERVICE_UNAVAILABLE", Companies: []ipc.Company{}}
	}
	defer intent.Close()
	q := ipc.Request{Op: "access.reset-complete", Name: name, Password: password, ResetNonce: begin.Data.ResetNonce}
	r := a.client.Call(a.ctx, q)
	a.ppGeneration++
	if resetOutcomeUnknown(r) {
		return Result{Code: "RESET_OUTCOME_UNKNOWN", Companies: []ipc.Company{}}
	}
	if r.Code != "OK" && intent.Clear() != nil {
		return Result{Code: "RESET_VERIFICATION_REQUIRED", Companies: []ipc.Company{}}
	}
	a.updateRecoveryAfterCall(q, r)
	return accountResult(r)
}

func resetOutcomeUnknown(r ipc.Response) bool {
	return r.Code == foundation.Deadline || r.Code == foundation.Unavailable || r.Code == foundation.Version
}

func (a *App) prepareResetVerification(q ipc.Request) (*ipc.ResetIntent, string) {
	if q.Op != "access.login" {
		return nil, ""
	}
	pending, err := a.client.ResetIntentPending()
	if err != nil {
		return nil, "SERVICE_UNAVAILABLE"
	}
	if !pending {
		return nil, ""
	}
	intent, err := a.client.LockResetIntent()
	if err != nil || intent == nil {
		return nil, "RESET_VERIFICATION_REQUIRED"
	}
	// The begin operation serializes behind any earlier completion in the daemon.
	barrier := a.client.Call(a.ctx, ipc.Request{Op: "access.reset-begin"})
	if barrier.Code != "OK" {
		intent.Close()
		return nil, "RESET_VERIFICATION_REQUIRED"
	}
	return intent, ""
}

func (a *App) clearResetIntent() string {
	pending, err := a.client.ResetIntentPending()
	if err != nil {
		return "RESET_VERIFICATION_REQUIRED"
	}
	if !pending {
		return ""
	}
	intent, err := a.client.LockResetIntent()
	if err != nil || intent == nil {
		return "RESET_VERIFICATION_REQUIRED"
	}
	defer intent.Close()
	if intent.Clear() != nil {
		return "RESET_VERIFICATION_REQUIRED"
	}
	return ""
}
