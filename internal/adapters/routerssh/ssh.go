// Package routerssh owns fixed read-only RouterOS SSH commands and parsing.
package routerssh

import (
	"bytes"
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

const MaxOutput = 512 * 1024

var errLimit = errors.New("output limit")
var errInspect = errors.New("inspection complete before authentication")

type Adapter struct{}
type outputBudget struct {
	mu       sync.Mutex
	n        int
	limit    int
	overflow bool
	conn     net.Conn
}
type limitedOutput struct {
	budget *outputBudget
	buf    bytes.Buffer
}

func (w *limitedOutput) Write(b []byte) (int, error) {
	w.budget.mu.Lock()
	defer w.budget.mu.Unlock()
	limit := MaxOutput
	if w.budget.limit > 0 && w.budget.limit < limit {
		limit = w.budget.limit
	}
	if w.budget.n+len(b) > limit {
		w.budget.overflow = true
		w.budget.conn.Close()
		return 0, errLimit
	}
	w.budget.n += len(b)
	return w.buf.Write(b)
}
func timedOut(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	if deadline, ok := ctx.Deadline(); ok && !time.Now().Before(deadline) {
		return true
	}
	var timeout net.Error
	return errors.As(err, &timeout) && timeout.Timeout()
}
func dial(ctx context.Context, d equipment.Device) (net.Conn, func(), equipment.Code) {
	return dialBudget(ctx, d, equipment.QueryBudget)
}
func dialBudget(ctx context.Context, d equipment.Device, budget time.Duration) (net.Conn, func(), equipment.Code) {
	if budget <= 0 || budget > 60*time.Second {
		return nil, func() {}, equipment.Invalid
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	conn, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(d.Address, strconv.Itoa(d.Port)))
	if e != nil {
		expired := timedOut(ctx, e)
		cancel()
		if expired {
			return nil, func() {}, "DEADLINE_EXCEEDED"
		}
		return nil, func() {}, "CONNECT_FAILED"
	}
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	return conn, func() { stop(); cancel(); conn.Close() }, equipment.OK
}
func config(user string) *ssh.ClientConfig {
	a := ssh.SupportedAlgorithms()
	// User-authorized global fallback, L07 dispatch P2.4 and Grill decision 19:
	// append only algorithms required by observed pilots, modern algorithms first.
	legacy := ssh.InsecureAlgorithms()
	for _, candidate := range legacy.KeyExchanges {
		if candidate == ssh.InsecureKeyExchangeDH14SHA1 {
			a.KeyExchanges = append(a.KeyExchanges, candidate)
		}
	}
	for _, candidate := range legacy.HostKeys {
		if candidate == ssh.KeyAlgoRSA {
			a.HostKeys = append(a.HostKeys, candidate)
		}
	}
	return &ssh.ClientConfig{User: user, Config: ssh.Config{KeyExchanges: a.KeyExchanges, Ciphers: a.Ciphers, MACs: a.MACs}, HostKeyAlgorithms: a.HostKeys, Timeout: 5 * time.Second}
}

func legacyNegotiated(conn ssh.Conn) []string {
	a, ok := conn.(ssh.AlgorithmsConnMetadata)
	if !ok {
		return nil
	}
	negotiated := a.Algorithms()
	var legacy []string
	if negotiated.KeyExchange == ssh.InsecureKeyExchangeDH14SHA1 {
		legacy = append(legacy, negotiated.KeyExchange)
	}
	if negotiated.HostKey == ssh.KeyAlgoRSA {
		legacy = append(legacy, negotiated.HostKey)
	}
	return legacy
}
func permissionDiagnostic(output string) bool {
	for _, line := range routeros.Lines(output) {
		line = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(line)), "failure: ")
		switch line {
		case "not enough permissions", "not enough permissions (9)", "permission denied":
			return true
		}
	}
	return false
}
func (Adapter) Inspect(ctx context.Context, d equipment.Device) (string, equipment.Code) {
	conn, done, c := dial(ctx, d)
	if c != equipment.OK {
		return "", c
	}
	defer done()
	fingerprint := ""
	cfg := config("infradesktop-host-key-inspection")
	cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint = ssh.FingerprintSHA256(key)
		return errInspect
	}
	_, _, _, err := ssh.NewClientConn(conn, net.JoinHostPort(d.Address, strconv.Itoa(d.Port)), cfg)
	if timedOut(ctx, err) {
		return "", "DEADLINE_EXCEEDED"
	}
	if fingerprint == "" {
		return "", "SSH_NEGOTIATION_FAILED"
	}
	if d.Fingerprint != "" && d.Fingerprint != fingerprint {
		return "", equipment.TrustChanged
	}
	return fingerprint, equipment.OK
}
