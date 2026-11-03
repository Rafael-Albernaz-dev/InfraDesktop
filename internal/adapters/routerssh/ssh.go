// Package routerssh owns fixed read-only RouterOS SSH commands and parsing.
package routerssh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Label data before classifying diagnostics: even an identity equal to an error
// phrase remains data. RouterOS scripting supports concatenation in v6/v7.
const IdentityCommand = `:put ("identity=" . [/system identity get name])`
const ResourceCommand = `:put ("version=" . [/system resource get version]); :put ("board=" . [/system resource get board-name]); :put ("uptime=" . [/system resource get uptime]); :put ("cpu=" . [/system resource get cpu-load]); :put ("free=" . [/system resource get free-memory]); :put ("total=" . [/system resource get total-memory]); :put "done"`

// Dynamic interfaces (including PPP tunnels) are counted, not enumerated as sessions.
// Static identifiers are captured once; payload and displayed sample are bounded.
const InterfacesCommand = routerFlagScript + `:local ids [/interface find where dynamic!=yes]; :put ("eligible=" . [:len $ids]); :put ("dynamic=" . [/interface print count-only where dynamic=yes]); :local n 0; :foreach i in=$ids do={:if ($n < 128) do={:put ("name=" . [/interface get $i name]); :put ("type=" . [/interface get $i type]); :put ("running=" . [$ppflag [/interface get $i running]]); :put ("disabled=" . [/interface get $i disabled]); :put "end"; :set n ($n + 1)}}; :put "done"`

// In fixed RouterOS flag slots an unmarked flag is nil, not an explicit false.
// This is only for documented flags (running/dynamic/invalid), never optional
// configuration. Unexpected values fail the read instead of becoming false.
const routerFlagScript = `:local ppflag do={:local flagValue $1; :if ([:typeof $flagValue]="nil") do={:return "false"}; :local flagText [:tostr $flagValue]; :if (($flagText="true")||($flagText="yes")) do={:return "true"}; :if (($flagText="false")||($flagText="no")) do={:return "false"}; :error "flag-type"}; `

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
	for _, line := range lines(output) {
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

func clean(s string, max int) bool {
	if !utf8.ValidString(s) || len(s) < 1 || len(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func lines(s string) []string {
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}
func parseSummary(identity, resource string) (*equipment.Observation, equipment.Code) {
	o := &equipment.Observation{Interfaces: []equipment.InterfaceReading{}}
	ids := lines(identity)
	if len(ids) != 1 || !clean(ids[0], 128) {
		return nil, equipment.Partial
	}
	o.Identity = ids[0]
	fields, ok := extractSummaryResourceFields(resource)
	if !ok {
		return nil, equipment.Partial
	}
	return populateSummaryObservation(o, fields)
}
func parseInterfaces(o *equipment.Observation, interfaces string) equipment.Code {
	ls := lines(interfaces)
	if len(ls) >= 4 && ppDecimal(ls[1]) {
		// The embedded dynamic count can be echoed before its labeled value.
		// It must match exactly; this is not a general skip-unknown-lines rule.
		if ls[2] != "dynamic="+ls[1] {
			return equipment.Partial
		}
		ls = append([]string{ls[0]}, ls[2:]...)
	}
	if len(ls) < 3 {
		return equipment.Partial
	}
	counts := []int{}
	for n, k := range []string{"eligible=", "dynamic="} {
		if !strings.HasPrefix(ls[n], k) {
			return equipment.Partial
		}
		count, e := strconv.Atoi(strings.TrimPrefix(ls[n], k))
		if e != nil || count < 0 || count > 1000000 {
			return equipment.Partial
		}
		counts = append(counts, count)
	}
	ls = ls[2:]
	if ls[len(ls)-1] != "done" || (len(ls)-1)%5 != 0 {
		return equipment.Partial
	}
	count := (len(ls) - 1) / 5
	expected := counts[0]
	if expected > 128 {
		expected = 128
	}
	if count != expected {
		return equipment.Partial
	}
	part := &equipment.InterfacePart{CountsKnown: true, Status: "complete", Code: equipment.OK, Total: counts[0] + counts[1], Eligible: counts[0], Dynamic: counts[1]}
	if counts[0] > count {
		part.Status = "limited"
	}
	seen := map[string]bool{}
	for i := 0; i < len(ls)-1; i += 5 {
		if ls[i+4] != "end" {
			return equipment.Partial
		}
		values := []string{}
		for n, k := range []string{"name", "type", "running", "disabled"} {
			prefix := k + "="
			if !strings.HasPrefix(ls[i+n], prefix) {
				return equipment.Partial
			}
			values = append(values, strings.TrimPrefix(ls[i+n], prefix))
		}
		if !clean(values[0], 128) || !clean(values[1], 80) || seen[values[0]] || (values[2] != "true" && values[2] != "false") || (values[3] != "true" && values[3] != "false") {
			return equipment.Partial
		}
		seen[values[0]] = true
		o.Interfaces = append(o.Interfaces, equipment.InterfaceReading{Name: values[0], Kind: values[1], Running: values[2] == "true", Disabled: values[3] == "true"})
	}
	// JSON escaping also has a budget; never let interface bytes discard summary at persistence/IPC.
	for len(o.Interfaces) > 0 {
		encoded, _ := json.Marshal(o.Interfaces)
		if len(encoded) <= 32*1024 {
			break
		}
		o.Interfaces = o.Interfaces[:len(o.Interfaces)-1]
		part.Status = "limited"
	}
	o.InterfacePart = part
	return equipment.OK
}
