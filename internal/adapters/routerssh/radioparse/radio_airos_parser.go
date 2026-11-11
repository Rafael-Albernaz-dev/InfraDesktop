package radioparse

import (
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
)

func ParseRadioAirOS(text string) (*equipment.RadioReading, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	if !radioProcFooter.MatchString(strings.ReplaceAll(text, "\r", "")) {
		return PartialReading("cpe-parse", len(text))
	}
	fields, valid := radioAirOSFields(text)
	if !valid {
		return PartialReading("cpe-parse", len(text))
	}
	r, code := radioAirOSIdentity(fields)
	if code == equipment.Partial {
		return PartialReading("cpe-parse", len(text))
	}
	if code != equipment.OK {
		return r, code
	}
	r.Counters = radioAirCounters(fields)
	if !addRadioAirOSMetrics(r, fields) {
		return PartialReading("cpe-parse", len(text))
	}
	addRadioAirOSSNR(r, fields)
	if !addRadioMetric(&r.Metrics, fields, radioMetricSpec{"cpuUsage", "cpu", "%", "CPE", 1}) {
		return PartialReading("cpe-parse", len(text))
	}
	if !setRadioAirOSUptimes(r, fields) {
		return PartialReading("cpe-parse", len(text))
	}
	return r, equipment.OK
}
func radioAirOSFields(text string) (map[string]string, bool) {
	fields := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		for _, part := range strings.Split(line, ",") {
			if k, v, ok := strings.Cut(part, "="); ok {
				if _, exists := fields[k]; exists {
					return nil, false
				}
				fields[k] = v
			}
		}
	}
	return fields, true
}
func radioAirOSIdentity(fields map[string]string) (*equipment.RadioReading, equipment.Code) {
	version := fields["firmwareVersion"]

	mac, valid := equipment.RadioMAC(fields["deviceId"])
	if !valid {
		return nil, equipment.Partial
	}
	bssid, valid := equipment.RadioMAC(fields["apMac"])
	if !valid || fields["wlanOpmode"] != "sta" {
		return nil, equipment.Partial
	}
	if (!strings.HasPrefix(version, "XM.") && !strings.HasPrefix(version, "XW.")) || !strings.Contains(version, ".v6.") {
		// Identification only, never a metrics driver for this firmware. These
		// closed mca-status identity fields permit an explicit AP-only fallback.
		if !equipment.ValidText(version, 160) || !equipment.ValidText(fields["platform"], 128) {
			return nil, equipment.Partial
		}
		return &equipment.RadioReading{Family: equipment.RadioUnsupported,
				Version:     version,
				Model:       fields["platform"],
				Perspective: "CPE",
				APBSSID:     bssid,
				WirelessMAC: mac,
				Metrics:     []equipment.RadioMetric{},
				Interfaces:  []equipment.RadioInterface{},
				Clients:     []equipment.RadioClient{}},
			equipment.RadioCodeUnsupported
	}
	r := &equipment.RadioReading{Family: equipment.RadioAirOSV6,
		Version:     version,
		Model:       fields["platform"],
		Perspective: "CPE",
		APBSSID:     bssid,
		WirelessMAC: mac,
		Metrics:     []equipment.RadioMetric{},
		Interfaces:  []equipment.RadioInterface{},
		Clients:     []equipment.RadioClient{}}
	return r, equipment.OK
}
func addRadioAirOSMetrics(r *equipment.RadioReading, fields map[string]string) bool {
	specs := []struct {
		k, n, u, d string
		scale      float64
	}{
		{"signal", "signal", "dBm", "AP → CPE", 1},
		{"chain0Signal", "chain0", "dBm", "AP → CPE", 1},
		{"chain1Signal", "chain1", "dBm", "AP → CPE", 1},
		{"noise", "noise", "dBm", "receive", 1},
		{"ccq", "ccq", "%", "CPE → AP", 10},
		{"freq", "frequency", "MHz", "receive", 1},
		{"chanbw", "channel_width", "MHz", "receive", 1},
		{"wlanTxRate", "rate_tx", "Mbps", "CPE → AP", 1},
		{"wlanRxRate", "rate_rx", "Mbps", "AP → CPE", 1},
		{"distance", "distance", "m", "link", 1},
	}
	for _, f := range specs {
		if !addRadioMetric(&r.Metrics, fields, radioMetricSpec{f.k, f.n, f.u, f.d, f.scale}) {
			return false
		}
	}
	return fields["signal"] != ""
}
func addRadioAirOSSNR(r *equipment.RadioReading, fields map[string]string) {
	if signal, e1 := strconv.ParseFloat(fields["signal"], 64); e1 == nil {
		if noise, e2 := strconv.ParseFloat(fields["noise"], 64); e2 == nil && signal >= noise {
			r.Metrics = append(r.Metrics,
				equipment.RadioMetric{Name: "snr",
					Value:     signal - noise,
					Unit:      "dB",
					Direction: "AP → CPE",
					Quality:   "derived"})
		}
	}
}
func setRadioAirOSUptimes(r *equipment.RadioReading, fields map[string]string) bool {
	if raw, exists := fields["wlanUptime"]; exists {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return false
		}
		r.WirelessUptime = strconv.FormatUint(n, 10) + "s"
	}
	if raw, exists := fields["uptime"]; exists {
		n, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return false
		}
		r.DeviceUptime = strconv.FormatUint(n, 10) + "s"
	}
	return true
}
