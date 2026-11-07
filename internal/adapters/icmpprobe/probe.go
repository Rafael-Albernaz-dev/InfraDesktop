// Package icmpprobe implements the pulse port using the system ping process.
package icmpprobe

import (
	"context"
	"errors"
	"net/netip"
	"os"
	"os/exec"
	"strings"

	"infradesktop/internal/modules/pulse"
)

type Runner interface {
	Run(context.Context, string, []string, []string) ([]byte, error)
}
type processRunner struct{}

func (processRunner) Run(ctx context.Context, path string, args, env []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Env = env
	return cmd.CombinedOutput()
}

type Probe struct{ Runner Runner }

func (p Probe) Probe(ctx context.Context, address string) pulse.Outcome {
	// ignored: Only validation is needed before invoking ping.
	if _, err := netip.ParseAddr(address); err != nil {
		return pulse.ProbeUnavailable
	}
	runner := p.Runner
	if runner == nil {
		runner = processRunner{}
	}
	output, err := runner.Run(ctx, "/usr/bin/ping", []string{"-n", "-c", "1", "-W", "2", "--", address}, []string{"LC_ALL=C"})
	if err == nil {
		return pulse.Reply
	}
	if unavailable(err, string(output)) {
		return pulse.ProbeUnavailable
	}
	return pulse.NoReply
}
func unavailable(err error, output string) bool {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, os.ErrPermission) {
		return true
	}
	text := strings.ToLower(output)
	return strings.Contains(text, "operation not permitted") || strings.Contains(text, "permission denied") ||
		strings.Contains(text, "cap_net_raw")
}
