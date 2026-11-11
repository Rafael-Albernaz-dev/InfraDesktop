package pppoeparse

import (
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
	"time"
)

const PDPingContextV6 = `:put ("C\tv6\t".[/ip route vrf print count-only where disabled!=yes and invalid!=yes]."\t".[/ip route rule print count-only where disabled!=yes and invalid!=yes]."\t".[/ip firewall mangle print count-only where chain=output and disabled!=yes and invalid!=yes])`
const PDPingContextV7 = `:put ("C\tv7\t".[/ip vrf print count-only where name!="main" and disabled!=yes and invalid!=yes]."\t".[/routing rule print count-only where disabled!=yes and invalid!=yes]."\t".[/ip firewall mangle print count-only where chain=output and disabled!=yes and invalid!=yes])`

func PingContextSafe(family, text string) bool {
	text, counts, ok := ppCountPrelude(text, 3)
	if !ok {
		return false
	}
	fields := strings.Split(strings.TrimSpace(text), "\t")
	if len(fields) != 5 || fields[0] != "C" || fields[1] != family {
		return false
	}
	for i, value := range fields[2:] {
		n, ok := pdNumber(value, 1000000)
		if !ok || n != 0 || len(counts) != 0 && counts[i] != n {
			return false
		}
	}
	return true
}

func pdRemoteIdentity(value string, remote bool) bool {
	if remote {
		if !strings.HasPrefix(value, "*") || len(value) < 2 || len(value) > 17 {
			return false
		}
		for _, c := range value[1:] {
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return false
			}
		}
		return true
	}
	if len(value) == 0 || len(value) > 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c == '-' || c == ':' || c == '.' || c == '_') {
			return false
		}
	}
	return true
}
func BuildPDPingCommand(target *equipment.PPPingTarget) (string, bool) {
	if target == nil || !pdRemoteIdentity(target.Row.RemoteID, true) || !pdRemoteIdentity(target.Row.SessionID, false) || !target.Row.Uptime.Known || !target.Observation.Provenance.RouterUptime.Known {
		return "", false
	}
	address, ok := equipment.PPPingAddress(target.Row)
	if !ok {
		return "", false
	}
	family := target.Observation.Provenance.Family
	tables, argument, pingCommand := "", "", "/tool ping"
	switch family {
	case equipment.RouterFamilyV6:
		tables = `([/ip route vrf print count-only where disabled!=yes and invalid!=yes]>0)||([/ip route rule print count-only where disabled!=yes and invalid!=yes]>0)`
		argument = "routing-table=main"
		// ROS6 /tool ping resolves to ping-speed, not the ICMP command.
		pingCommand = "/ping"
	case equipment.RouterFamilyV7:
		tables = `([/ip vrf print count-only where name!="main" and disabled!=yes and invalid!=yes]>0)||([/routing rule print count-only where disabled!=yes and invalid!=yes]>0)`
		argument = "vrf=main"
	default:
		return "", false
	}
	routerBounds, sessionBounds, age, valid := equipment.PPPingBounds(target, time.Since(target.Prepared))
	if !valid {
		return "", false
	}
	seconds := func(n int64) string { return strconv.FormatInt(n/int64(time.Second), 10) + "s" }
	upper := func(n int64) string { return seconds(n + int64(time.Second) - 1) }
	guard := func(variable string, bounds equipment.PPInterval) string {
		return "($" + variable + "<" + seconds(bounds.Min) + ")||($" + variable + ">" + upper(bounds.Max) + ")||($" + variable + "<=" + upper(age.Max) + ")"
	}
	// RouterOS string literals escape dollars too: login is data, never script.
	login := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(target.Row.Login)
	command := `:if (` + tables + `||([/ip firewall mangle print count-only where chain=output and disabled!=yes and invalid!=yes]>0)) do={:put "E\tcontext"} else={:do {`
	id := target.Row.RemoteID
	command += `:local ru [/system resource get uptime]; :local su [/ppp active get ` + id + ` uptime]; :if (([:tostr [/ppp active get ` + id + ` session-id]]!="` + target.Row.SessionID + `")||([/ppp active get ` + id + ` name]!="` + login + `")||([/ppp active get ` + id + ` address]!=` + address + `)||([/ppp active print count-only where address=` + address + `]!=1)||` + guard("ru", routerBounds) + `||` + guard("su", sessionBounds) + `) do={:put "E\tsession"} else={`
	command += pingCommand + ` address=` + address + ` count=3 interval=200ms size=64 ttl=64 ` + argument + `}} on-error={:put "E\tsession"}}`

	return command, true
}
