package pppoeparse

import (
	"bufio"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"sort"
	"strconv"
	"strings"
)

// All property lists are literal allowlists. No command/path originates in JS.
// These scripts are derived from official documentation, not firmware acceptance.
var PDCatalogCommand = buildPDCatalog()
var PDRadiusCommand = buildPDRadius()

func pdPrefix(scope, count string) string {
	return strings.Replace(routeros.EscapeScript,
		`> 512`,
		`> 256`,
		1) +
		`; :local up [/system resource get uptime]; :local eligible (` +
		count +
		`); :put ("D\t1\t` +
		scope +
		`\t".[/system resource get version]."\t".$up."\t".$eligible); :lo` +

		`cal n 0; :local bytes 0; :local reason "complete"; `
}

func buildPDCatalog() string {
	body := pdPrefix(equipment.PDCatalog, `[/interface print count-only where !(dynamic=yes and type="pppoe`+
		`-in")]+[/interface vlan print count-only]+[/interface pppoe-serv`+
		`er server print count-only]`)
	body += routeros.FlagScript
	body += `:if ($eligible>2048) do={:set reason "rows"} else={:local ifs [/` +
		`interface find where !(dynamic=yes and type="pppoe-in")]; :local` +
		` vlans [/interface vlan find]; :local servers [/interface pppoe-` +
		`server server find]; `
	body += `:foreach i in=$ifs do={:if ($reason="complete") do={` +
		routeros.Emit(`"I\t".[$esc $i]."\t".[$esc [/interface get $i name]]."\t".[$esc `+

			`[/interface get $i type]]."\t".[$ppflag [/interface get $i runni`+
			`ng]]."\t".[/interface get $i disabled]."\t".[$ppflag [/interface`+
			` get $i dynamic]]`) + `}}; `
	body += `:foreach i in=$vlans do={:if ($reason="complete") do={` +
		routeros.Emit(`"V\t".[$esc $i]."\t".[$esc [/interface vlan get $i name]]."\t".[`+

			`$esc [/interface vlan get $i interface]]."\t".[/interface vlan g`+
			`et $i vlan-id]."\t".[/interface vlan get $i use-service-tag]."\t`+
			`".[$ppflag [/interface vlan get $i running]]."\t".[/interface vl`+
			`an get $i disabled]`) + `}}; `
	body += `:foreach i in=$servers do={:if ($reason="complete") do={:local r` +
		`anges "?"; :local untagged "?"; :do {:set ranges [:tostr [/inter` +
		`face pppoe-server server get $i pppoe-over-vlan-range]]} on-erro` +
		`r={}; :do {:set untagged [/interface pppoe-server server get $i ` +
		`accept-untagged]} on-error={}; ` + routeros.Emit(`"S\t".[$esc $i]."\t".[$esc [/interface pppoe-server server get $`+
		`i service-name]]."\t".[$esc [/interface pppoe-server server get `+
		`$i interface]]."\t".[/interface pppoe-server server get $i disab`+
		`led]."\t".[$ppflag [/interface pppoe-server server get $i invali`+
		`d]]."\t".[$esc $ranges]."\t".$untagged`) + `}}; `
	body += `:if (([:tostr $ifs]!=[:tostr [/interface find where !(dynamic=ye` +
		`s and type="pppoe-in")]])||([:tostr $vlans]!=[:tostr [/interface` +
		` vlan find]])||([:tostr $servers]!=[:tostr [/interface pppoe-ser` +
		`ver server find]])) do={:set reason "churn"}}; `
	return body + routeros.Footer()
}
func buildPDRadius() string {
	body := pdPrefix(equipment.PDRadius, `[/radius print count-only]+1`)
	body += `:if ($eligible>33) do={:set reason "rows"} else={:local ids [/ra` +
		`dius find]; ` + routeros.Emit(`"A\t".[/ppp aaa get use-radius]."\t".[/ppp aaa get accounting]`)
	body += `:foreach i in=$ids do={:if ($reason="complete") do={:local proto` +
		`col "?"; :do {:set protocol [/radius get $i protocol]} on-error=` +
		`{}; :local stats [/radius monitor $i once as-value]; `
	body += routeros.Emit(`"R\t".[$esc $i]."\t".[$esc [/radius get $i address]]."\t".[$esc ` +
		`[:tostr [/radius get $i service]]]."\t".[/radius get $i disabled` +
		`]."\t".[/radius get $i authentication-port]."\t".[/radius get $i` +
		` accounting-port]."\t".[$esc $protocol]."\t".($stats->"pending")` +
		`."\t".($stats->"requests")."\t".($stats->"accepts")."\t".($stats` +
		`->"rejects")."\t".($stats->"resends")."\t".($stats->"timeouts").` +
		`"\t".($stats->"bad-replies")."\t".($stats->"last-request-rtt")`)
	body += `}}; :if ([:tostr $ids]!=[:tostr [/radius find]]) do={:set reason "churn"}}; `
	return body + routeros.Footer()
}

func pdNumber(value string, max uint64) (uint64, bool) {
	if !routeros.Decimal(value) {
		return 0, false
	}
	n, err := strconv.ParseUint(value, 10, 64)
	return n, err == nil && n <= max
}

func ParsePDRange(value string) (string, bool) {
	if value == "?" || value == "" {
		return value, true
	}
	ranges := []pdRangeSpan{}
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
		ranges = append(ranges, pdRangeSpan{int(lo), int(hi)})
	}
	return mergePDRanges(ranges), true
}

type pdRangeSpan struct{ lo, hi int }

func mergePDRanges(ranges []pdRangeSpan) string {
	sort.Slice(ranges, func(i, j int) bool { return ranges[i].lo < ranges[j].lo })
	merged := []pdRangeSpan{}
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
	return strings.Join(parts, ",")
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
	if pdInputScopeOrSizeInvalid(scope, text) {
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
			var reason string
			eligible, reason = parsePDHeader(o, fields, scope, counts)
			if reason != "" {
				return fail(reason)
			}
			continue
		}
		if len(fields) > 0 && fields[0] == "Z" {
			if reason := parsePDFooter(o, fields, eligible, scope, seen); reason != "" {
				return fail(reason)
			}
			ended = true
			continue
		}
		if reason := appendPDRow(o, fields, scope, seen); reason != "" {
			return fail(reason)
		}
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

func pdInputScopeOrSizeInvalid(scope, text string) bool {
	return scope != equipment.PDCatalog && scope != equipment.PDRadius || len(text) > equipment.PPWireBytes
}

func appendPDRow(o *equipment.PDDiagnostic, fields []string, scope string, seen map[string]bool) string {
	if len(o.Rows) >= equipment.PPMaxRows || scope == equipment.PDRadius && len(o.Rows) >= equipment.PDMaxRadius+1 {
		return "rows"
	}
	for i := 1; i < len(fields); i++ {
		v, ok := routeros.Unescape(fields[i])
		if !ok || len(v) > 256 {
			return "codec"
		}
		fields[i] = v
	}
	row, ok := parsePDRow(scope, fields)
	if !ok {
		return "codec"
	}
	key := row.Kind + "/" + row.RemoteID
	if seen[key] {
		return "duplicate"
	}
	seen[key] = true
	row.ID = key
	o.Rows = append(o.Rows, row)
	return ""
}

func parsePDHeader(o *equipment.PDDiagnostic, fields []string, scope string, counts []uint64) (int, string) {
	if len(fields) != 6 || fields[0] != "D" || fields[1] != "1" || fields[2] != scope {
		return 0, "codec"
	}
	n, ok := pdNumber(fields[5], 1000000)
	if !ok {
		return 0, "codec"
	}
	if len(counts) != 0 {
		total := uint64(0)
		for _, count := range counts {
			total += count
		}
		if scope == equipment.PDRadius {
			total++
		} // The AAA configuration is one additional projected row.
		if total != n {
			return 0, "codec"
		}
	}
	eligible := int(n)
	family := ""
	if strings.HasPrefix(fields[3], "6.") {
		family = "v6"
	}
	if strings.HasPrefix(fields[3], "7.") {
		family = "v7"
	}
	if family == "" || len(fields[3]) > 128 {
		return 0, "firmware"
	}
	o.Provenance = equipment.PPProvenance{
		Projection: "diagnostic-" + scope + "-v1", Codec: "diagnostic-fields-v1",
		Capabilities: "projected-fields-v1", Family: family, Version: fields[3], RouterUptime: ParsePPDuration(fields[4]),
	}
	o.Quality.CountsKnown = true
	o.Quality.Eligible = eligible
	return eligible, ""
}

func parsePDFooter(o *equipment.PDDiagnostic, fields []string, eligible int, scope string, seen map[string]bool) string {
	if len(fields) != 4 {
		return "codec"
	}
	n, ok := pdNumber(fields[1], equipment.PPMaxRows)
	if !ok || int(n) != len(o.Rows) {
		return "codec"
	}
	o.Quality.Terminated = true
	o.Provenance.RouterFinalUptime = ParsePPDuration(fields[3])
	if fields[2] == "rows" || fields[2] == "wire_bytes" {
		o.Rows = nil
		o.Quality.Status = "limited"
		o.Quality.Code = equipment.Limit
		o.Quality.Reason = fields[2]
		return ""
	}
	if fields[2] != "complete" || len(o.Rows) != eligible {
		return "churn"
	}
	if scope == equipment.PDRadius && !seen["aaa/"] {
		return "codec"
	}
	o.Quality.Status = "complete"
	o.Quality.Code = equipment.OK
	o.Quality.MembershipChecked = true
	return ""
}
