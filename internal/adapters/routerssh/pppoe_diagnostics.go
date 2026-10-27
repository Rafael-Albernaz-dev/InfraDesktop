package routerssh

import (
	"bufio"
	"context"
	"infradesktop/internal/modules/equipment"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"
)

// All property lists are literal allowlists. No command/path originates in JS.
// These scripts are derived from official documentation, not firmware acceptance.
var PDCatalogCommand = buildPDCatalog()
var PDRadiusCommand = buildPDRadius()

func pdEmit(expression string) string {
	return `:local line (` + expression + `); :if (($bytes+[:len $line]+2)>458752) do={:set reason "wire_bytes"}; :if ($reason="complete") do={:put $line; :set bytes ($bytes+[:len $line]+2); :set n ($n+1)}; `
}
func pdPrefix(scope, count string) string {
	return strings.Replace(ppEscapeScript, `> 512`, `> 256`, 1) + `; :local up [/system resource get uptime]; :local eligible (` + count + `); :put ("D\t1\t` + scope + `\t".[/system resource get version]."\t".$up."\t".$eligible); :local n 0; :local bytes 0; :local reason "complete"; `
}
func pdFooter() string {
	return `:if ([/system resource get uptime]<$up) do={:set reason "churn"}; :put ("Z\t".$n."\t".$reason."\t".[/system resource get uptime]); `
}
func buildPDCatalog() string {
	body := pdPrefix(equipment.PDCatalog, `[/interface print count-only where !(dynamic=yes and type="pppoe-in")]+[/interface vlan print count-only]+[/interface pppoe-server server print count-only]`)
	body += routerFlagScript
	body += `:if ($eligible>2048) do={:set reason "rows"} else={:local ifs [/interface find where !(dynamic=yes and type="pppoe-in")]; :local vlans [/interface vlan find]; :local servers [/interface pppoe-server server find]; `
	body += `:foreach i in=$ifs do={:if ($reason="complete") do={` + pdEmit(`"I\t".[$esc $i]."\t".[$esc [/interface get $i name]]."\t".[$esc [/interface get $i type]]."\t".[$ppflag [/interface get $i running]]."\t".[/interface get $i disabled]."\t".[$ppflag [/interface get $i dynamic]]`) + `}}; `
	body += `:foreach i in=$vlans do={:if ($reason="complete") do={` + pdEmit(`"V\t".[$esc $i]."\t".[$esc [/interface vlan get $i name]]."\t".[$esc [/interface vlan get $i interface]]."\t".[/interface vlan get $i vlan-id]."\t".[/interface vlan get $i use-service-tag]."\t".[$ppflag [/interface vlan get $i running]]."\t".[/interface vlan get $i disabled]`) + `}}; `
	body += `:foreach i in=$servers do={:if ($reason="complete") do={:local ranges "?"; :local untagged "?"; :do {:set ranges [:tostr [/interface pppoe-server server get $i pppoe-over-vlan-range]]} on-error={}; :do {:set untagged [/interface pppoe-server server get $i accept-untagged]} on-error={}; ` + pdEmit(`"S\t".[$esc $i]."\t".[$esc [/interface pppoe-server server get $i service-name]]."\t".[$esc [/interface pppoe-server server get $i interface]]."\t".[/interface pppoe-server server get $i disabled]."\t".[$ppflag [/interface pppoe-server server get $i invalid]]."\t".[$esc $ranges]."\t".$untagged`) + `}}; `
	body += `:if (([:tostr $ifs]!=[:tostr [/interface find where !(dynamic=yes and type="pppoe-in")]])||([:tostr $vlans]!=[:tostr [/interface vlan find]])||([:tostr $servers]!=[:tostr [/interface pppoe-server server find]])) do={:set reason "churn"}}; `
	return body + pdFooter()
}
func buildPDRadius() string {
	body := pdPrefix(equipment.PDRadius, `[/radius print count-only]+1`)
	body += `:if ($eligible>33) do={:set reason "rows"} else={:local ids [/radius find]; ` + pdEmit(`"A\t".[/ppp aaa get use-radius]."\t".[/ppp aaa get accounting]`)
	body += `:foreach i in=$ids do={:if ($reason="complete") do={:local protocol "?"; :do {:set protocol [/radius get $i protocol]} on-error={}; :local stats [/radius monitor $i once as-value]; `
	body += pdEmit(`"R\t".[$esc $i]."\t".[$esc [/radius get $i address]]."\t".[$esc [:tostr [/radius get $i service]]]."\t".[/radius get $i disabled]."\t".[/radius get $i authentication-port]."\t".[/radius get $i accounting-port]."\t".[$esc $protocol]."\t".($stats->"pending")."\t".($stats->"requests")."\t".($stats->"accepts")."\t".($stats->"rejects")."\t".($stats->"resends")."\t".($stats->"timeouts")."\t".($stats->"bad-replies")."\t".($stats->"last-request-rtt")`)
	body += `}}; :if ([:tostr $ids]!=[:tostr [/radius find]]) do={:set reason "churn"}}; `
	return body + pdFooter()
}

func pdBool(value string, optional bool) (*bool, bool) {
	if optional && value == "?" {
		return nil, true
	}
	var result bool
	switch value {
	case "true", "yes":
		result = true
	case "false", "no":
		result = false
	default:
		return nil, false
	}
	return &result, true
}
func pdNumber(value string, max uint64) (uint64, bool) {
	if !ppDecimal(value) {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 10, 64)
	return n, err == nil && n <= max
}

func ParsePDRange(value string) (string, bool) {
	if value == "?" || value == "" {
		return value, true
	}
	type span struct{ lo, hi int }
	ranges := []span{}
	for _, part := range strings.Split(strings.ReplaceAll(value, ";", ","), ",") {
		ends := strings.Split(part, "-")
		if len(ends) > 2 {
			return "", false
		}
		lo, ok := pdNumber(ends[0], 4094)
		if !ok || lo == 0 {
			return "", false
		}
		hi := lo
		if len(ends) == 2 {
			hi, ok = pdNumber(ends[1], 4094)
			if !ok || hi < lo {
				return "", false
			}
		}
		ranges = append(ranges, span{int(lo), int(hi)})
	}
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })
	merged := []span{}
	for _, r := range ranges {
		n := len(merged)
		if n > 0 && r.lo <= merged[n-1].hi+1 {
			if r.hi > merged[n-1].hi {
				merged[n-1].hi = r.hi
			}
		} else {
			merged = append(merged, r)
		}
	}
	parts := []string{}
	for _, r := range merged {
		part := strconv.Itoa(r.lo)
		if r.hi != r.lo {
			part += "-" + strconv.Itoa(r.hi)
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ","), true
}

func ParsePPDiagnostic(scope, text string) (*equipment.PDDiagnostic, equipment.Code) {
	o := &equipment.PDDiagnostic{Scope: scope, Quality: equipment.PPQuality{Status: "partial", Code: equipment.Partial, WireBytes: len(text)}}
	fail := func(reason string) (*equipment.PDDiagnostic, equipment.Code) {
		o.Rows = nil
		o.Quality.Status = "partial"
		o.Quality.Code = equipment.Partial
		o.Quality.Reason = reason
		return o, equipment.OK
	}
	if scope != equipment.PDCatalog && scope != equipment.PDRadius || len(text) > equipment.PPWireBytes {
		return fail("wire_bytes")
	}
	countLines := 1
	if scope == equipment.PDCatalog {
		countLines = 3
	}
	text, counts, ok := ppCountPrelude(text, countLines)
	if !ok {
		return fail("codec")
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	line, eligible := 0, 0
	ended := false
	seen := map[string]bool{}
	for scanner.Scan() {
		fields := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		line++
		if ended {
			return fail("codec")
		}
		if line == 1 {
			if len(fields) != 6 || fields[0] != "D" || fields[1] != "1" || fields[2] != scope {
				return fail("codec")
			}
			n, ok := pdNumber(fields[5], 1000000)
			if !ok {
				return fail("codec")
			}
			if len(counts) != 0 {
				total := uint64(0)
				for _, count := range counts {
					total += count
				}
				if scope == equipment.PDRadius {
					total++ // The AAA configuration is one additional projected row.
				}
				if total != n {
					return fail("codec")
				}
			}
			eligible = int(n)
			family := ""
			if strings.HasPrefix(fields[3], "6.") {
				family = "v6"
			}
			if strings.HasPrefix(fields[3], "7.") {
				family = "v7"
			}
			if family == "" || len(fields[3]) > 128 {
				return fail("firmware")
			}
			o.Provenance = equipment.PPProvenance{Projection: "diagnostic-" + scope + "-v1", Codec: "diagnostic-fields-v1", Capabilities: "projected-fields-v1", Family: family, Version: fields[3], RouterUptime: ParsePPDuration(fields[4])}
			o.Quality.CountsKnown = true
			o.Quality.Eligible = eligible
			continue
		}
		if len(fields) > 0 && fields[0] == "Z" {
			if len(fields) != 4 {
				return fail("codec")
			}
			n, ok := pdNumber(fields[1], equipment.PPMaxRows)
			if !ok || int(n) != len(o.Rows) {
				return fail("codec")
			}
			ended = true
			o.Quality.Terminated = true
			o.Provenance.RouterFinalUptime = ParsePPDuration(fields[3])
			if fields[2] == "rows" || fields[2] == "wire_bytes" {
				o.Rows = nil
				o.Quality.Status = "limited"
				o.Quality.Code = equipment.Limit
				o.Quality.Reason = fields[2]
				continue
			}
			if fields[2] != "complete" || len(o.Rows) != eligible {
				return fail("churn")
			}
			if scope == equipment.PDRadius && !seen["aaa/"] {
				return fail("codec")
			}
			o.Quality.Status = "complete"
			o.Quality.Code = equipment.OK
			o.Quality.MembershipChecked = true
			continue
		}
		if len(o.Rows) >= equipment.PPMaxRows || scope == equipment.PDRadius && len(o.Rows) >= equipment.PDMaxRadius+1 {
			return fail("rows")
		}
		for i := 1; i < len(fields); i++ {
			v, ok := ppUnescape(fields[i])
			if !ok || len(v) > 256 {
				return fail("codec")
			}
			fields[i] = v
		}
		row, ok := parsePDRow(scope, fields)
		if !ok {
			return fail("codec")
		}
		key := row.Kind + "/" + row.RemoteID
		if seen[key] {
			return fail("duplicate")
		}
		seen[key] = true
		row.ID = key
		o.Rows = append(o.Rows, row)
	}
	if scanner.Err() != nil || !ended {
		return fail("codec")
	}
	if o.Quality.Status == "complete" {
		o.Count = len(o.Rows)
		o.Quality.Returned = o.Count
	}
	return o, equipment.OK
}

func parsePDRow(scope string, f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	if len(f) < 2 {
		return r, false
	}
	ok := true
	switch {
	case scope == equipment.PDCatalog && f[0] == "I" && len(f) == 7:
		r.Kind = "interface"
		r.RemoteID = f[1]
		r.Name = f[2]
		r.Type = f[3]
		r.Running, ok = pdBool(f[4], false)
		if !ok {
			return r, false
		}
		r.Disabled, ok = pdBool(f[5], false)
		if !ok {
			return r, false
		}
		r.Dynamic, ok = pdBool(f[6], false)
	case scope == equipment.PDCatalog && f[0] == "V" && len(f) == 8:
		r.Kind = "vlan"
		r.RemoteID = f[1]
		r.Name = f[2]
		r.Parent = f[3]
		n, valid := pdNumber(f[4], 4094)
		if !valid || n == 0 {
			return r, false
		}
		r.VLANID = int(n)
		r.ServiceTag, ok = pdBool(f[5], false)
		if !ok {
			return r, false
		}
		r.Running, ok = pdBool(f[6], false)
		if !ok {
			return r, false
		}
		r.Disabled, ok = pdBool(f[7], false)
	case scope == equipment.PDCatalog && f[0] == "S" && len(f) == 8:
		r.Kind = "service"
		r.RemoteID = f[1]
		r.Name = f[2]
		r.Parent = f[3]
		r.Disabled, ok = pdBool(f[4], false)
		if !ok {
			return r, false
		}
		r.Invalid, ok = pdBool(f[5], false)
		if !ok {
			return r, false
		}
		r.Range, ok = ParsePDRange(f[6])
		if !ok {
			return r, false
		}
		r.AcceptUntagged, ok = pdBool(f[7], true)
	case scope == equipment.PDRadius && f[0] == "A" && len(f) == 3:
		r.Kind = "aaa"
		r.UseRadius, ok = pdBool(f[1], false)
		if !ok {
			return r, false
		}
		r.Accounting, ok = pdBool(f[2], false)
		return r, ok
	case scope == equipment.PDRadius && f[0] == "R" && len(f) == 16:
		r.Kind = "radius"
		r.RemoteID = f[1]
		r.Address = f[2]
		r.Services = strings.ReplaceAll(f[3], ";", ",")
		r.Protocol = f[7]
		allowedService := map[string]bool{"ppp": true, "login": true, "hotspot": true, "wireless": true, "dhcp": true, "ipsec": true, "dot1x": true}
		if r.Services != "" {
			for _, service := range strings.Split(r.Services, ",") {
				if !allowedService[service] {
					r.Services = "desconhecido"
					break
				}
			}
		}
		if _, err := netip.ParseAddr(strings.Split(f[2], "@")[0]); err != nil {
			return r, false
		}
		if r.Protocol != "?" && r.Protocol != "udp" && r.Protocol != "radsec" {
			return r, false
		}
		r.Disabled, ok = pdBool(f[4], false)
		if !ok {
			return r, false
		}
		a, valid := pdNumber(f[5], 65535)
		if !valid || a == 0 {
			return r, false
		}
		b, valid := pdNumber(f[6], 65535)
		if !valid || b == 0 {
			return r, false
		}
		r.AuthPort = int(a)
		r.AccountingPort = int(b)
		c := &equipment.PDRadiusCounters{}
		values := []*string{&c.Pending, &c.Requests, &c.Accepts, &c.Rejects, &c.Resends, &c.Timeouts, &c.BadReplies}
		for i, p := range values {
			n, valid := pdNumber(f[i+8], ^uint64(0))
			if !valid {
				return r, false
			}
			*p = strconv.FormatUint(n, 10)
		}
		c.LastRTT = ParsePDRTTInterval(f[15])
		r.Counters = c
	default:
		return r, false
	}
	return r, ok && pdRemoteIdentity(r.RemoteID, true)
}

func (Adapter) CollectPPDiagnostic(ctx context.Context, d equipment.Device, user, secret, scope string, ping *equipment.PPPingTarget, check func() equipment.Code) (*equipment.PDDiagnostic, equipment.Code) {
	if !equipment.PDValidScope(scope) {
		return nil, equipment.Invalid
	}
	if scope == equipment.PDPing {
		if _, ok := BuildPDPingCommand(ping); !ok {
			return nil, equipment.ObservationIncompatible
		}
	}
	connection, c := openPPConnection(ctx, d, user, secret, check)
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	if scope == equipment.PDPing {
		return collectPDPing(ctx, connection, ping, check)
	}
	command := PDCatalogCommand
	if scope == equipment.PDRadius {
		command = PDRadiusCommand
	} else if scope != equipment.PDCatalog {
		return nil, equipment.Invalid
	}
	started := time.Now()
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	o, c := ParsePPDiagnostic(scope, text)
	if c == equipment.OK && o.Quality.Status == "complete" {
		if !ppCollectionStable(o.Provenance, time.Since(started)) {
			o.Rows = nil
			o.Quality.Status = "partial"
			o.Quality.Code = equipment.Partial
			o.Quality.Reason = "churn"
			o.Quality.MembershipChecked = false
		}
	}
	return o, c
}
