package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"strings"
)

// One bounded registration-table projection per explicit AP. No rich monitor,
// survey, CPU query, user-provided code, or remote background loop.
const RadioIndexCommand = radioTickEscape + (":put \"L07-INDEX\"; :local rows [/interface wireless registration-table print " +
	"stats as-value]; :if ([:len $rows]>512) do={:error \"field-limit\"}; :foreach " +
	"r in=$rows do={:local iface ($r->\"interface\"); :put \" 0 row=1\"; :foreach k" +
	" in={\"mac-address\";\"interface\";\"last-ip\";\"signal-strength\"} do={:put (" +
	"\"  \".$k.\"=\\\"\".[$esc ($r->$k)].\"\\\"\")}; :put (\"  bssid=\\\"\".[/inter" +
	"face wireless get [/interface wireless find where name=$iface] mac-address].\"" +
	"\\\"\")}; :put \"L07-INDEX-DONE\"")

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
	text, _, c := session.run(RadioIndexCommand)
	if c != equipment.OK {
		return nil, c
	}
	return ParseRadioIndex(text)
}
func ParseRadioIndex(text string) ([]equipment.RadioIndexEntry, equipment.Code) {
	text = strings.ReplaceAll(text, "\r", "")
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	if !strings.HasPrefix(text, "L07-INDEX\n") || !strings.HasSuffix(strings.TrimSpace(text), "\nL07-INDEX-DONE") {
		return nil, equipment.Partial
	}
	body := strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(text, "L07-INDEX\n")), "L07-INDEX-DONE")
	rows, ok := radioRecords(body)
	if !ok || len(rows) > equipment.RadioMaxClients {
		return nil, equipment.Partial
	}
	out := []equipment.RadioIndexEntry{}
	seen := map[string]bool{}
	for _, row := range rows {
		mac, mok := equipment.RadioMAC(row["mac-address"])
		bssid, bok := equipment.RadioMAC(row["bssid"])
		if !mok || !bok || !equipment.ValidText(row["interface"], 80) || seen[row["interface"]+mac] {
			return nil, equipment.Partial
		}
		seen[row["interface"]+mac] = true
		entry := equipment.RadioIndexEntry{MAC: mac, Interface: row["interface"], BSSID: bssid}
		if !populateRadioIndexMetrics(&entry, row) {
			return nil, equipment.Partial
		}
		out = append(out, entry)
	}
	return out, equipment.OK
}
