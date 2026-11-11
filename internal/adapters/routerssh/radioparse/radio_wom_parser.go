package radioparse

import (
	"infradesktop/internal/modules/equipment"
	"regexp"
	"strconv"
	"strings"
)

func ParseRadioWOM(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	if !radioProcFooter.MatchString(strings.ReplaceAll(text, "\r", "")) {
		return PartialReading("cpe-parse", len(text))
	}
	connected := regexp.MustCompile(`(?m)^Connected to ([0-9a-fA-F:]{17}) \(on wlan0\)`).FindStringSubmatch(text)
	station := regexp.MustCompile(`(?m)^Station ([0-9a-fA-F:]{17}) \(on wlan0\)`).FindStringSubmatch(text)
	if len(connected) != 2 || len(station) != 2 || !strings.EqualFold(connected[1], station[1]) {
		return PartialReading("cpe-parse", len(text))
	}
	bssid, valid := equipment.RadioMAC(connected[1])
	if !valid {
		return PartialReading("cpe-parse", len(text))
	}
	r := &equipment.RadioReading{Family: equipment.RadioIntelbrasWOM,
		Version:     "WOM / iw (firmware não exposto)",
		Perspective: "CPE",
		APBSSID:     bssid,
		Metrics:     []equipment.RadioMetric{},
		Interfaces:  []equipment.RadioInterface{},
		Clients:     []equipment.RadioClient{}}
	r.Counters = radioWOMCounters(text)
	addr := regexp.MustCompile(`(?m)^\s*addr ([0-9a-fA-F:]{17})`).FindStringSubmatch(text)
	if len(addr) == 2 {
		r.WirelessMAC, _ = equipment.RadioMAC(addr[1])
	}
	if !addRadioWOMSignal(r, strings.Split(text, "Station ")[0]) {
		return PartialReading("cpe-parse", len(text))
	}
	if !addRadioWOMRates(r, text) {
		return PartialReading("cpe-parse", len(text))
	}
	return r, equipment.OK
}
func addRadioWOMSignal(r *equipment.RadioReading, linkText string) bool {
	signal := womSignal.FindStringSubmatch(linkText)
	if len(signal) < 2 {
		return false
	}
	m, valid := radioMetric(signal[1], "signal", "dBm", "AP → CPE", 1)
	if !valid {
		return false
	}
	r.Metrics = append(r.Metrics, m)
	if len(signal) > 2 && signal[2] != "" {
		for i, chain := range strings.Split(signal[2], ",") {
			if i >= 4 {
				return false
			}
			m, valid := radioMetric(strings.TrimSpace(chain), "chain"+strconv.Itoa(i), "dBm", "AP → CPE", 1)
			if !valid {
				return false
			}
			r.Metrics = append(r.Metrics, m)
		}
	}
	ccq := regexp.MustCompile(`(?m)^\s*CCQ:\s*([0-9.]+) %`).FindStringSubmatch(linkText)
	if len(ccq) == 2 {
		m, valid := radioMetric(ccq[1], "ccq", "%", "CPE → AP", 1)
		if !valid {
			return false
		}
		r.Metrics = append(r.Metrics, m)
	}
	return true
}
func addRadioWOMRates(r *equipment.RadioReading, text string) bool {
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
			return false
		}
		r.Metrics = append(r.Metrics, m)
	}
	return true
}
