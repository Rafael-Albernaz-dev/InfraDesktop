package pppoeparse

import (
	"bufio"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
)

// BuildPPBandCommand generates the bounded RouterOS monitor command from service rows.
func BuildPPBandCommand(rows []equipment.PPRow) (string, bool) {
	if len(rows) > equipment.PageSize {
		return "", false
	}
	selection, ok := buildPPBandSelection(rows)
	if !ok {
		return "", false
	}
	find := `[/interface pppoe-server find where running=yes]`
	if selection != "" {
		find = `[/interface pppoe-server find where running=yes and (` + selection + `)]`
	}
	s := routeros.EscapeScript +
		`; :local up [/system resource get uptime]; :local cpu "?"; ` +
		`:do {:set cpu [/system resource get cpu-load]} on-error={}; ` +
		`:put ("B\t1\t".[/system resource get version]."\t".$up."\t".$cpu); :local ids ` +
		find +
		`; :local n 0; :local bytes 0; :local reason "complete"; ` +
		`:if ([:len $ids]>2048) do={:set reason "rows"} else={` +
		`:foreach i in=$ids do={:if ($reason="complete") do={:do {`
	s += `:local m [/interface pppoe-server monitor $i once as-value]; ` +
		`:local ppBandName [/interface pppoe-server get $i name]; :local rx "?"; :local tx "?"; ` +
		`:local ifs [/interface find where name=$ppBandName and type="pppoe-in"]; ` +
		`:if ([:len $ifs]=1) do={:local p [:pick $ifs 0]; ` +
		`:do {:set rx [/interface get $p rx-byte]; :set tx [/interface get $p tx-byte]} ` +
		`on-error={:set rx "?"; :set tx "?"}}; ` +
		`:local after [/interface pppoe-server monitor $i once as-value]; ` +
		`:if ((($after->"user")!=($m->"user"))||(($after->"caller-id")!=($m->"caller-id"))||` +
		`(($after->"remote-address")!=($m->"remote-address"))||(($after->"uptime")<($m->"uptime"))) ` +
		`do={:set rx "?"; :set tx "?"}; `
	fields := []string{
		`$i`, `$ppBandName`, `($m->"user")`, `($m->"caller-id")`,
		`($m->"remote-address")`, `($m->"uptime")`, `$rx`, `$tx`,
	}
	expr := `"T"`
	for _, field := range fields {
		expr += `."\t".[$esc ` + field + `]`
	}
	s += routeros.Emit(expr) + `} on-error={}}}}; ` + routeros.Footer()
	return s, true
}

func buildPPBandSelection(rows []equipment.PPRow) (string, bool) {
	selection := ""
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Login == "" || len(row.Login) > 256 {
			return "", false
		}
		if !seen[row.Login] {
			if selection != "" {
				selection += " or "
			}
			selection += "user=" + routeros.Literal(row.Login)
			seen[row.Login] = true
		}
	}
	return selection, true
}

func ParsePPBand(text string) ([]equipment.PPTunnel, string, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, "", equipment.Limit
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	st := &ppBandParseState{seen: map[string]bool{}, out: []equipment.PPTunnel{}}
	for scanner.Scan() {
		f := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		if code := st.consumeLine(f); code != equipment.OK {
			return nil, st.version, code
		}
	}
	if scanner.Err() != nil || !st.ended {
		return nil, st.version, equipment.Partial
	}
	return st.out, st.version, equipment.OK
}

type ppBandParseState struct {
	version string
	up      equipment.PPInterval
	cpu     *uint64
	out     []equipment.PPTunnel
	ended   bool
	seen    map[string]bool
}

func (st *ppBandParseState) consumeLine(f []string) equipment.Code {
	if st.ended {
		return equipment.Partial
	}
	if st.version == "" {
		return st.parseHeader(f)
	}
	if f[0] == "Z" {
		return st.parseFooter(f)
	}
	return st.parseTunnelRow(f)
}

func (st *ppBandParseState) parseHeader(f []string) equipment.Code {
	if (len(f) != 4 && len(f) != 5) || f[0] != "B" || f[1] != "1" {
		return equipment.Partial
	}
	if !strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.") {
		return equipment.Partial
	}
	st.version = f[2]
	if len(st.version) > 128 {
		return equipment.Partial
	}
	// ignored: validate escaping while retaining the original version token
	if _, ok := routeros.Unescape(st.version); !ok {
		return equipment.Partial
	}
	st.up = ParsePPDuration(f[3])
	if len(f) == 5 {
		st.cpu = routeros.Counter(f[4])
		if st.cpu != nil && *st.cpu > 100 {
			st.cpu = nil
		}
	}
	return equipment.OK
}

func (st *ppBandParseState) parseFooter(f []string) equipment.Code {
	if len(f) != 4 {
		return equipment.Partial
	}
	n, e := strconv.Atoi(f[1])
	final := ParsePPDuration(f[3])
	if e != nil || n != len(st.out) || f[2] != "complete" || !st.up.Known || !final.Known || final.Max < st.up.Min {
		return equipment.Partial
	}
	st.ended = true
	return equipment.OK
}

func (st *ppBandParseState) parseTunnelRow(f []string) equipment.Code {
	if len(f) != 9 || f[0] != "T" || len(st.out) >= equipment.PPMaxRows {
		return equipment.Partial
	}
	for i := 1; i < len(f); i++ {
		v, ok := routeros.Unescape(f[i])
		if !ok || len(v) > 256 {
			return equipment.Partial
		}
		f[i] = v
	}
	if f[1] == "" || st.seen[f[1]] {
		return equipment.Partial
	}
	st.seen[f[1]] = true
	st.out = append(st.out, equipment.PPTunnel{
		ID: f[1], Name: f[2], Login: f[3], Caller: f[4], Address: f[5],
		Uptime: ParsePPDuration(f[6]), RouterUptime: st.up, RouterCPU: st.cpu,
		RX: routeros.Counter(f[7]), TX: routeros.Counter(f[8]),
	})
	return equipment.OK
}
