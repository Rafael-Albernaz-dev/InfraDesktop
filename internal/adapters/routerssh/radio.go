package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"math"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Executed on the real P02 v6 before integration. No user text is interpolated.
const RadioAPCommand = `:put "L07-CPU-BEFORE"; /system resource print; :put "L07-INTERFACES"; /interface wireless print detail without-paging; :put "L07-MONITORS"; :foreach w in=[/interface wireless find] do={:put ("L07-INTERFACE=" . [/interface wireless get $w name]); /interface wireless monitor $w once}; :put "L07-CPU-DURING"; :put ("cpu-load=" . [/system resource get cpu-load]); :put "L07-CLIENTS"; /interface wireless registration-table print stats detail without-paging; :put "L07-CPU-AFTER"; :put ("cpu-load=" . [/system resource get cpu-load]); :put "L07-DONE"`
const RadioAirOSCommand = `mca-status; cat /proc/uptime`
const RadioWOMCommand = `iw dev; iw dev wlan0 link; iw dev wlan0 station dump; cat /proc/uptime`
const RadioLoadCommand = `:put "L07-LOAD"; :put ("cpu=" . [/system resource get cpu-load]); :put "L07-LOAD-DONE"`

var radioAssignment = regexp.MustCompile(`([A-Za-z0-9_-]+)=("(?:\\.|[^"\\])*"|[^\s]+)`)

// RouterOS print detail aligns record indexes in the first three columns.
// Continuations are indented further and can themselves start with a number
// when strength-at-rates wraps between HT40- and its numeric suffix.
var radioRecord = regexp.MustCompile(`^[ \t]{0,2}[0-9]+\s`)
var radioNumber = regexp.MustCompile(`^(-?[0-9]+(?:\.[0-9]+)?)`)
var womSignal = regexp.MustCompile(`(?m)^\s*signal:\s*(-?[0-9]+)(?:\s+dBm)?\s*(?:\[([^\]]+)\])?`)
var womRate = regexp.MustCompile(`(?m)^\s*(tx|rx) bitrate:\s*([0-9.]+) MBit/s`)
var radioColonKey = regexp.MustCompile(`^[a-z][a-z-]*$`)
var radioProcFooter = regexp.MustCompile(`(?:^|\n)[0-9]+(?:\.[0-9]+)? [0-9]+(?:\.[0-9]+)?\s*$`)

func radioRecords(text string) ([]map[string]string, bool) {
	var records []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "Flags:") {
			continue
		}
		if radioRecord.MatchString(line) {
			records = append(records, line)
		} else {
			if len(records) == 0 {
				return nil, false
			}
			records[len(records)-1] += " " + strings.TrimSpace(line)
		}
	}
	out := make([]map[string]string, 0, len(records))
	for _, text := range records {
		fields := map[string]string{}
		for _, match := range radioAssignment.FindAllStringSubmatch(text, -1) {
			if _, exists := fields[match[1]]; exists {
				return nil, false
			}
			value := match[2]
			if strings.HasPrefix(value, `"`) {
				var err error
				value, err = strconv.Unquote(value)
				if err != nil {
					return nil, false
				}
			}
			fields[match[1]] = value
		}
		out = append(out, fields)
	}
	return out, true
}
func radioColon(text string) map[string]string {
	fields := map[string]string{}
	key := ""
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if k, v, found := strings.Cut(trim, ":"); found && radioColonKey.MatchString(k) {
			key = k
			fields[k] = strings.TrimSpace(v)
		} else if key != "" && trim != "" {
			fields[key] += trim
		}
	}
	return fields
}
func radioMetric(raw, name, unit, direction string, divisor float64) (equipment.RadioMetric, bool) {
	m := equipment.RadioMetric{Name: name, Unit: unit, Direction: direction}
	match := radioNumber.FindStringSubmatch(raw)
	if len(match) != 2 {
		return m, false
	}
	n, err := strconv.ParseFloat(match[1], 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return m, false
	}
	m.Value = n / divisor
	if unit == "dBm" && !strings.HasPrefix(name, "power_") && (m.Value < -150 || m.Value > 0) {
		return m, false
	}
	if unit == "dBm" && strings.HasPrefix(name, "power_") && (m.Value < -30 || m.Value > 70) {
		return m, false
	}
	if unit == "%" && (m.Value < 0 || m.Value > 100) {
		return m, false
	}
	if unit != "dBm" && m.Value < 0 {
		return m, false
	}
	return m, true
}
func addRadioMetric(out *[]equipment.RadioMetric, fields map[string]string, key, name, unit, direction string, divisor float64) bool {
	text, exists := fields[key]
	if !exists || text == "" {
		return true
	}
	m, valid := radioMetric(text, name, unit, direction, divisor)
	if !valid {
		return false
	}
	*out = append(*out, m)
	return true
}
func ParseRadioAP(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	text = strings.ReplaceAll(text, "\r", "")
	if collectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect
	}
	markers := []string{"L07-CPU-BEFORE", "L07-INTERFACES", "L07-MONITORS", "L07-CPU-DURING", "L07-CLIENTS", "L07-CPU-AFTER", "L07-DONE"}
	parts := make([]string, 0, len(markers))
	rest := strings.TrimSpace(text)
	for i, marker := range markers {
		if i == 0 {
			if !strings.HasPrefix(rest, marker+"\n") {
				return nil, equipment.Partial
			}
			rest = strings.TrimPrefix(rest, marker+"\n")
			continue
		}
		before, after, found := strings.Cut(rest, "\n"+marker)
		if !found {
			return nil, equipment.Partial
		}
		parts = append(parts, before)
		rest = strings.TrimPrefix(after, "\n")
	}
	if strings.TrimSpace(rest) != "" || len(parts) != 6 {
		return nil, equipment.Partial
	}
	resource := radioColon(parts[0])
	if !strings.HasPrefix(resource["version"], "6.") {
		return nil, "UNSUPPORTED"
	}
	r := &equipment.RadioReading{Family: "routeros-wireless-v6", Version: resource["version"], Perspective: "AP", DeviceUptime: resource["uptime"], Interfaces: []equipment.RadioInterface{}, Clients: []equipment.RadioClient{}, Metrics: []equipment.RadioMetric{}}
	for _, raw := range []string{resource["cpu-load"], strings.TrimSpace(strings.TrimPrefix(parts[3], "cpu-load=")), strings.TrimSpace(strings.TrimPrefix(parts[5], "cpu-load="))} {
		n, err := strconv.Atoi(strings.TrimSuffix(raw, "%"))
		if err != nil || n < 0 || n > 100 {
			return nil, equipment.Partial
		}
		r.CPU = append(r.CPU, n)
	}
	interfaces, valid := radioRecords(parts[1])
	if !valid || len(interfaces) == 0 || len(interfaces) > 32 {
		return nil, equipment.Partial
	}
	index := map[string]int{}
	for _, fields := range interfaces {
		name := fields["name"]
		if !equipment.ValidText(name, 80) {
			return nil, equipment.Partial
		}
		if _, exists := index[name]; exists {
			return nil, equipment.Partial
		}
		mac, valid := equipment.RadioMAC(fields["mac-address"])
		if !valid {
			return nil, equipment.Partial
		}
		iface := equipment.RadioInterface{Name: name, Master: fields["master-interface"], BSSID: mac, Band: fields["band"], ConfiguredWidth: fields["channel-width"], Metrics: []equipment.RadioMetric{}}
		if !addRadioMetric(&iface.Metrics, fields, "frequency", "frequency", "MHz", "configuration", 1) {
			return nil, equipment.Partial
		}
		index[name] = len(r.Interfaces)
		r.Interfaces = append(r.Interfaces, iface)
	}
	monitorParts := strings.Split(parts[2], "L07-INTERFACE=")
	if strings.TrimSpace(monitorParts[0]) != "" || len(monitorParts) != len(interfaces)+1 {
		return nil, equipment.Partial
	}
	seen := map[string]bool{}
	for _, block := range monitorParts[1:] {
		name, body, found := strings.Cut(block, "\n")
		i, exists := index[name]
		if !found || !exists || seen[name] {
			return nil, equipment.Partial
		}
		seen[name] = true
		fields := radioColon(body)
		iface := &r.Interfaces[i]
		if fields["status"] != "running-ap" {
			return nil, equipment.Partial
		}
		clients, err := strconv.Atoi(fields["registered-clients"])
		if err != nil || clients < 0 || clients > equipment.RadioMaxClients {
			return nil, equipment.Partial
		}
		iface.Clients = &clients
		iface.Channel, iface.Power = fields["channel"], fields["current-tx-powers"]
		for _, match := range regexp.MustCompile(`(?:^|,)([A-Za-z0-9-]+):(-?[0-9]+)\([^)]*\)`).FindAllStringSubmatch(iface.Power, -1) {
			m, ok := radioMetric(match[2], "power_"+match[1], "dBm", "AP → CPE", 1)
			if !ok {
				return nil, equipment.Partial
			}
			m.Label = "Potência · " + match[1]
			iface.PowerMetrics = append(iface.PowerMetrics, m)
		}
		if !addRadioMetric(&iface.Metrics, fields, "noise-floor", "noise", "dBm", "receive", 1) || !addRadioMetric(&iface.Metrics, fields, "overall-tx-ccq", "ccq", "%", "AP → CPE", 1) {
			return nil, equipment.Partial
		}
	}
	clients, valid := radioRecords(parts[4])
	if !valid || len(clients) > equipment.RadioMaxClients {
		return nil, equipment.Partial
	}
	macs := map[string]bool{}
	counts := map[string]int{}
	for _, fields := range clients {
		mac, valid := equipment.RadioMAC(fields["mac-address"])
		i, exists := index[fields["interface"]]
		if !valid || !exists || macs[fields["interface"]+mac] {
			return nil, equipment.Partial
		}
		macs[fields["interface"]+mac] = true
		counts[fields["interface"]]++
		client := equipment.RadioClient{MAC: mac, Name: fields["radio-name"], Interface: fields["interface"], BSSID: r.Interfaces[i].BSSID, Uptime: fields["uptime"], LastActivity: fields["last-activity"], Metrics: []equipment.RadioMetric{}}
		for _, f := range []struct{ k, n, u, d string }{{"signal-strength", "signal", "dBm", "CPE → AP"}, {"signal-strength-ch0", "chain0", "dBm", "CPE → AP"}, {"signal-strength-ch1", "chain1", "dBm", "CPE → AP"}, {"signal-to-noise", "snr", "dB", "CPE → AP"}, {"tx-signal-strength", "remote_signal", "dBm", "AP → CPE"}, {"tx-ccq", "ccq_tx", "%", "AP → CPE"}, {"rx-ccq", "ccq_rx", "%", "CPE → AP"}, {"tx-rate", "rate_tx", "Mbps", "AP → CPE"}, {"rx-rate", "rate_rx", "Mbps", "CPE → AP"}} {
			if !addRadioMetric(&client.Metrics, fields, f.k, f.n, f.u, f.d, 1) {
				return nil, equipment.Partial
			}
		}
		r.Clients = append(r.Clients, client)
	}
	for _, iface := range r.Interfaces {
		if iface.Clients == nil || counts[iface.Name] != *iface.Clients {
			return nil, equipment.Partial
		}
		if iface.Master != "" {
			if _, exists := index[iface.Master]; !exists || iface.Master == iface.Name {
				return nil, equipment.Partial
			}
		}
	}
	return r, equipment.OK
}
func ParseRadioAirOS(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	if !radioProcFooter.MatchString(strings.ReplaceAll(text, "\r", "")) {
		return nil, equipment.Partial
	}
	fields := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		for _, part := range strings.Split(line, ",") {
			if k, v, ok := strings.Cut(part, "="); ok {
				if _, exists := fields[k]; exists {
					return nil, equipment.Partial
				}
				fields[k] = v
			}
		}
	}
	version := fields["firmwareVersion"]
	if (!strings.HasPrefix(version, "XM.") && !strings.HasPrefix(version, "XW.")) || !strings.Contains(version, ".v6.") {
		return nil, "UNSUPPORTED"
	}
	mac, valid := equipment.RadioMAC(fields["deviceId"])
	if !valid {
		return nil, equipment.Partial
	}
	bssid, valid := equipment.RadioMAC(fields["apMac"])
	if !valid || fields["wlanOpmode"] != "sta" {
		return nil, equipment.Partial
	}
	r := &equipment.RadioReading{Family: "airos-v6", Version: version, Perspective: "CPE", APBSSID: bssid, WirelessMAC: mac, Metrics: []equipment.RadioMetric{}, Interfaces: []equipment.RadioInterface{}, Clients: []equipment.RadioClient{}}
	for _, f := range []struct {
		k, n, u, d string
		scale      float64
	}{{"signal", "signal", "dBm", "AP → CPE", 1}, {"chain0Signal", "chain0", "dBm", "AP → CPE", 1}, {"chain1Signal", "chain1", "dBm", "AP → CPE", 1}, {"noise", "noise", "dBm", "receive", 1}, {"ccq", "ccq", "%", "CPE → AP", 10}, {"freq", "frequency", "MHz", "receive", 1}, {"chanbw", "channel_width", "MHz", "receive", 1}, {"wlanTxRate", "rate_tx", "Mbps", "CPE → AP", 1}, {"wlanRxRate", "rate_rx", "Mbps", "AP → CPE", 1}, {"distance", "distance", "m", "link", 1}} {
		if !addRadioMetric(&r.Metrics, fields, f.k, f.n, f.u, f.d, f.scale) {
			return nil, equipment.Partial
		}
	}
	if fields["signal"] == "" {
		return nil, equipment.Partial
	}
	if raw, exists := fields["wlanUptime"]; exists {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, equipment.Partial
		}
		r.WirelessUptime = strconv.FormatUint(n, 10) + "s"
	}
	if raw, exists := fields["uptime"]; exists {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return nil, equipment.Partial
		}
		r.DeviceUptime = strconv.FormatUint(n, 10) + "s"
	}
	return r, equipment.OK
}
func ParseRadioWOM(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	if !radioProcFooter.MatchString(strings.ReplaceAll(text, "\r", "")) {
		return nil, equipment.Partial
	}
	connected := regexp.MustCompile(`(?m)^Connected to ([0-9a-fA-F:]{17}) \(on wlan0\)`).FindStringSubmatch(text)
	station := regexp.MustCompile(`(?m)^Station ([0-9a-fA-F:]{17}) \(on wlan0\)`).FindStringSubmatch(text)
	if len(connected) != 2 || len(station) != 2 || !strings.EqualFold(connected[1], station[1]) {
		return nil, equipment.Partial
	}
	bssid, valid := equipment.RadioMAC(connected[1])
	if !valid {
		return nil, equipment.Partial
	}
	r := &equipment.RadioReading{Family: "intelbras-wom", Version: "WOM / iw (firmware não exposto)", Perspective: "CPE", APBSSID: bssid, Metrics: []equipment.RadioMetric{}, Interfaces: []equipment.RadioInterface{}, Clients: []equipment.RadioClient{}}
	addr := regexp.MustCompile(`(?m)^\s*addr ([0-9a-fA-F:]{17})`).FindStringSubmatch(text)
	if len(addr) == 2 {
		r.WirelessMAC, _ = equipment.RadioMAC(addr[1])
	}
	linkText := strings.Split(text, "Station ")[0]
	signal := womSignal.FindStringSubmatch(linkText)
	if len(signal) < 2 {
		return nil, equipment.Partial
	}
	m, valid := radioMetric(signal[1], "signal", "dBm", "AP → CPE", 1)
	if !valid {
		return nil, equipment.Partial
	}
	r.Metrics = append(r.Metrics, m)
	if len(signal) > 2 && signal[2] != "" {
		for i, chain := range strings.Split(signal[2], ",") {
			if i >= 4 {
				return nil, equipment.Partial
			}
			m, valid := radioMetric(strings.TrimSpace(chain), "chain"+strconv.Itoa(i), "dBm", "AP → CPE", 1)
			if !valid {
				return nil, equipment.Partial
			}
			r.Metrics = append(r.Metrics, m)
		}
	}
	ccq := regexp.MustCompile(`(?m)^\s*CCQ:\s*([0-9.]+) %`).FindStringSubmatch(linkText)
	if len(ccq) == 2 {
		m, valid := radioMetric(ccq[1], "ccq", "%", "CPE → AP", 1)
		if !valid {
			return nil, equipment.Partial
		}
		r.Metrics = append(r.Metrics, m)
	}
	for _, match := range womRate.FindAllStringSubmatch(text, -1) {
		name := "rate_" + match[1]
		found := false
		for _, m := range r.Metrics {
			found = found || m.Name == name
		}
		if found {
			continue
		}
		direction := "CPE → AP"
		if match[1] == "rx" {
			direction = "AP → CPE"
		}
		m, valid := radioMetric(match[2], name, "Mbps", direction, 1)
		if !valid {
			return nil, equipment.Partial
		}
		r.Metrics = append(r.Metrics, m)
	}
	return r, equipment.OK
}
func (Adapter) ReadRadio(ctx context.Context, d equipment.Device, user, secret, family string, check func() equipment.Code) (*equipment.RadioReading, equipment.Code) {
	command := ""
	var parse func(string) (*equipment.RadioReading, equipment.Code)
	switch family {
	case "routeros-wireless-v6":
		if d.Role != "ap" {
			return nil, equipment.Invalid
		}
		command, parse = RadioAPCommand, ParseRadioAP
	case "airos-v6":
		if d.Role != "cpe" {
			return nil, equipment.Invalid
		}
		command, parse = RadioAirOSCommand, ParseRadioAirOS
	case "intelbras-wom":
		if d.Role != "cpe" {
			return nil, equipment.Invalid
		}
		command, parse = RadioWOMCommand, ParseRadioWOM
	default:
		return nil, "UNSUPPORTED"
	}
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	commandStarted := time.Now()
	text, c := connection.run(command)
	commandSeconds := time.Since(commandStarted).Seconds()
	if c != equipment.OK {
		return nil, c
	}
	r, c := parse(text)
	if c != equipment.OK {
		return nil, c
	}
	r.CommandSeconds = commandSeconds
	r.Legacy = append([]string{}, connection.legacy...)
	if family == "routeros-wireless-v6" {
		ping := exec.CommandContext(ctx, "/usr/bin/ping", "-n", "-c", "1", "-W", "1", d.Address)
		ping.Env = append(os.Environ(), "LC_ALL=C")
		if output, err := ping.Output(); err == nil {
			match := regexp.MustCompile(`time[=<]([0-9.]+) ms`).FindSubmatch(output)
			if len(match) == 2 {
				if value, err := strconv.ParseFloat(string(match[1]), 64); err == nil {
					r.PingRTT = &value
				}
			}
		}
	}
	return r, equipment.OK
}

// The observer reuses its extra AP session only for fixed, closed one-shot
// CPU frames. There is no remote loop to outlive cancellation. The local loop
// has a hard 12-second ceiling, independent of the normal read's QueryBudget.
func (Adapter) ObserveRadioLoad(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code, ready func(), sampleDone <-chan struct{}) (equipment.RadioLoad, equipment.Code) {
	load := equipment.RadioLoad{}
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return load, c
	}
	defer connection.close()
	probeCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		select {
		case <-probeCtx.Done():
			connection.close()
		case <-stopped:
		}
	}()
	defer close(stopped)
	load.Started = time.Now()
	collect := func() equipment.Code {
		if probeCtx.Err() != nil {
			return equipment.Cancelled
		}
		type pingResult struct {
			sent, received int
			rtt            []float64
			valid          bool
		}
		pingDone := make(chan pingResult, 1)
		go func() {
			ping := exec.CommandContext(probeCtx, "/usr/bin/ping", "-n", "-c", "1", "-W", "1", d.Address)
			ping.Env = append(os.Environ(), "LC_ALL=C")
			output, _ := ping.Output()
			counts := regexp.MustCompile(`([0-9]+) packets transmitted, ([0-9]+) received`).FindSubmatch(output)
			result := pingResult{valid: len(counts) == 3}
			if result.valid {
				result.sent, _ = strconv.Atoi(string(counts[1]))
				result.received, _ = strconv.Atoi(string(counts[2]))
			}
			for _, match := range regexp.MustCompile(`time[=<]([0-9.]+) ms`).FindAllSubmatch(output, -1) {
				value, e := strconv.ParseFloat(string(match[1]), 64)
				if e != nil {
					result.valid = false
				}
				result.rtt = append(result.rtt, value)
			}
			pingDone <- result
		}()
		text, c := connection.run(RadioLoadCommand)
		ping := <-pingDone
		if c != equipment.OK {
			return c
		}
		lines := strings.Fields(text)
		if len(lines) != 3 || lines[0] != "L07-LOAD" || lines[2] != "L07-LOAD-DONE" || !strings.HasPrefix(lines[1], "cpu=") {
			return equipment.Partial
		}
		cpu, e := strconv.Atoi(strings.TrimPrefix(lines[1], "cpu="))
		if e != nil || cpu < 0 || cpu > 100 || !ping.valid || ping.sent != 1 || ping.received < 0 || ping.received > 1 || len(ping.rtt) != ping.received {
			return equipment.Partial
		}
		load.CPU = append(load.CPU, cpu)
		load.PingSent += ping.sent
		load.PingReceived += ping.received
		load.PingRTT = append(load.PingRTT, ping.rtt...)
		return equipment.OK
	}
	if c := collect(); c != equipment.OK {
		return load, c
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	select {
	case <-probeCtx.Done():
		return load, equipment.Cancelled
	case <-ticker.C:
	}
	if c := collect(); c != equipment.OK {
		return load, c
	}
	ready()
	for {
		select {
		case <-probeCtx.Done():
			return load, equipment.Cancelled
		case <-sampleDone:
			if c := collect(); c != equipment.OK {
				return load, c
			}
			load.Ended = time.Now()
			return load, equipment.OK
		case <-ticker.C:
			if c := collect(); c != equipment.OK {
				return load, c
			}
		}
	}
}
