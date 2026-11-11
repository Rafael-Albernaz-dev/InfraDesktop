//go:build !race

package foundation

// RaceScale is 1 in every shipped and normal test build: product budgets hold as written.
const RaceScale = 1
