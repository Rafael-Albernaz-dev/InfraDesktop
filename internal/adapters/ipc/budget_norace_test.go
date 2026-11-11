//go:build !race

package ipc

import (
	"testing"
	"time"
)

// The race-only scale must never reach a normal or shipped build.
func TestBudgetIsThreeSecondsOutsideRace(t *testing.T) {
	if Budget != 3*time.Second {
		t.Fatal("IPC budget outside race builds", Budget)
	}
}
