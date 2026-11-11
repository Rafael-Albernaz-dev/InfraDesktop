//go:build race

package foundation

// RaceScale stretches in-process time budgets only in race-detector builds, which run
// several times slower and are never shipped. Timing is proven by the normal `go test`;
// -race only looks for races.
const RaceScale = 5
