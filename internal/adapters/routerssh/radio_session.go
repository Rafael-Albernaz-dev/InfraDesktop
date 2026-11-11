package routerssh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Model A only. No remote loop is enabled without the real v6 channel-close
// proof. Each command has its own three-second deadline, on one pinned socket.
type radioSession struct {
	failure       *equipment.RadioFailure
	ctx           context.Context
	cancel        context.CancelFunc
	conn          net.Conn
	client        *ssh.Client
	check         func() equipment.Code
	d             equipment.Device
	family        equipment.RadioFamily
	serverVersion string
	link          *equipment.RadioLink
	initial       *equipment.RadioReading
	legacy        []string
	negotiated    []string
	once          sync.Once
	stop          func() bool
	probeMu       sync.Mutex
	probe         radioProbeState
	probeReady    chan struct{}
	probeExited   chan struct{}
}

type radioProbeState struct {
	load equipment.RadioLoad
	code equipment.Code
}

func (Adapter) OpenRadioSession(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	family equipment.RadioFamily,
	link *equipment.RadioLink,
	check func() equipment.Code) (equipment.RadioReadSession,
	equipment.Code) {
	session, c := openRadioSession(ctx, d, user, secret, family, link, check, true)
	if c != equipment.OK {
		return nil, c
	}
	return session, equipment.OK
}
func openRadioSession(ctx context.Context, d equipment.Device, user, secret string, family equipment.RadioFamily,
	link *equipment.RadioLink, check func() equipment.Code, probe bool) (*radioSession, equipment.Code) {
	if !admitsRadioSession(d.Role, family, link) {
		return nil, equipment.Invalid
	}

	if c := check(); c != equipment.OK {
		return nil, c
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	conn, e := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", net.JoinHostPort(d.Address, strconv.Itoa(d.Port)))
	if e != nil {
		cancel()
		return nil, equipment.RadioCodeConnectFailed
	}
	r := &radioSession{ctx: ctx, cancel: cancel, conn: conn, check: check, d: d, family: family, link: link}
	r.stop = context.AfterFunc(ctx, func() { _ = conn.Close() })
	_ = conn.SetDeadline(time.Now().Add(equipment.QueryBudget))
	cfg := config(user)
	authCode := equipment.OK
	seen, mismatch := false, false
	cfg.HostKeyCallback = func(_ string, _ net.Addr, key ssh.PublicKey) error {
		seen = true
		if ssh.FingerprintSHA256(key) != d.Fingerprint {
			mismatch = true
			return errors.New("pin changed")
		}
		authCode = check()
		if authCode != equipment.OK {
			return errors.New("authorization changed")
		}
		return nil
	}
	cfg.Auth = []ssh.AuthMethod{ssh.PasswordCallback(func() (string, error) {
		authCode = check()
		if authCode != equipment.OK {
			return "", errors.New("authorization changed")
		}
		return secret, nil
	})}
	cc, ch, req, e := ssh.NewClientConn(conn, net.JoinHostPort(d.Address, strconv.Itoa(d.Port)), cfg)
	secret = ""
	if e != nil {
		// Classify before Close cancels the session context.
		defer r.Close()
		return nil, radioHandshakeResult(ctx, e, authCode, mismatch, seen)
	}
	r.serverVersion = string(cc.ServerVersion())
	r.client = ssh.NewClient(cc, ch, req)
	r.recordNegotiatedConnection(cc)
	_ = conn.SetDeadline(time.Time{})
	if c := check(); c != equipment.OK {
		r.Close()
		return nil, c
	}
	if d.Role == equipment.RoleAP && probe {
		r.startProbe()
	}
	return r, equipment.OK
}

func (r *radioSession) Close() {
	r.once.Do(func() {
		if r.stop != nil {
			r.stop()
		}
		r.cancel()
		_ = r.conn.Close()
	})
	if r.probeExited != nil {
		<-r.probeExited
	}
}

// ICMP is local, one probe per second, no SSH observer or TCP probe. Results
// are consumed once and include loss and permission/parse failures explicitly.
func (r *radioSession) probeLoop() {
	defer close(r.probeExited)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for r.ctx.Err() == nil {
		started := time.Now()
		p := exec.CommandContext(r.ctx, "/usr/bin/ping", "-n", "-c", "1", "-W", "1", r.d.Address)
		p.Env = append(os.Environ(), "LC_ALL=C")
		out, _ := p.Output()
		ended := time.Now()
		counts := regexp.MustCompile(`([0-9]+) packets transmitted, ([0-9]+) received`).FindSubmatch(out)
		code := equipment.OK
		sent, received := 0, 0
		var values []float64
		if len(counts) != 3 {
			code = equipment.Partial
		} else {
			sent, _ = strconv.Atoi(string(counts[1]))
			received, _ = strconv.Atoi(string(counts[2]))
			for _, m := range regexp.MustCompile(`time[=<]([0-9.]+) ms`).FindAllSubmatch(out, -1) {
				v, e := strconv.ParseFloat(string(m[1]), 64)
				if e != nil {
					code = equipment.Partial
				}
				values = append(values, v)
			}
			if sent != 1 || received < 0 || received > 1 || len(values) != received {
				code = equipment.Partial
			}
		}
		r.probeMu.Lock()
		load := &r.probe.load
		if load.Started.IsZero() {
			load.Started = started
		}
		load.Ended = ended
		load.PingSent += sent
		load.PingReceived += received
		load.PingRTT = append(load.PingRTT, values...)
		if code != equipment.OK {
			r.probe.code = code
		}
		r.probeMu.Unlock()
		select {
		case r.probeReady <- struct{}{}:
		default:
		}
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func (r *radioSession) Probe() (equipment.RadioLoad, equipment.Code) {
	if r.probeReady == nil {
		return equipment.RadioLoad{}, equipment.Invalid
	}
	for {
		r.probeMu.Lock()
		value := r.probe
		if !value.load.Ended.IsZero() {
			r.probe = radioProbeState{}
			r.probeMu.Unlock()
			if value.code == "" {
				value.code = equipment.OK
			}
			return value.load, value.code
		}
		r.probeMu.Unlock()
		select {
		case <-r.ctx.Done():
			return equipment.RadioLoad{}, equipment.Cancelled
		case <-r.probeReady:
		}
	}
}

func (r *radioSession) run(command string) (string, float64, equipment.Code) {
	if c := r.check(); c != equipment.OK {
		r.Close()
		return "", 0, c
	}
	if r.ctx.Err() != nil {
		r.Close()
		return "", 0, equipment.Cancelled
	}
	started := time.Now()
	_ = r.conn.SetDeadline(started.Add(3 * time.Second))
	s, e := r.client.NewSession()
	if e != nil {
		r.Close()
		return "", time.Since(started).Seconds(), equipment.RadioCodeSSHConnectionLost
	}
	defer s.Close()
	budget := &outputBudget{conn: r.conn, limit: equipment.PPWireBytes}
	out, errout := &limitedOutput{budget: budget}, &limitedOutput{budget: budget}
	s.Stdout, s.Stderr = out, errout
	e = s.Run(command)
	seconds := time.Since(started).Seconds()
	_ = r.conn.SetDeadline(time.Time{})
	if c := r.check(); c != equipment.OK {
		r.Close()
		return "", seconds, c
	}
	if r.ctx.Err() != nil {
		r.Close()
		return "", seconds, equipment.Cancelled
	}
	if budget.overflow {
		r.Close()
		return "", seconds, equipment.Limit
	}
	if routeros.CollectorSyntaxDiagnostic(out.buf.String()) || routeros.CollectorSyntaxDiagnostic(errout.buf.String()) {
		r.Close()
		return "", seconds, equipment.CollectorDefect
	}
	if permissionDiagnostic(out.buf.String()) || permissionDiagnostic(errout.buf.String()) {
		r.Close()
		return "", seconds, equipment.RadioCodeRemotePermissionDenied
	}
	if e != nil || errout.buf.Len() != 0 {
		reason := "exit"
		if errout.buf.Len() != 0 {
			reason = "stderr"
		}
		r.failure = &equipment.RadioFailure{Reason: reason, Bytes: out.buf.Len() + errout.buf.Len()}
		r.Close()
		if seconds >= 3 {
			return "", seconds, equipment.RadioCodeDeadlineExceeded
		}
		return "", seconds, equipment.Partial
	}
	return out.buf.String(), seconds, equipment.OK
}

func (r *radioSession) Initial() (*equipment.RadioReading, equipment.Code) {
	if r.initial != nil {
		return nil, equipment.Invalid
	}
	command, parse := radioparse.RadioAPCommand, radioparse.ParseRadioAP
	if r.family == equipment.RadioStandalone || r.family == equipment.RadioAuto {
		return r.identifyStandalone()
	}
	if r.family == equipment.RadioRouterOSWirelessV6 && r.link == nil && (r.d.Role == equipment.RoleCPE || r.d.Role == equipment.RolePTP) {
		command, parse = radioparse.RadioStationCommand, radioparse.ParseRadioStationV6
	}
	if r.family == equipment.RadioAirOSV6 {
		command, parse = radioparse.RadioAirOSCommand, radioparse.ParseRadioAirOS
	}
	if r.family == equipment.RadioIntelbrasWOM {
		command, parse = radioparse.RadioWOMCommand, radioparse.ParseRadioWOM
	}
	text, seconds, c := r.run(command)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, c
	}
	reading, c := parse(text)
	if c != equipment.OK {
		r.Close()
		return reading, c
	}
	reading.CommandSeconds = seconds
	reading.Legacy = append([]string{}, r.legacy...)
	reading.Negotiated = append([]string{}, r.negotiated...)
	r.initial = reading
	return reading, equipment.OK
}

func (r *radioSession) Tick() (*equipment.RadioReading, equipment.Code) {
	if r.family == equipment.RadioRouterOSWirelessV6 && r.link == nil {
		return nil, equipment.Invalid
	}
	if r.initial == nil {
		r.initial = &equipment.RadioReading{Family: r.family, Perspective: "AP"}
	}
	command := radioparse.RadioAirOSCommand
	if r.family == equipment.RadioRouterOSWirelessV6 {
		command = radioparse.RadioAPTickCommand(r.link.MAC)
	}
	if r.family == equipment.RadioIntelbrasWOM {
		command = radioparse.RadioWOMTickCommand
	}
	text, seconds, c := r.run(command)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Scope: "link-tick", Failure: r.failure}, c
	}
	var reading *equipment.RadioReading
	switch r.family {
	case equipment.RadioRouterOSWirelessV6:
		reading, c = radioparse.ParseRadioAPTick(text, r.initial, r.link)
	case equipment.RadioAirOSV6:
		reading, c = radioparse.ParseRadioAirOS(text)
	case equipment.RadioIntelbrasWOM:
		reading, c = radioparse.ParseRadioWOM(text)
	}
	if c != equipment.OK {
		if reading == nil {
			reading = &equipment.RadioReading{}
		}
		reading.CommandSeconds, reading.Scope = seconds, "link-tick"
		return reading, c
	}
	reading.CommandSeconds = seconds
	reading.Legacy = append([]string{}, r.legacy...)
	reading.Negotiated = append([]string{}, r.negotiated...)
	reading.Scope = "link-tick"
	if reading.WirelessMAC == "" {
		reading.WirelessMAC = r.initial.WirelessMAC
	}
	return reading, equipment.OK
}

func (Adapter) ReadRadio(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	family equipment.RadioFamily,
	check func() equipment.Code) (*equipment.RadioReading,
	equipment.Code) {
	session, c := (Adapter{}).OpenRadioSession(ctx, d, user, secret, family, nil, check)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer session.Close()
	r, c := session.Initial()
	if c != equipment.OK {
		return r, c
	}
	if d.Role == equipment.RoleAP {
		load, probe := session.Probe()
		if probe == equipment.OK && len(load.PingRTT) > 0 {
			r.PingRTT = &load.PingRTT[len(load.PingRTT)-1]
		}
	}
	return r, c
}

func (Adapter) ReadRadioIndex(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	check func() equipment.Code) ([]equipment.RadioIndexEntry,
	equipment.Code) {
	if d.Role != equipment.RoleAP {
		return nil, equipment.Invalid
	}
	session, c := openRadioSession(ctx, d, user, secret, equipment.RadioRouterOSWirelessV6, nil, check, false)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer session.Close()
	// ignored: index parser uses stdout; execution code already includes stderr diagnostics
	text, _, c := session.run(radioparse.RadioIndexCommand)
	if c != equipment.OK {
		return nil, c
	}
	return radioparse.ParseRadioIndex(text)
}
