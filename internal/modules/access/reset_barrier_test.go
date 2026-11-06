package access

import (
	"context"
	"testing"
	"time"
)

type blockedResetCrypto struct {
	*resetCrypto
	started, release chan struct{}
}

func (c *blockedResetCrypto) Hash(p string) (Verifier, Code) {
	close(c.started)
	<-c.release
	return c.resetCrypto.Hash(p)
}
func TestResetBeginWaitsForInFlightCompletion(t *testing.T) {
	svc, _, proof, _ := resetFixture()
	nonce := beginReset(t, svc, proof)
	crypto := &blockedResetCrypto{resetCrypto: &resetCrypto{next: 10}, started: make(chan struct{}), release: make(chan struct{})}
	svc.crypto = crypto
	completed := make(chan Code, 1)
	go func() { _, code := completeReset(svc, "requester", nonce); completed <- code }()
	<-crypto.started
	barrier := make(chan Code, 1)
	started := make(chan struct{})
	go func() {
		close(started)
		_, code := svc.Handle(context.Background(), 1000, Command{Op: "access.reset-begin", Owner: "verifier"})
		barrier <- code
	}()
	<-started
	select {
	case code := <-barrier:
		t.Fatal("verification crossed in-flight reset", code)
	case <-time.After(50 * time.Millisecond):
	}
	close(crypto.release)
	if code := <-completed; code != OK {
		t.Fatal("reset", code)
	}
	if code := <-barrier; code != OK {
		t.Fatal("verification barrier", code)
	}
}
