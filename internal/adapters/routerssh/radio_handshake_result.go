package routerssh

import (
	"context"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"strings"
)

// Classification observes the error before the caller closes its context.
// This helper neither authenticates nor changes the connection or credentials.
func radioHandshakeResult(ctx context.Context, err error, authCode equipment.Code, mismatch, seen bool) equipment.Code {
	switch {
	case authCode != equipment.OK:
		return authCode
	case mismatch:
		return equipment.TrustChanged
	case ctx.Err() == context.Canceled:
		return equipment.Cancelled
	case timedOut(ctx, err):
		return equipment.RadioCodeDeadlineExceeded
	case strings.Contains(err.Error(), "unable to authenticate"):
		return equipment.RadioCodeRemoteAuthFailed
	case !seen:
		return equipment.RadioCodeSSHNegotiationFailed
	default:
		return equipment.RadioCodeSSHConnectionLost
	}
}

func (r *radioSession) recordNegotiatedConnection(conn ssh.Conn) {
	r.legacy = legacyNegotiated(conn)
	if algorithms, ok := conn.(ssh.AlgorithmsConnMetadata); ok {
		selected := algorithms.Algorithms()
		r.negotiated = []string{selected.KeyExchange, selected.HostKey}
	}
}

func (r *radioSession) startProbe() {
	r.probeReady = make(chan struct{}, 1)
	r.probeExited = make(chan struct{})
	go r.probeLoop()
}
