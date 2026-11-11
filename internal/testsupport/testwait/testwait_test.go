package testwait

import (
	"testing"
	"time"
)

func TestLimitFollowsTheTestDeadline(t *testing.T) {
	limit := Limit(t)
	if deadline, ok := t.Deadline(); ok {
		if !limit.Equal(deadline.Add(-margin)) {
			t.Fatal("limit ignores the test deadline", limit, deadline)
		}
		return
	}
	if until := time.Until(limit); until <= 0 || until > fallback {
		t.Fatal("fallback limit", until)
	}
}
