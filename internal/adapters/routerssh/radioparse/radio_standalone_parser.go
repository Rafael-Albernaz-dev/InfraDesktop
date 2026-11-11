package radioparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"regexp"
	"strings"
)

// RadioIdentificationCommand identifies RouterOS before any wireless command.
const RadioIdentificationCommand = (":put \"L07-ID-RESOURCE\"; /system resource print; :put \"L07-ID-PACKAGES\"; /s" +
	"ystem package print; :put \"L07-ID-BOARD\"; /system routerboard print; :put \"" +
	"L07-ID-DONE\"")

// RadioStationCommand uses only the read commands observed in the v6 station golden.
const RadioStationCommand = (":put \"L07-AVULSO-RESOURCE\"; /system resource print; :put \"L07-AVULSO-PACKAG" +
	"ES\"; /system package print; :put \"L07-AVULSO-BOARD\"; /system routerboard pr" +
	"int; :put \"L07-AVULSO-HEALTH\"; /system health print; :put \"L07-AVULSO-WIREL" +
	"ESS\"; /interface wireless print detail without-paging; :put \"L07-AVULSO-REGI" +
	"STRATION\"; /interface wireless registration-table print stats detail without-" +
	"paging; :put \"L07-AVULSO-DONE\"")

func ParseRadioStationV6(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	text = strings.ReplaceAll(text, "\r", "")
	if routeros.CollectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect
	}
	parts, ok := radioMarkerSections(text, radioStationMarkers)
	if !ok {
		return PartialReading("markers", len(text))
	}
	resource, board, code := radioStationIdentity(parts)
	if code == equipment.RadioCodeUnsupported {
		return nil, code
	}
	if code != equipment.OK {
		return PartialReading("identification", len(text))
	}
	fields, ok := radioSingleStationInterface(parts[4])
	if !ok {
		return PartialReading("interface-count", len(text))
	}
	if !radioStationMode(fields["mode"]) {
		return nil, equipment.RadioCodeUnsupported
	}
	local, ok := equipment.RadioMAC(fields["mac-address"])
	if !ok || fields["name"] == "" {
		return PartialReading("interface-identity", len(text))
	}
	reading := newRadioStationReading(resource, board, fields, local)
	if !appendRadioStationInterface(reading, fields, local) {
		return PartialReading("interface-metric", len(text))
	}
	return parseRadioStationPeer(parts[5], fields["name"], reading, len(text))
}

func radioStationIdentity(parts []string) (map[string]string, map[string]string, equipment.Code) {
	resource, board := radioColon(parts[0]), radioColon(parts[2])
	if resource["platform"] != "MikroTik" || !strings.HasPrefix(resource["version"], "6.") {
		return resource, board, equipment.RadioCodeUnsupported
	}
	if !regexp.MustCompile(`(?m)^\s*\d+\s+wireless\s+6\.`).MatchString(parts[1]) || board["model"] == "" {
		return resource, board, equipment.Partial
	}
	return resource, board, equipment.OK
}

func radioSingleStationInterface(text string) (map[string]string, bool) {
	interfaces, ok := radioRecords(text)
	if !ok || len(interfaces) != 1 {
		return nil, false
	}
	return interfaces[0], true
}

func radioStationMode(mode string) bool { return mode == "station" || mode == "station-bridge" }

func newRadioStationReading(resource, board, fields map[string]string, local string) *equipment.RadioReading {
	return &equipment.RadioReading{
		Family: equipment.RadioRouterOSWirelessV6, Manufacturer: "MikroTik",
		Version: resource["version"], Model: board["model"], Perspective: "CPE",
		Mode: fields["mode"], WirelessProtocol: fields["wireless-protocol"],
		DeviceUptime: resource["uptime"], WirelessMAC: local,
		Interfaces: []equipment.RadioInterface{}, Clients: []equipment.RadioClient{}, Metrics: []equipment.RadioMetric{},
	}
}

func appendRadioStationInterface(reading *equipment.RadioReading, fields map[string]string, local string) bool {
	iface := equipment.RadioInterface{
		Name: fields["name"], BSSID: local, Band: fields["band"], ConfiguredWidth: fields["channel-width"],
		Metrics: []equipment.RadioMetric{},
	}
	if !addRadioMetric(&iface.Metrics, fields, radioMetricSpec{"frequency", "frequency", "MHz", "configuration", 1}) {
		return false
	}
	reading.Interfaces = append(reading.Interfaces, iface)
	return true
}

func parseRadioStationPeer(
	text, interfaceName string,
	reading *equipment.RadioReading, wireBytes int,
) (*equipment.RadioReading, equipment.Code) {
	peers, ok := radioRecords(text)
	if !ok || len(peers) > 1 {
		return PartialReading("station-peers", wireBytes)
	}
	if len(peers) == 0 {
		return reading, equipment.OK
	}
	peer := peers[0]
	remote, ok := equipment.RadioMAC(peer["mac-address"])
	if !ok || peer["interface"] != interfaceName || peer["ap"] != "yes" {
		return PartialReading("station-peer-identity", wireBytes)
	}
	reading.APBSSID, reading.WirelessUptime = remote, peer["uptime"]
	if !addRadioStationPeerMetrics(&reading.Metrics, peer) {
		return PartialReading("station-metric", wireBytes)
	}
	reading.Clients = append(reading.Clients, equipment.RadioClient{
		MAC: remote, Name: peer["radio-name"], Interface: interfaceName, BSSID: remote,
		Uptime: peer["uptime"], LastActivity: peer["last-activity"], Metrics: []equipment.RadioMetric{},
	})
	return reading, equipment.OK
}

func addRadioStationPeerMetrics(metrics *[]equipment.RadioMetric, peer map[string]string) bool {
	for _, field := range []struct{ k, n, u, d string }{
		{"signal-strength", "signal", "dBm", "AP → CPE"},
		{"signal-strength-ch0", "chain0", "dBm", "AP → CPE"},
		{"signal-strength-ch1", "chain1", "dBm", "AP → CPE"},
		{"signal-to-noise", "snr", "dB", "receive"},
		{"tx-ccq", "ccq_tx", "%", "CPE → AP"}, {"rx-ccq", "ccq_rx", "%", "AP → CPE"},
		{"tx-rate", "rate_tx", "Mbps", "CPE → AP"}, {"rx-rate", "rate_rx", "Mbps", "AP → CPE"},
	} {
		if !addRadioMetric(metrics, peer, radioMetricSpec{field.k, field.n, field.u, field.d, 1}) {
			return false
		}
	}
	return true
}

func RadioStandaloneAPMode(text string) bool {
	parts, ok := radioMarkerSections(text, radioStationMarkers)
	if !ok {
		return false
	}
	fields := radioColon(parts[0])
	if fields["platform"] != "MikroTik" || !strings.HasPrefix(fields["version"], "6.") {
		return false
	}
	rows, valid := radioRecords(parts[4])
	if !valid || len(rows) != 1 {
		return false
	}
	return rows[0]["mode"] == "ap-bridge" || rows[0]["mode"] == "bridge"
}

func RadioStandaloneUnsupported(text string) *equipment.RadioReading {
	if len(text) > equipment.PPWireBytes || routeros.CollectorSyntaxDiagnostic(text) {
		return nil
	}
	parts, ok := radioMarkerSections(text, radioStationMarkers)
	if !ok {
		return nil
	}
	f, b := radioColon(parts[0]), radioColon(parts[2])
	if f["platform"] != "MikroTik" || !equipment.ValidText(f["version"], 160) || !equipment.ValidText(b["model"], 128) {
		return nil
	}
	return &equipment.RadioReading{Family: equipment.RadioUnsupported,
		Manufacturer: "MikroTik",
		DriverReason: "Versão ou modo RouterOS sem driver comprovado",
		Version:      f["version"],
		Model:        b["model"],
		Perspective:  "CPE",
		Metrics:      []equipment.RadioMetric{},
		Interfaces:   []equipment.RadioInterface{},
		Clients:      []equipment.RadioClient{}}
}

func ParseRadioIdentification(text string) (*equipment.RadioReading, bool, equipment.Code) {
	text = strings.ReplaceAll(text, "\r", "")
	if len(text) > equipment.PPWireBytes {
		return nil, false, equipment.Limit
	}
	if routeros.CollectorSyntaxDiagnostic(text) {
		return nil, false, equipment.CollectorDefect
	}
	parts, ok := radioMarkerSections(text, []string{"L07-ID-RESOURCE", "L07-ID-PACKAGES", "L07-ID-BOARD", "L07-ID-DONE"})
	if !ok {
		return nil, false, equipment.Partial
	}
	resource, packages, board := parts[0], parts[1], parts[2]
	f, b := radioColon(resource), radioColon(board)
	if f["platform"] != "MikroTik" || !equipment.ValidText(f["version"], 160) || !equipment.ValidText(b["model"], 128) {
		return nil, false, equipment.Partial
	}
	r := &equipment.RadioReading{Family: equipment.RadioUnsupported,
		Manufacturer: "MikroTik",
		Version:      f["version"],
		Model:        b["model"],
		DriverReason: "Versão ou pacote RouterOS sem driver comprovado",
		Perspective:  "CPE",
		Metrics:      []equipment.RadioMetric{},
		Interfaces:   []equipment.RadioInterface{},
		Clients:      []equipment.RadioClient{}}
	enabled := strings.HasPrefix(f["version"], "6.") && regexp.MustCompile(`(?m)^\s*\d+\s+wireless\s+6\.`).MatchString(packages)
	return r, enabled, equipment.OK
}

func RadioStandaloneWireless(text string) map[string]string {
	parts, ok := radioMarkerSections(text, radioStationMarkers)
	if !ok {
		return nil
	}
	rows, ok := radioRecords(parts[4])
	if !ok || len(rows) != 1 {
		return nil
	}
	return rows[0]
}

func IdentifyAirOS(text string) (*equipment.RadioReading, bool) {
	fields := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		for _, part := range strings.Split(line, ",") {
			k, v, ok := strings.Cut(part, "=")
			if !ok {
				continue
			}
			if k != "firmwareVersion" && k != "platform" && k != "wlanOpmode" {
				continue
			}
			if _, exists := fields[k]; exists {
				return nil, false
			}
			fields[k] = v
		}
	}
	version, model := fields["firmwareVersion"], fields["platform"]
	if !equipment.ValidText(version, 160) || !equipment.ValidText(model, 128) {
		return nil, false
	}
	r := &equipment.RadioReading{Family: equipment.RadioUnsupported,
		Manufacturer: "Ubiquiti",
		Version:      version,
		Model:        model,
		Mode:         fields["wlanOpmode"],
		DriverReason: "Firmware airOS sem driver comprovado",
		Perspective:  "CPE",
		Interfaces:   []equipment.RadioInterface{},
		Clients:      []equipment.RadioClient{},
		Metrics:      []equipment.RadioMetric{}}
	return r, true
}
