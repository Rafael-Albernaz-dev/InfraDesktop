package routerssh

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
)

const liveCommandPrefix = `:put ("L\t1\t".[/system resource get uptime]."\t".[/system resource get cpu-load]."\t".`
const liveConcentratorCommand = liveCommandPrefix + `[:len [/ppp active find where service=pppoe]])`
const liveAPCommand = liveCommandPrefix + `[:len [/interface wireless registration-table find]])`

type liveSession struct {
	client *ssh.Client
	conn   net.Conn
}

// OpenLive owns a separate connection with a handshake deadline, never a cumulative read budget.
func OpenLive(ctx context.Context, d equipment.Device, user, secret string) (pulse.ReadSession, pulse.ReadFault) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	endpoint := net.JoinHostPort(d.Address, strconv.Itoa(d.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		return nil, pulse.ReadLost
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	// ignored: WithTimeout always establishes a deadline.
	deadline, _ := ctx.Deadline()
	if err := conn.SetDeadline(deadline); err != nil {
		conn.Close()
		return nil, pulse.ReadLost
	}
	client, fault := liveHandshake(conn, d, user, secret)
	secret = ""
	if fault != pulse.ReadOK {
		conn.Close()
		return nil, fault
	}
	if !stop() || ctx.Err() != nil {
		client.Close()
		return nil, pulse.ReadLost
	}
	if err := conn.SetDeadline(time.Time{}); err != nil {
		client.Close()
		return nil, pulse.ReadLost
	}
	return &liveSession{client: client, conn: conn}, pulse.ReadOK
}
func liveHandshake(conn net.Conn, d equipment.Device, user, secret string) (*ssh.Client, pulse.ReadFault) {
	cfg := config(user)
	mismatch := false
	cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		if d.Fingerprint == "" || ssh.FingerprintSHA256(key) != d.Fingerprint {
			mismatch = true
			return errors.New("host key mismatch")
		}
		return nil
	}
	cfg.Auth = []ssh.AuthMethod{ssh.PasswordCallback(func() (string, error) { return secret, nil })}
	endpoint := net.JoinHostPort(d.Address, strconv.Itoa(d.Port))
	cc, channels, requests, err := ssh.NewClientConn(conn, endpoint, cfg)
	secret = ""
	cfg.Auth = nil
	if err != nil {
		return nil, liveConnectFault(err, mismatch)
	}
	return ssh.NewClient(cc, channels, requests), pulse.ReadOK
}
func liveConnectFault(err error, mismatch bool) pulse.ReadFault {
	if mismatch {
		return pulse.ReadKeyChanged
	}
	if strings.Contains(err.Error(), "unable to authenticate, attempted methods") {
		return pulse.ReadRefused
	}
	return pulse.ReadLost
}
func (s *liveSession) Close() { s.client.Close() }
func (s *liveSession) Read(ctx context.Context, t pulse.ReadTarget) (pulse.Reading, pulse.ReadFault) {
	command := liveConcentratorCommand
	switch t.Role {
	case equipment.RoleConcentrator:
	case equipment.RoleAP:
		command = liveAPCommand
	default:
		return pulse.Reading{}, pulse.ReadLost
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	// ignored: WithTimeout always establishes a deadline.
	deadline, _ := ctx.Deadline()
	if err := s.conn.SetDeadline(deadline); err != nil {
		return pulse.Reading{}, pulse.ReadLost
	}
	stop := context.AfterFunc(ctx, func() { s.conn.Close() })
	defer stop()
	session, err := s.client.NewSession()
	if err != nil {
		return pulse.Reading{}, pulse.ReadLost
	}
	defer session.Close()
	budget := &outputBudget{conn: s.conn, limit: 4096}
	stdout, stderr := &limitedOutput{budget: budget}, &limitedOutput{budget: budget}
	session.Stdout, session.Stderr = stdout, stderr
	err = session.Run(command)
	if err != nil || ctx.Err() != nil || budget.overflow || stderr.buf.Len() != 0 {
		return pulse.Reading{}, pulse.ReadLost
	}
	if !stop() {
		return pulse.Reading{}, pulse.ReadLost
	}
	if err := s.conn.SetDeadline(time.Time{}); err != nil {
		return pulse.Reading{}, pulse.ReadLost
	}
	return parseLiveReading(stdout.buf.String())
}
func parseLiveReading(output string) (pulse.Reading, pulse.ReadFault) {
	fields := strings.Split(strings.TrimRight(output, "\r\n"), "\t")
	if len(fields) != 5 || fields[0] != "L" || fields[1] != "1" {
		return pulse.Reading{}, pulse.ReadLost
	}
	uptime := ParsePPDuration(fields[2])
	cpu, errCPU := strconv.Atoi(fields[3])
	clients, errClients := strconv.Atoi(fields[4])
	if !uptime.Known || !ppDecimal(fields[3]) || !ppDecimal(fields[4]) ||
		errCPU != nil || errClients != nil || cpu < 0 || cpu > 100 || clients < 0 {
		return pulse.Reading{}, pulse.ReadLost
	}
	return pulse.Reading{CPU: cpu, Clients: clients, Uptime: time.Duration(uptime.Min)}, pulse.ReadOK
}
