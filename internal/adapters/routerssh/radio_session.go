package routerssh

import (
	"context"
	"errors"
	"golang.org/x/crypto/ssh"
	"infradesktop/internal/modules/equipment"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
	if collectorSyntaxDiagnostic(out.buf.String()) || collectorSyntaxDiagnostic(errout.buf.String()) {
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
	command, parse := RadioAPCommand, ParseRadioAP
	if r.family == equipment.RadioStandalone || r.family == equipment.RadioAuto {
		return r.identifyStandalone()
	}
	if r.family == equipment.RadioRouterOSWirelessV6 && r.link == nil && (r.d.Role == equipment.RoleCPE || r.d.Role == equipment.RolePTP) {
		command, parse = RadioStationCommand, ParseRadioStationV6
	}
	if r.family == equipment.RadioAirOSV6 {
		command, parse = RadioAirOSCommand, ParseRadioAirOS
	}
	if r.family == equipment.RadioIntelbrasWOM {
		command, parse = RadioWOMCommand, ParseRadioWOM
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

// Project only the selected registration's used fields. Escape device data as
// quoted values; no name or interface supplied by the UI becomes RouterOS code.
const radioTickEscape = (":local esc do={:local v [:tostr $1]; :local out \"\"; :if ([:len $v]>256) do={" +
	":error \"field-limit\"}; :if ([:len $v]=0) do={:return \"\"}; :for n from=0 to" +
	"=([:len $v]-1) do={:local c [:pick $v $n ($n+1)]; :if (($c=\"\\\\\")||($c=\"\\" +
	"\"\")) do={:set c (\"\\\\\".$c)}; :if ($c=\"\\n\") do={:set c \"\\\\n\"}; :if " +
	"($c=\"\\r\") do={:set c \"\\\\r\"}; :if ($c=\"\\t\") do={:set c \"\\\\t\"}; :s" +
	"et out ($out.$c)}; :return $out}; ")

func RadioAPTickCommand(mac string) string {
	mac, ok := equipment.RadioMAC(mac)
	if !ok {
		return ""
	}
	return radioTickEscape + (":put \"L07-TICK\"; :put (\"cpu=\" . [/system resource get cpu-load]); :local i" +
		"ds [/interface wireless registration-table find where mac-address=") +
		mac + ("]; :if ([:len $ids]=1) do={:local iface [/interface wireless registration-tabl" +
		"e get $ids interface]; :put (\"bssid=\" . [/interface wireless get [/interface" +
		" wireless find where name=$iface] mac-address])} else={:put \"bssid=unconfirme" +
		"d\"}; :put \"L07-CLIENTS\"; :local rows [/interface wireless registration-tabl" +
		"e print stats as-value where mac-address=") +
		mac + ("]; :if ([:len $rows]=1) do={:foreach r in=$rows do={:put \" 0 row=1\"; :foreac" +
		"h k in={\"interface\";\"mac-address\";\"uptime\";\"last-activity\";\"signal-st" +
		"rength\";\"signal-strength-ch0\";\"signal-strength-ch1\";\"signal-to-noise\";" +
		"\"tx-signal-strength\";\"tx-ccq\";\"rx-ccq\";\"tx-rate\";\"rx-rate\";\"bytes\"" +
		";\"packets\";\"frames\";\"hw-frames\"} do={:local v ($r->$k); :if ([:typeof $v" +
		"]!=\"nil\") do={:put (\"  \".$k.\"=\\\"\".[$esc $v].\"\\\"\")}}}}; :put \"L07-" +
		"TICK-DONE\"")
}

const RadioWOMTickCommand = RadioWOMCommand

func (r *radioSession) Tick() (*equipment.RadioReading, equipment.Code) {
	if r.family == equipment.RadioRouterOSWirelessV6 && r.link == nil {
		return nil, equipment.Invalid
	}
	if r.initial == nil {
		r.initial = &equipment.RadioReading{Family: r.family, Perspective: "AP"}
	}
	command := RadioAirOSCommand
	if r.family == equipment.RadioRouterOSWirelessV6 {
		command = RadioAPTickCommand(r.link.MAC)
	}
	if r.family == equipment.RadioIntelbrasWOM {
		command = RadioWOMTickCommand
	}
	text, seconds, c := r.run(command)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Scope: "link-tick", Failure: r.failure}, c
	}
	var reading *equipment.RadioReading
	switch r.family {
	case equipment.RadioRouterOSWirelessV6:
		reading, c = ParseRadioAPTick(text, r.initial, r.link)
	case equipment.RadioAirOSV6:
		reading, c = ParseRadioAirOS(text)
	case equipment.RadioIntelbrasWOM:
		reading, c = ParseRadioWOM(text)
	}
	if c != equipment.OK {
		r.Close()
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

func ParseRadioAPTick(text string, initial *equipment.RadioReading, link *equipment.RadioLink) (*equipment.RadioReading, equipment.Code) {
	text = strings.ReplaceAll(text, "\r", "")
	if collectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect
	}
	if len(text) > equipment.PPWireBytes || initial == nil || link == nil {
		return radioPartial("tick-parse", len(text))
	}
	lines, ok := radioAPTickFrame(text)
	if !ok {
		return radioPartial("tick-parse", len(text))
	}
	cpu, ok := radioAPTickCPU(lines[1])
	if !ok {
		return radioPartial("tick-parse", len(text))
	}
	peer, ok := parseRadioAPTickPeer(lines, link)
	if !ok {
		return nil, equipment.RadioCodeLinkUnconfirmed
	}
	client := equipment.RadioClient{
		MAC: peer.mac, Interface: peer.fields["interface"], BSSID: peer.bssid,
		Uptime: peer.fields["uptime"], LastActivity: peer.fields["last-activity"],
		Name: peer.fields["radio-name"], Metrics: []equipment.RadioMetric{},
	}
	if !addRadioAPClientMetrics(&client.Metrics, peer.fields) {
		return radioPartial("tick-parse", len(text))
	}
	client.Counters = radioPairCounters(peer.fields)
	appendRadioInitialNoise(initial, link.Interface, &client)
	return &equipment.RadioReading{
		Family: initial.Family, Version: initial.Version, Perspective: "AP", CPU: []int{cpu},
		Interfaces: []equipment.RadioInterface{}, Clients: []equipment.RadioClient{client}, Metrics: []equipment.RadioMetric{},
	}, equipment.OK
}

type radioTickPeer struct {
	fields     map[string]string
	mac, bssid string
}

func radioAPTickFrame(text string) ([]string, bool) {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return lines, len(lines) >= 5 && lines[0] == "L07-TICK" && lines[3] == "L07-CLIENTS" && lines[len(lines)-1] == "L07-TICK-DONE"
}

func radioAPTickCPU(line string) (int, bool) {
	cpu, err := strconv.Atoi(strings.TrimPrefix(line, "cpu="))
	return cpu, err == nil && strings.HasPrefix(line, "cpu=") && cpu >= 0 && cpu <= 100
}

func parseRadioAPTickPeer(lines []string, link *equipment.RadioLink) (radioTickPeer, bool) {
	bssid, ok := equipment.RadioMAC(strings.TrimPrefix(lines[2], "bssid="))
	if !ok || !strings.HasPrefix(lines[2], "bssid=") || bssid != link.BSSID {
		return radioTickPeer{}, false
	}
	fields, ok := radioRecords(strings.Join(lines[4:len(lines)-1], "\n"))
	if !ok || len(fields) != 1 {
		return radioTickPeer{}, false
	}
	peer := fields[0]
	mac, ok := equipment.RadioMAC(peer["mac-address"])
	if !ok || mac != link.MAC || peer["interface"] != link.Interface {
		return radioTickPeer{}, false
	}
	return radioTickPeer{fields: peer, mac: mac, bssid: bssid}, true
}

func appendRadioInitialNoise(initial *equipment.RadioReading, interfaceName string, client *equipment.RadioClient) {
	for _, iface := range initial.Interfaces {
		if iface.Name != interfaceName {
			continue
		}
		for _, metric := range iface.Metrics {
			if metric.Name == "noise" {
				metric.Quality = "initial"
				client.Metrics = append(client.Metrics, metric)
			}
		}
	}
}
