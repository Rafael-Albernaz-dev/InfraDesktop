package icmpprobe

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"infradesktop/internal/modules/pulse"
)

type runnerFunc func(context.Context, string, []string, []string) ([]byte, error)

func (f runnerFunc) Run(c context.Context, p string, a, e []string) ([]byte, error) {
	return f(c, p, a, e)
}
func TestPingClassifiesProbeFailureSeparatelyFromUnreachable(t *testing.T) {
	for _, tc := range []struct {
		name, output string
		err          error
		want         pulse.Outcome
	}{
		{"reply", "", nil, pulse.Reply},
		{"lost", "", errors.New("exit 1"), pulse.NoReply},
		{"route", "ping: connect: Network is unreachable", errors.New("exit 2"), pulse.NoReply},
		{"permission", "ping: socket: Operation not permitted", errors.New("exit 2"), pulse.ProbeUnavailable},
		{"denied", "ping: Permission denied", errors.New("exit 2"), pulse.ProbeUnavailable},
		{"missing", "", os.ErrNotExist, pulse.ProbeUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Probe{Runner: runnerFunc(func(_ context.Context, path string, args, env []string) ([]byte, error) {
				if path != "/usr/bin/ping" || !reflect.DeepEqual(args, []string{"-n", "-c", "1", "-W", "2", "--", "127.0.0.1"}) ||
					!reflect.DeepEqual(env, []string{"LC_ALL=C"}) {
					t.Fatal("unsafe invocation", path, args, env)
				}
				return []byte(tc.output), tc.err
			})}
			if got := p.Probe(context.Background(), "127.0.0.1"); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}
func TestInvalidAddressNeverRunsPing(t *testing.T) {
	p := Probe{Runner: runnerFunc(func(context.Context, string, []string, []string) ([]byte, error) {
		t.Fatal("runner called")
		return nil, nil
	})}
	if p.Probe(context.Background(), "--help") != pulse.ProbeUnavailable {
		t.Fatal("invalid accepted")
	}
}
func TestLoopbackPing(t *testing.T) {
	if got := (Probe{}).Probe(context.Background(), "127.0.0.1"); got != pulse.Reply {
		t.Fatalf("loopback: %s", got)
	}
}
func TestCancelledContextStopsProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := (Probe{}).Probe(ctx, "127.0.0.1"); got == pulse.Reply {
		t.Fatal("cancelled process replied")
	}
}
