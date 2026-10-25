package ipc

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestProcessOwnerIdentification(t *testing.T) {
	for _, test := range []struct {
		name, stat, want string
		err              error
	}{
		{name: "live", stat: "42 (name with ) parenthesis) S " + strings.Repeat("0 ", 18) + "789", want: "42:789"},
		{name: "zombie", stat: "42 (child) Z " + strings.Repeat("0 ", 18) + "789", err: os.ErrNotExist},
		{name: "dead", stat: "42 (child) X " + strings.Repeat("0 ", 18) + "789", err: os.ErrNotExist},
		{name: "malformed", stat: "bad", err: errUnsafe},
		{name: "truncated", stat: "42 (child) S 0", err: errUnsafe},
		{name: "invalid startticks", stat: "42 (child) S " + strings.Repeat("0 ", 18) + "invalid", err: errUnsafe},
		{name: "missing", err: os.ErrNotExist},
		{name: "denied", err: os.ErrPermission},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := readProcessOwner(42, func(path string) ([]byte, error) {
				if path != "/proc/42/stat" {
					t.Fatal(path)
				}
				if test.stat == "" {
					return nil, test.err
				}
				return []byte(test.stat), nil
			})
			if got != test.want || !errors.Is(err, test.err) {
				t.Fatalf("got %q/%v want %q/%v", got, err, test.want, test.err)
			}
		})
	}
	owner, err := processOwner(int32(os.Getpid()))
	if err != nil || !strings.HasPrefix(owner, fmt.Sprint(os.Getpid())+":") {
		t.Fatalf("real PID: %q %v", owner, err)
	}
	if _, err := processOwner(-1); err == nil {
		t.Fatal("unidentified new peer accepted")
	}
}
func TestOwnerLivenessConservative(t *testing.T) {
	for _, test := range []struct {
		name, owner, current string
		err                  error
		want                 bool
	}{
		{"same", "42:1", "42:1", nil, true},
		{"startticks differ", "42:1", "42:2", nil, false},
		{"PID differs", "42:1", "43:1", nil, false},
		{"gone", "42:1", "", os.ErrNotExist, false},
		{"inspection denied", "42:1", "", os.ErrPermission, true},
		{"inspection failure", "42:1", "", errors.New("proc unavailable"), true},
		{"malformed", "bad", "", nil, false},
		{"PID malformed", "no:1", "", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := inspectOwnerAlive(test.owner, func(pid int32) (string, error) {
				if pid != 42 {
					t.Fatal(pid)
				}
				return test.current, test.err
			})
			if got != test.want {
				t.Fatalf("liveness %v want %v", got, test.want)
			}
		})
	}
}
