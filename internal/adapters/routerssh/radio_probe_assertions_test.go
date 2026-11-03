package routerssh_test

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func assertRadioProbeWindow(t *testing.T, load equipment.RadioLoad, c equipment.Code, started time.Time) {
	t.Helper()
	if c != equipment.OK ||
		load.Started.Before(started.Add(-time.Second)) ||
		load.Ended.Before(load.Started) ||
		load.Ended.After(time.Now()) ||
		load.PingSent != 1 ||
		load.PingReceived != 1 ||
		len(load.PingRTT) != 1 {
		t.Fatal(c, load)
	}
}

func assertRadioProbeCloses(t *testing.T, session equipment.RadioReadSession, cancel context.CancelFunc) {
	t.Helper()
	cancel()
	done := make(chan struct{})
	go func() { session.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("probe goroutine did not join")
	}
}
