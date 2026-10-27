// Package routerssh owns fixed read-only RouterOS SSH commands and parsing.
package routerssh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	return &ssh.ClientConfig{User: user, Config: ssh.Config{KeyExchanges: a.KeyExchanges, Ciphers: a.Ciphers, MACs: a.MACs}, HostKeyAlgorithms: a.HostKeys, Timeout: 5 * time.Second}
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
func (Adapter) Query(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code) (*equipment.Observation, equipment.Code) {
	if c := check(); c != equipment.OK {
		return nil, c
	}
	conn, done, c := dial(ctx, d)
	if c != equipment.OK {
		return nil, c
	}
	defer done()
	cfg := config(user)
	mismatch := false
	hostSeen := false

	cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		hostSeen = true
		if ssh.FingerprintSHA256(key) != d.Fingerprint {
			mismatch = true
			return errors.New("host key mismatch")
		}
		if c := check(); c != equipment.OK {
			return errors.New("authorization changed")
		}
		return nil
	}
	cfg.Auth = []ssh.AuthMethod{ssh.PasswordCallback(func() (string, error) {
		if c := check(); c != equipment.OK {
			return "", errors.New("authorization changed")
		}
		return secret, nil
	})}
	cc, channels, requests, e := ssh.NewClientConn(conn, net.JoinHostPort(d.Address, strconv.Itoa(d.Port)), cfg)
	secret = ""
	if e != nil {
		if mismatch {
			return nil, equipment.TrustChanged
		}
		if timedOut(ctx, e) {
			return nil, "DEADLINE_EXCEEDED"
		}
		// x/crypto wraps authentication rejection as this fixed protocol diagnostic.
		// Callback invocation alone does not prove an authentication failure.
		if strings.Contains(e.Error(), "unable to authenticate, attempted methods") {
			return nil, "REMOTE_AUTH_FAILED"
		}
		if !hostSeen || strings.Contains(e.Error(), "no common algorithm") || strings.Contains(e.Error(), "no common") {
			return nil, "SSH_NEGOTIATION_FAILED"
		}
		return nil, "SSH_CONNECTION_LOST"
	}
	client := ssh.NewClient(cc, channels, requests)
	defer client.Close()
	budget := &outputBudget{conn: conn}
	run := func(command string) (string, equipment.Code) {
		if c := check(); c != equipment.OK {
			return "", c
		}
		session, e := client.NewSession()
		if e != nil {
			var denied *ssh.OpenChannelError
			if errors.As(e, &denied) && denied.Reason == ssh.Prohibited {
				return "", "REMOTE_PERMISSION_DENIED"
			}
			return "", "SSH_CONNECTION_LOST"
		}
		defer session.Close()
		out, stderr := &limitedOutput{budget: budget}, &limitedOutput{budget: budget}
		session.Stdout = out
		session.Stderr = stderr
		e = session.Run(command)
		budget.mu.Lock()
		overflow := budget.overflow
		budget.mu.Unlock()
		if overflow {
			return "", equipment.Limit
		}
		if timedOut(ctx, e) {
			return "", "DEADLINE_EXCEEDED"
		}
		text := out.buf.String()
		if permissionDiagnostic(text) || permissionDiagnostic(stderr.buf.String()) {
			return "", "REMOTE_PERMISSION_DENIED"
		}
		if e != nil || stderr.buf.Len() != 0 {
			return "", equipment.Partial
		}
		return text, equipment.OK
	}
	identity, c := run(IdentityCommand)
	if c != equipment.OK {
		return nil, c
	}
	if !strings.HasPrefix(identity, "identity=") {
		return nil, equipment.Partial
	}
	identity = strings.TrimPrefix(identity, "identity=")
	resource, c := run(ResourceCommand)
	if c != equipment.OK {
		return nil, c
	}
	// Parse the useful summary before the independent optional interface part.
	o, c := parseSummary(identity, resource)
	if c != equipment.OK {
		return nil, c
	}
	o.At = time.Now().UTC().Format(time.RFC3339Nano)
	o.Source = fmt.Sprintf("SSH · %s:%d · %s", d.Address, d.Port, d.Fingerprint)
	o.Quality = "complete"
	// Reserve a separate finite part budget within the overall job deadline.
	deadline := time.Now().Add(6 * time.Second)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	_ = conn.SetDeadline(deadline)
	interfaces, code := run(InterfacesCommand)
	if code == equipment.OK {
		code = parseInterfaces(o, interfaces)
	}
	if code != equipment.OK {
		o.Interfaces = nil
		o.InterfacePart = &equipment.InterfacePart{Status: "failed", Code: code}
		o.Quality = "partial"
	} else {
		o.InterfacePart.At = time.Now().UTC().Format(time.RFC3339Nano)
		o.InterfacePart.Source = o.Source
		if o.InterfacePart.Status != "complete" {
			o.Quality = "partial"
		}
	}

	return o, equipment.OK
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
	fields := map[string]string{}
	rs := lines(resource)
	if len(rs) != 7 || rs[6] != "done" {
		return nil, equipment.Partial
	}
	for _, line := range rs[:6] {
		k, v, ok := strings.Cut(line, "=")
		if !ok || fields[k] != "" || !clean(v, 128) {
			return nil, equipment.Partial
		}
		fields[k] = v
	}
	for _, k := range []string{"version", "board", "uptime", "cpu", "free", "total"} {
		if fields[k] == "" {
			return nil, equipment.Partial
		}
	}
	o.Version = fields["version"]
	if strings.HasPrefix(o.Version, "6.") {
		o.Family = "v6"
	} else if strings.HasPrefix(o.Version, "7.") {
		o.Family = "v7"
	} else {
		return nil, "UNSUPPORTED"
	}
	if len(o.Version) < 3 || o.Version[2] < '0' || o.Version[2] > '9' {
		return nil, equipment.Partial
	}
	o.Board = fields["board"]
	o.Uptime = fields["uptime"]
	var e error
	o.CPU, e = strconv.Atoi(fields["cpu"])
	if e != nil || o.CPU < 0 || o.CPU > 100 {
		return nil, equipment.Partial
	}
	o.FreeMemory, e = strconv.ParseUint(fields["free"], 10, 64)
	if e != nil {
		return nil, equipment.Partial
	}
	o.TotalMemory, e = strconv.ParseUint(fields["total"], 10, 64)
	if e != nil || o.TotalMemory == 0 || o.FreeMemory > o.TotalMemory {
		return nil, equipment.Partial
	}
	return o, equipment.OK
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
