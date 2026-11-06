package presentation

import (
	"context"
	"errors"
	"os/exec"
	"os/user"
	"regexp"
	"time"
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
	if errors.As(err, &exit) {
		switch exit.ExitCode() {
		case 126:
			return "ADMIN_AUTH_CANCELLED"
		case 127:
			return "ADMIN_AUTH_DENIED"
		}
	}
	return "ADMIN_AUTH_UNAVAILABLE"
}
