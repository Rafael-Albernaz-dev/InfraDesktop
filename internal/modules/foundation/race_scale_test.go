//go:build !race

package foundation

import "testing"

// The race-only stretch must never reach a normal or shipped build.
func TestRaceScaleIsOneOutsideRace(t *testing.T) {
	if RaceScale != 1 {
		t.Fatal("race scale outside race builds", RaceScale)
	}
}
