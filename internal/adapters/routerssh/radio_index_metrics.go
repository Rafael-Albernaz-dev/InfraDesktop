package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"net/netip"
)

func populateRadioIndexMetrics(entry *equipment.RadioIndexEntry, row map[string]string) bool {
	if ip := row["last-ip"]; ip != "" {
		addr, e := netip.ParseAddr(ip)
		if e != nil || addr.Zone() != "" || addr.IsMulticast() {
			return false
		}
		if !addr.IsUnspecified() {
			entry.LastIP = addr.String()
		}
	}
	if signal := row["signal-strength"]; signal != "" {
		m, ok := radioMetric(signal, "signal", "dBm", "CPE → AP", 1)
		if !ok {
			return false
		}
		entry.Signal = &m.Value
	}
	return true
}
