package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"regexp"
	"strconv"
	"strings"
)

var radioAPMarkers = []string{
	"L07-CPU-BEFORE", "L07-INTERFACES", "L07-MONITORS", "L07-CPU-DURING",
	"L07-CLIENTS", "L07-CPU-AFTER", "L07-DONE",
}

func ParseRadioAP(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	text = strings.ReplaceAll(text, "\r", "")
	if collectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect
	}
	parts, ok := radioMarkerSections(text, radioAPMarkers)
	if !ok || len(parts) != 6 {
		return radioPartial("markers", len(text))
	}
	resource := radioColon(parts[0])
	if !strings.HasPrefix(resource["version"], "6.") {
		return nil, equipment.RadioCodeUnsupported
	}
	reading := &equipment.RadioReading{Family: equipment.RadioRouterOSWirelessV6, Version: resource["version"],
		Perspective: "AP", DeviceUptime: resource["uptime"], Interfaces: []equipment.RadioInterface{},
		Clients: []equipment.RadioClient{}, Metrics: []equipment.RadioMetric{}}
	if !parseRadioAPCPU(reading, resource, parts[3], parts[5]) {
		return radioPartial("cpu", len(text))
	}
	index, reason := parseRadioAPInterfaces(reading, parts[1])
	if reason != "" {
		return radioPartial(reason, len(text))
	}
	if reason = parseRadioAPMonitors(reading, index, parts[2]); reason != "" {
		return radioPartial(reason, len(text))
	}
	if reason = parseRadioAPClients(reading, index, parts[4]); reason != "" {
		return radioPartial(reason, len(text))
	}
	return reading, equipment.OK
}

func parseRadioAPCPU(reading *equipment.RadioReading, resource map[string]string, during, after string) bool {
	values := []string{resource["cpu-load"], strings.TrimSpace(strings.TrimPrefix(during, "cpu-load=")),
		strings.TrimSpace(strings.TrimPrefix(after, "cpu-load="))}
	for _, raw := range values {
		value, err := strconv.Atoi(strings.TrimSuffix(raw, "%"))
		if err != nil || value < 0 || value > 100 {
			return false
		}
		reading.CPU = append(reading.CPU, value)
	}
	return true
}

func parseRadioAPInterfaces(reading *equipment.RadioReading, text string) (map[string]int, string) {
	rows, valid := radioRecords(text)
	if !valid || len(rows) == 0 || len(rows) > 32 {
		return nil, "interface-count"
	}
	index := map[string]int{}
	for _, fields := range rows {
		name := fields["name"]
		if !equipment.ValidText(name, 80) {
			return nil, "interface-identity"
		}
		if _, exists := index[name]; exists {
			return nil, "interface-identity"
		}
		mac, valid := equipment.RadioMAC(fields["mac-address"])
		if !valid {
			return nil, "interface-identity"
		}
		iface := equipment.RadioInterface{Name: name, Master: fields["master-interface"], BSSID: mac,
			Band: fields["band"], ConfiguredWidth: fields["channel-width"], Metrics: []equipment.RadioMetric{}}
		if !addRadioMetric(&iface.Metrics, fields, "frequency", "frequency", "MHz", "configuration", 1) {
			return nil, "interface-metric"
		}
		index[name] = len(reading.Interfaces)
		reading.Interfaces = append(reading.Interfaces, iface)
	}
	return index, ""
}

func parseRadioAPMonitors(reading *equipment.RadioReading, index map[string]int, text string) string {
	blocks := strings.Split(text, "L07-INTERFACE=")
	if strings.TrimSpace(blocks[0]) != "" || len(blocks) != len(reading.Interfaces)+1 {
		return "monitor-count"
	}
	seen := map[string]bool{}
	for _, block := range blocks[1:] {
		name, body, found := strings.Cut(block, "\n")
		i, exists := index[name]
		if !found || !exists || seen[name] {
			return "monitor-identity"
		}
		seen[name] = true
		if reason := populateRadioAPMonitor(&reading.Interfaces[i], radioColon(body)); reason != "" {
			return reason
		}
	}
	return ""
}

func populateRadioAPMonitor(iface *equipment.RadioInterface, fields map[string]string) string {
	if fields["status"] != "running-ap" {
		return "monitor-status"
	}
	clients, err := strconv.Atoi(fields["registered-clients"])
	if err != nil || clients < 0 || clients > equipment.RadioMaxClients {
		return "client-count"
	}
	iface.Clients = &clients
	iface.Channel, iface.Power = fields["channel"], fields["current-tx-powers"]
	if !addRadioAPMonitorPower(iface) {
		return "monitor-metric"
	}
	if !addRadioMetric(&iface.Metrics, fields, "noise-floor", "noise", "dBm", "receive", 1) {
		return "monitor-metric"
	}
	if !addRadioMetric(&iface.Metrics, fields, "overall-tx-ccq", "ccq", "%", "AP → CPE", 1) {
		return "monitor-metric"
	}
	return ""
}

func addRadioAPMonitorPower(iface *equipment.RadioInterface) bool {
	pattern := regexp.MustCompile(`(?:^|,)([A-Za-z0-9-]+):(-?[0-9]+)\([^)]*\)`)
	for _, match := range pattern.FindAllStringSubmatch(iface.Power, -1) {
		metric, ok := radioMetric(match[2], "power_"+match[1], "dBm", "AP → CPE", 1)
		if !ok {
			return false
		}
		metric.Label = "Potência · " + match[1]
		iface.PowerMetrics = append(iface.PowerMetrics, metric)
	}
	return true
}

func parseRadioAPClients(reading *equipment.RadioReading, index map[string]int, text string) string {
	rows, valid := radioRecords(text)
	if !valid || len(rows) > equipment.RadioMaxClients {
		return "client-parse"
	}
	macs := map[string]bool{}
	counts := map[string]int{}
	for _, fields := range rows {
		mac, valid := equipment.RadioMAC(fields["mac-address"])
		i, exists := index[fields["interface"]]
		if !valid || !exists || macs[fields["interface"]+mac] {
			return "client-identity"
		}
		macs[fields["interface"]+mac] = true
		counts[fields["interface"]]++
		client := equipment.RadioClient{MAC: mac, Name: fields["radio-name"], Interface: fields["interface"],
			BSSID: reading.Interfaces[i].BSSID, Uptime: fields["uptime"], LastActivity: fields["last-activity"],
			Metrics: []equipment.RadioMetric{}}
		client.Counters = radioPairCounters(fields)
		if !addRadioAPClientMetrics(&client.Metrics, fields) {
			return "client-metric"
		}
		reading.Clients = append(reading.Clients, client)
	}
	return validateRadioAPRegistration(reading.Interfaces, index, counts)
}

func validateRadioAPRegistration(interfaces []equipment.RadioInterface, index map[string]int, counts map[string]int) string {
	for _, iface := range interfaces {
		if iface.Clients == nil || counts[iface.Name] != *iface.Clients {
			return "registration-count"
		}
		if iface.Master != "" {
			if _, exists := index[iface.Master]; !exists || iface.Master == iface.Name {
				return "master-interface"
			}
		}
	}
	return ""
}

func addRadioAPClientMetrics(metrics *[]equipment.RadioMetric, fields map[string]string) bool {
	for _, field := range []struct{ k, n, u, d string }{
		{"signal-strength", "signal", "dBm", "CPE → AP"},
		{"signal-strength-ch0", "chain0", "dBm", "CPE → AP"},
		{"signal-strength-ch1", "chain1", "dBm", "CPE → AP"},
		{"signal-to-noise", "snr", "dB", "CPE → AP"},
		{"tx-signal-strength", "remote_signal", "dBm", "AP → CPE"},
		{"tx-ccq", "ccq_tx", "%", "AP → CPE"}, {"rx-ccq", "ccq_rx", "%", "CPE → AP"},
		{"tx-rate", "rate_tx", "Mbps", "AP → CPE"}, {"rx-rate", "rate_rx", "Mbps", "CPE → AP"},
	} {
		if !addRadioMetric(metrics, fields, field.k, field.n, field.u, field.d, 1) {
			return false
		}
	}
	return true
}
