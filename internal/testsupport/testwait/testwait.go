// Package testwait bounds waits for an asynchronous result (a job reaching a final
// state) by the test's own deadline instead of a fixed wall-clock guess. Jobs end
// through the product's own deadlines; a fixed 3 s wait only measured machine load,
// and failed under -race with parallel packages.
package testwait

import (
	"testing"
	"time"
)

// margin leaves time for the failure report before `go test -timeout` kills the binary.
const margin = 5 * time.Second

// fallback applies when the test runs without a deadline (-timeout 0).
const fallback = 2 * time.Minute

// Limit is the instant a wait for an asynchronous result gives up.
func Limit(t *testing.T) time.Time {
	t.Helper()
	if deadline, ok := t.Deadline(); ok {
		return deadline.Add(-margin)
	}
	return time.Now().Add(fallback)
}
