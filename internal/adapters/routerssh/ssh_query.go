package routerssh

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"net"
	"strconv"
	"strings"
	"time"
)

func (Adapter) Query(
	ctx context.Context,
	d equipment.Device,
	user, secret string,
	check func() equipment.Code,
) (*equipment.Observation, equipment.Code) {
	if c := check(); c != equipment.OK {
		return nil, c
	}
	conn, done, c := dial(ctx, d)
	if c != equipment.OK {
		return nil, c
	}
	defer done()
	client, c := openQuerySSHClient(ctx, conn, d, user, secret, check)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer client.Close()
	budget := &outputBudget{conn: conn}
	run := func(command string) (string, equipment.Code) {
		return runQuerySession(ctx, client, budget, command, check)
	}
	o, c := readQuerySummary(d, run)
	if c != equipment.OK {
		return nil, c
	}
	readQueryInterfaces(ctx, conn, o, run)
	return o, equipment.OK
}

func openQuerySSHClient(
	ctx context.Context,
	conn net.Conn,
	d equipment.Device,
	user, secret string,
	check func() equipment.Code,
) (*ssh.Client, equipment.Code) {
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
	endpoint := net.JoinHostPort(d.Address, strconv.Itoa(d.Port))
	cc, channels, requests, e := ssh.NewClientConn(conn, endpoint, cfg)
	secret = ""
	if e != nil {
		return nil, classifySSHConnectError(ctx, e, mismatch, hostSeen)
	}
	return ssh.NewClient(cc, channels, requests), equipment.OK
}

func classifySSHConnectError(
	ctx context.Context, e error, mismatch, hostSeen bool,
) equipment.Code {
	if mismatch {
		return equipment.TrustChanged
	}
	if timedOut(ctx, e) {
		return equipment.RadioCodeDeadlineExceeded
	}
	if strings.Contains(e.Error(), "unable to authenticate, attempted methods") {
		return equipment.RadioCodeRemoteAuthFailed
	}
	if !hostSeen ||
		strings.Contains(e.Error(), "no common algorithm") ||
		strings.Contains(e.Error(), "no common") {
		return equipment.RadioCodeSSHNegotiationFailed
	}
	return equipment.RadioCodeSSHConnectionLost
}

func runQuerySession(
	ctx context.Context,
	client *ssh.Client,
	budget *outputBudget,
	command string,
	check func() equipment.Code,
) (string, equipment.Code) {
	if c := check(); c != equipment.OK {
		return "", c
	}
	session, e := client.NewSession()
	if e != nil {
		var denied *ssh.OpenChannelError
		if errors.As(e, &denied) && denied.Reason == ssh.Prohibited {
			return "", equipment.RadioCodeRemotePermissionDenied
		}
		return "", equipment.RadioCodeSSHConnectionLost
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
		return "", equipment.RadioCodeDeadlineExceeded
	}
	text := out.buf.String()
	if permissionDiagnostic(text) || permissionDiagnostic(stderr.buf.String()) {
		return "", equipment.RadioCodeRemotePermissionDenied
	}
	if e != nil || stderr.buf.Len() != 0 {
		return "", equipment.Partial
	}
	return text, equipment.OK
}

func extractSummaryResourceFields(resource string) (map[string]string, bool) {
	fields := map[string]string{}
	rs := lines(resource)
	if len(rs) != 7 || rs[6] != "done" {
		return nil, false
	}
	for _, line := range rs[:6] {
		k, v, ok := strings.Cut(line, "=")
		if !ok || fields[k] != "" || !clean(v, 128) {
			return nil, false
		}
		fields[k] = v
	}
	for _, k := range []string{"version", "board", "uptime", "cpu", "free", "total"} {
		if fields[k] == "" {
			return nil, false
		}
	}
	return fields, true
}

func populateSummaryObservation(
	o *equipment.Observation, fields map[string]string,
) (*equipment.Observation, equipment.Code) {
	o.Version = fields["version"]
	if strings.HasPrefix(o.Version, "6.") {
		o.Family = equipment.RouterFamilyV6
	} else if strings.HasPrefix(o.Version, "7.") {
		o.Family = equipment.RouterFamilyV7
	} else {
		return nil, equipment.RadioCodeUnsupported
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

func readQuerySummary(
	d equipment.Device, run func(string) (string, equipment.Code),
) (*equipment.Observation, equipment.Code) {
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
	o, c := parseSummary(identity, resource)
	if c != equipment.OK {
		return nil, c
	}
	o.At = time.Now().UTC().Format(time.RFC3339Nano)
	o.Source = fmt.Sprintf("SSH · %s:%d · %s", d.Address, d.Port, d.Fingerprint)
	o.Quality = "complete"
	return o, equipment.OK
}

func readQueryInterfaces(
	ctx context.Context,
	conn net.Conn,
	o *equipment.Observation,
	run func(string) (string, equipment.Code),
) {
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
		return
	}
	o.InterfacePart.At = time.Now().UTC().Format(time.RFC3339Nano)
	o.InterfacePart.Source = o.Source
	if o.InterfacePart.Status != "complete" {
		o.Quality = "partial"
	}
}
