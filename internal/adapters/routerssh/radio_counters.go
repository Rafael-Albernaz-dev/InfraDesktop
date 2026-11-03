package routerssh

import (
	"strconv"
	"strings"
	"time"
)

func radioPairCounters(fields map[string]string) map[string]string {
	out := map[string]string{}
	if up := ParsePPDuration(fields["uptime"]); up.Known {
		out["uptime_seconds"] = strconv.FormatInt(up.Min/int64(time.Second), 10)
	}
	for _, key := range []string{"bytes", "packets", "frames", "hw-frames"} {
		parts := strings.FieldsFunc(fields[key], func(c rune) bool { return c == ',' || c == ';' })
		if len(parts) != 2 {
			continue
		}
		for i, side := range []string{"tx", "rx"} {
			if _, e := strconv.ParseUint(parts[i], 10, 64); e == nil {
				out[side+"_"+strings.ReplaceAll(key, "-", "_")] = parts[i]
			}
		}
	}
	return out
}
func radioAirCounters(fields map[string]string) map[string]string {
	out := map[string]string{}
	if raw, ok := fields["wlanUptime"]; ok {
		if _, e := strconv.ParseUint(raw, 10, 64); e == nil {
			out["uptime_seconds"] = raw
		}
	}
	fieldsByCounter := radioAirCounterFields()
	for key, wire := range fieldsByCounter {
		if raw, ok := fields[wire]; ok {
			if _, e := strconv.ParseUint(raw, 10, 64); e == nil {
				out[key] = raw
			}
		}
	}
	return out
}

func radioAirCounterFields() map[string]string {
	return map[string]string{
		"rx_bytes": "wlanRxBytes", "tx_bytes": "wlanTxBytes",
		"rx_packets": "wlanRxPackets", "tx_packets": "wlanTxPackets",
		"rx_errors": "wlanRxErrors", "tx_errors": "wlanTxErrors",
	}
}
