package radioparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
)

func ParseRadioAPTick(text string, initial *equipment.RadioReading, link *equipment.RadioLink) (*equipment.RadioReading, equipment.Code) {
	text = strings.ReplaceAll(text, "\r", "")
	if routeros.CollectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect
	}
	if len(text) > equipment.PPWireBytes || initial == nil || link == nil {
		return PartialReading("tick-frame", len(text))
	}
	lines, ok := radioAPTickFrame(text)
	if !ok {
		return PartialReading("tick-frame", len(text))
	}
	cpu, ok := radioAPTickCPU(lines[1])
	if !ok {
		return PartialReading("tick-cpu", len(text))
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
	failure := addRadioAPTickMetrics(&client.Metrics, peer.fields, len(text))
	client.Counters = radioPairCounters(peer.fields)
	appendRadioInitialNoise(initial, link.Interface, &client)
	return &equipment.RadioReading{
		Family: initial.Family, Version: initial.Version, Perspective: "AP", CPU: []int{cpu}, Failure: failure,
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
