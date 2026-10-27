package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

const PDPingContextV6 = `:put ("C\tv6\t".[/ip route vrf print count-only where disabled!=yes and invalid!=yes]."\t".[/ip route rule print count-only where disabled!=yes and invalid!=yes]."\t".[/ip firewall mangle print count-only where chain=output and disabled!=yes and invalid!=yes])`
const PDPingContextV7 = `:put ("C\tv7\t".[/ip vrf print count-only where name!="main" and disabled!=yes and invalid!=yes]."\t".[/routing rule print count-only where disabled!=yes and invalid!=yes]."\t".[/ip firewall mangle print count-only where chain=output and disabled!=yes and invalid!=yes])`

func pdContextSafe(family, text string) bool {
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
	case "v6":
		tables = `([/ip route vrf print count-only where disabled!=yes and invalid!=yes]>0)||([/ip route rule print count-only where disabled!=yes and invalid!=yes]>0)`
		argument = "routing-table=main"
		// ROS6 /tool ping resolves to ping-speed, not the ICMP command.
		pingCommand = "/ping"
	case "v7":
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

func collectPDPing(ctx context.Context, connection *ppConnection, target *equipment.PPPingTarget, check func() equipment.Code) (*equipment.PDDiagnostic, equipment.Code) {
	command, ok := BuildPDPingCommand(target)
	if !ok {
		return nil, equipment.ObservationIncompatible
	}
	family := target.Observation.Provenance.Family
	contextCommand := PDPingContextV7
	if family == "v6" {
		contextCommand = PDPingContextV6
	}
	text, c := connection.run(contextCommand)
	if c != equipment.OK {
		return nil, c
	}
	if !pdContextSafe(family, text) {
		return nil, "PING_CONTEXT_UNAVAILABLE"
	}
	if c = check(); c != equipment.OK {
		return nil, c
	}
	pingContext, cancel := context.WithTimeout(ctx, equipment.PDPingBudget)
	defer cancel()
	stop := context.AfterFunc(pingContext, connection.close)
	defer stop()
	connection.tighten(8 * 1024)
	text, c = connection.run(command)
	if pingContext.Err() != nil {
		return nil, "DEADLINE_EXCEEDED"
	}
	if c != equipment.OK {
		return nil, c
	}
	if strings.HasSuffix(strings.TrimSpace(text), "E\tcontext") {
		return nil, "PING_CONTEXT_UNAVAILABLE"
	}
	if strings.HasSuffix(strings.TrimSpace(text), "E\tsession") {
		return nil, "PING_SESSION_CHANGED"
	}
	result, c := ParsePDPing(text, 3, target.Row.Address)
	if c != equipment.OK {
		log.Printf("routerssh ping parsing: bytes=%d esc=%d lf=%d cr=%d headers=%d summaries=%d leading_blank=%t", len(text), strings.Count(text, "\x1b"), strings.Count(text, "\n"), strings.Count(text, "\r"), strings.Count(text, "SEQ"), strings.Count(text, "sent="), strings.HasPrefix(text, "\r") || strings.HasPrefix(text, "\n"))
		return nil, c
	}
	o := &equipment.PDDiagnostic{Scope: equipment.PDPing, Provenance: target.Observation.Provenance, Quality: equipment.PPQuality{Status: "complete", Code: equipment.OK, CountsKnown: true, Eligible: 1, Returned: 1, WireBytes: len(text), Terminated: true, MembershipChecked: true}, Rows: []equipment.PDRow{{Kind: "ping", Address: target.Row.Address, Ping: result}}}
	return o, equipment.OK
}

// ParsePDRTTInterval accepts explicitly ordered, unique units and exact decimal
// fractions. It preserves the last printed unit's quantization without floats.
func ParsePDRTTInterval(value string) equipment.PPInterval {
	if strings.Contains(value, ":") {
		v := ParsePPDuration(value)
		if v.Known && v.Max <= int64(equipment.QueryBudget) {
			return v
		}
		return equipment.PPInterval{}
	}
	if value == "" || len(value) > 80 {
		return equipment.PPInterval{}
	}
	total, previous, quantum := int64(0), int64(math.MaxInt64), int64(0)
	for value != "" {
		n := 0
		for n < len(value) && value[n] >= '0' && value[n] <= '9' {
			n++
		}
		if n == 0 {
			return equipment.PPInterval{}
		}
		whole := value[:n]
		fraction := ""
		value = value[n:]
		if strings.HasPrefix(value, ".") {
			value = value[1:]
			n = 0
			for n < len(value) && value[n] >= '0' && value[n] <= '9' {
				n++
			}
			if n == 0 {
				return equipment.PPInterval{}
			}
			fraction = value[:n]
			value = value[n:]
		}
		factor := int64(0)
		for _, unit := range []struct {
			name   string
			factor int64
		}{{"ms", int64(time.Millisecond)}, {"us", int64(time.Microsecond)}, {"ns", 1}, {"s", int64(time.Second)}} {
			if strings.HasPrefix(value, unit.name) {
				factor = unit.factor
				value = value[len(unit.name):]
				break
			}
		}
		if factor == 0 || factor >= previous {
			return equipment.PPInterval{}
		}
		previous = factor
		quantum = factor
		number, ok := pdNumber(whole, uint64(equipment.QueryBudget))
		if !ok || number > uint64(int64(equipment.QueryBudget)/factor) {
			return equipment.PPInterval{}
		}
		addition := int64(number) * factor
		if fraction != "" {
			for range fraction {
				quantum /= 10
			}
			if quantum < 1 {
				return equipment.PPInterval{}
			}
			decimal, ok := pdNumber(fraction, uint64(factor))
			if !ok {
				return equipment.PPInterval{}
			}
			addition += int64(decimal) * quantum
		}
		if addition > int64(equipment.QueryBudget)-total {
			return equipment.PPInterval{}
		}
		total += addition
	}
	if quantum < 1 || total > int64(equipment.QueryBudget)-quantum {
		return equipment.PPInterval{}
	}
	return equipment.PPInterval{Min: total, Max: total + quantum - 1, Known: true}
}

func ParsePDPing(text string, count int, address string) (*equipment.PDPingResult, equipment.Code) {
	if len(text) > 8*1024 || count < 1 || count > 5 {
		return nil, equipment.Partial
	}
	text, counts, ok := ppCountPrelude(text, 4)
	if !ok || len(counts) != 0 && (counts[0] != 0 || counts[1] != 0 || counts[2] != 0 || counts[3] != 1) {
		return nil, equipment.Partial
	}
	result := &equipment.PDPingResult{Context: "concentrador · main"}
	sequences := map[int]bool{}
	summary := map[string]string{}
	replies := 0
	header := false
	ended := false
	statuses := []string{"timeout", "net unreachable", "host unreachable", "admin prohibited", "TTL exceeded", "fragmentation needed and DF set", "packet too large and cannot be fragmented", "no route to host", "echo reply"}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// RouterOS may wrap the final summary between key=value fields to fit
		// the console width. Once it starts, only unique allowlisted summary
		// fields may follow; probe rows or another summary remain invalid.
		if ended || strings.HasPrefix(line, "sent=") {
			for _, pair := range strings.Fields(line) {
				fields := strings.Split(pair, "=")
				if len(fields) != 2 || fields[1] == "" || summary[fields[0]] != "" {
					return nil, equipment.Partial
				}
				switch fields[0] {
				case "sent", "received", "packet-loss", "min-rtt", "avg-rtt", "max-rtt":
				default:
					return nil, equipment.Partial
				}
				summary[fields[0]] = fields[1]
			}
			ended = true
			continue
		}
		if strings.HasPrefix(line, "SEQ ") {
			if header || len(result.Probes) > 0 {
				return nil, equipment.Partial
			}
			header = true
			continue
		}
		fields := strings.Fields(line)
		if !header || len(fields) < 2 || len(result.Probes) >= count {
			return nil, equipment.Partial
		}
		seq, ok := pdNumber(fields[0], uint64(count-1))
		if !ok || sequences[int(seq)] {
			return nil, equipment.Partial
		}
		sequences[int(seq)] = true
		probe := equipment.PDPingProbe{Sequence: int(seq)}
		for _, status := range statuses {
			if strings.HasSuffix(line, " "+status) {
				probe.Status = status
				break
			}
		}
		if len(fields) >= 5 {
			duration := ParsePDRTTInterval(fields[4])
			if duration.Known {
				value := duration.Min
				probe.RTT = &value
			}
		}
		if probe.Status == "" || probe.Status == "echo reply" {
			if len(fields) < 5 || fields[1] != address || probe.RTT == nil {
				return nil, equipment.Partial
			}
			if _, ok = pdNumber(fields[2], 65535); !ok {
				return nil, equipment.Partial
			}
			if _, ok = pdNumber(fields[3], 255); !ok {
				return nil, equipment.Partial
			}
			if probe.Status == "" && len(fields) != 5 {
				return nil, equipment.Partial
			}
			replies++
		}
		result.Probes = append(result.Probes, probe)
	}
	sent, ok := pdNumber(summary["sent"], uint64(count))
	if !ok || int(sent) != count || len(result.Probes) != count || !ended {
		return nil, equipment.Partial
	}
	received, ok := pdNumber(summary["received"], uint64(count))
	if !ok || int(received) != replies {
		return nil, equipment.Partial
	}
	loss := strings.TrimSuffix(summary["packet-loss"], "%")
	if loss == summary["packet-loss"] {
		return nil, equipment.Partial
	}
	parts := strings.Split(loss, ".")
	if len(parts) > 2 || !ppDecimal(parts[0]) || len(parts) == 2 && (!ppDecimal(parts[1]) || len(parts[1]) > 3) {
		return nil, equipment.Partial
	}
	reported, err := strconv.ParseFloat(loss, 64)
	if err != nil || reported < 0 || reported > 100 {
		return nil, equipment.Partial
	}
	result.Sent = int(sent)
	result.Received = int(received)
	result.Loss = float64(sent-received) * 100 / float64(sent)
	scale := 1.0
	if len(parts) == 2 {
		scale = math.Pow10(len(parts[1]))
	}
	if reported < math.Floor(result.Loss*scale)/scale || reported > math.Ceil(result.Loss*scale)/scale {
		return nil, equipment.Partial
	}
	if replies > 0 {
		values := []*int64{}
		for _, name := range []string{"min-rtt", "avg-rtt", "max-rtt"} {
			duration := ParsePDRTTInterval(summary[name])
			if !duration.Known {
				return nil, equipment.Partial
			}
			v := duration.Min
			values = append(values, &v)
		}
		result.Min, result.Avg, result.Max = values[0], values[1], values[2]
		if *result.Min > *result.Avg || *result.Avg > *result.Max {
			return nil, equipment.Partial
		}
	} else if summary["min-rtt"] != "" || summary["avg-rtt"] != "" || summary["max-rtt"] != "" {
		return nil, equipment.Partial
	}
	return result, equipment.OK
}
