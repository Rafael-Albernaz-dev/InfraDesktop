package pppoeparse

import (
	"bufio"
	"encoding/binary"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Pool configuration ranges are consumed here and never enter the result DTO.
func PPPoolSize(value string) *uint64 {
	if value == "" || value == "?" || len(value) > 4096 {
		return nil
	}
	spans := []ppPoolSpan{}

	for _, part := range strings.Split(strings.ReplaceAll(value, ";", ","), ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "/") {
			p, e := netip.ParsePrefix(part)
			if e != nil || !p.Addr().Is4() {
				return nil
			}
			a := p.Masked().Addr().As4()
			lo := uint64(binary.BigEndian.Uint32(a[:]))
			spans = append(spans, ppPoolSpan{lo, lo + (uint64(1) << uint(32-p.Bits())) - 1})
			continue
		}
		ends := strings.Split(part, "-")
		if len(ends) > 2 {
			return nil
		}
		lo, ok := ppPoolIPv4Number(ends[0])
		if !ok {
			return nil
		}
		hi := lo
		if len(ends) == 2 {
			hi, ok = ppPoolIPv4Number(ends[1])
			if !ok || hi < lo {
				return nil
			}
		}
		spans = append(spans, ppPoolSpan{lo, hi})
	}
	if len(spans) == 0 {
		return nil
	}
	return ppPoolCardinality(spans)
}

type ppPoolSpan struct{ lo, hi uint64 }

func ppPoolCardinality(spans []ppPoolSpan) *uint64 {
	sort.Slice(spans, func(i, j int) bool { return spans[i].lo < spans[j].lo })
	current := spans[0]
	total := uint64(0)
	for _, next := range spans[1:] {
		if next.lo <= current.hi+1 {
			if next.hi > current.hi {
				current.hi = next.hi
			}
		} else {
			total += current.hi - current.lo + 1
			current = next
		}
	}
	total += current.hi - current.lo + 1
	return &total
}

type PPHealthInterface struct {
	RemoteID string
	Sample   equipment.PPTunnel
	Row      equipment.PPReadRow
}
type PPHealthSnapshot struct {
	Version                          string
	PoolsAvailable, UplinksAvailable bool
	Pools                            []equipment.PPReadRow
	Interfaces                       []PPHealthInterface
}

func ParsePPHealth(text string) (*PPHealthSnapshot, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	s := &ppHealthParser{snapshot: &PPHealthSnapshot{}, seen: map[string]bool{}}
	for scanner.Scan() {
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		if code := s.consume(strings.Split(raw, "\t"), raw); code != equipment.OK {
			return nil, code
		}
	}
	if scanner.Err() != nil || !s.ended {
		return nil, equipment.Partial
	}
	return s.snapshot, equipment.OK
}

type ppHealthParser struct {
	snapshot     *PPHealthSnapshot
	up           equipment.PPInterval
	ended        bool
	seen         map[string]bool
	pendingCount *uint64
	count        int
}

func (s *ppHealthParser) consume(f []string, raw string) equipment.Code {

	if s.ended {
		return equipment.Partial
	}
	if s.snapshot.Version == "" {
		return s.header(f)
	}
	if f[0] == "Z" {
		return s.footer(f)
	}
	// Some v6 commands echo count-only before its projected pool row. It must
	// match that exact row; arbitrary count lines are never silently discarded.
	if len(f) == 1 && routeros.Decimal(raw) {
		if s.pendingCount != nil {
			return equipment.Partial
		}
		s.pendingCount = routeros.Counter(raw)
		if s.pendingCount == nil {
			return equipment.Partial
		}
		return equipment.OK
	}
	if code := s.row(f); code != equipment.OK {
		return code
	}
	s.count++
	return equipment.OK
}

func (s *ppHealthParser) header(f []string) equipment.Code {

	if s.snapshot.Version == "" {
		if len(f) != 6 || f[0] != "N" || f[1] != "1" || len(f[2]) > 128 || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
			return equipment.Partial
		}
		// ignored: validate escaping while retaining the original version token
		if _, ok := routeros.Unescape(f[2]); !ok {
			return equipment.Partial
		}
		s.snapshot.Version = f[2]
		s.up = ParsePPDuration(f[3])
		p, ok := routeros.Bool(f[4], false)
		i, ok2 := routeros.Bool(f[5], false)
		if !ok || !ok2 {
			return equipment.Partial
		}
		s.snapshot.PoolsAvailable = *p
		s.snapshot.UplinksAvailable = *i
		return equipment.OK
	}
	return equipment.OK
}

func (s *ppHealthParser) footer(f []string) equipment.Code {

	if f[0] == "Z" {
		if len(f) != 4 {
			return equipment.Partial
		}
		if f[2] == "rows" || f[2] == "wire_bytes" {
			return equipment.Limit
		}
		n, e := strconv.Atoi(f[1])
		final := ParsePPDuration(f[3])
		if s.pendingCount != nil || e != nil || n != s.count || f[2] != "complete" || !s.up.Known || !final.Known || final.Max < s.up.Min {
			return equipment.Partial
		}
		s.ended = true
		return equipment.OK
	}
	return equipment.OK
}

func (s *ppHealthParser) row(f []string) equipment.Code {

	if s.count >= equipment.PPMaxRows {
		return equipment.Limit
	}
	if len(f) < 3 {
		return equipment.Partial
	}
	for i := 1; i < len(f); i++ {
		v, ok := routeros.Unescape(f[i])
		if !ok || len(v) > 256 {
			return equipment.Partial
		}
		f[i] = v
	}
	key := f[0] + "/" + f[1]
	if f[1] == "" || s.seen[key] {
		return equipment.Partial
	}
	s.seen[key] = true
	switch f[0] {
	case "P":
		return s.pool(f)
	case "I":
		return s.uplink(f)
	default:
		return equipment.Partial
	}
}

// Only named scalar properties are fetched. Pool used records never cross SSH.
func buildPPHealthCommand(pools bool) string {
	poolFind := `:local pools [:toarray ""]; :local poolsOK false; `
	if pools {
		poolFind = `:local pools [:toarray ""]; :local poolsOK true; :do {:set pools [/ip pool find]} on-error={:set poolsOK false}; `
	}
	s := routeros.EscapeScript + `; ` + routeros.FlagScript + poolFind + `:local ifs [:toarray ""]; :local ifsOK true; :do {:set ifs [/int` +
		`erface find where dynamic!=yes]} on-error={:set ifsOK false}; :l` +
		`ocal up [/system resource get uptime]; :put ("N\t1\t".[/system r` +
		`esource get version]."\t".$up."\t".$poolsOK."\t".$ifsOK); :local` +
		` n 0; :local bytes 0; :local reason "complete"; :if (([:len $poo` +
		`ls]+[:len $ifs])>2048) do={:set reason "rows"} else={`
	if pools {
		s += `:foreach i in=$pools do={:if ($reason="complete") do={:do {:loca` +
			`l ppHealthName [/ip pool get $i name]; :local ppHealthRanges "?"` +
			`; :local ppHealthUsed "?"; :do {:set ppHealthRanges [/ip pool ge` +
			`t $i ranges]; :if ([:len [:tostr $ppHealthRanges]]>256) do={:set` +
			` ppHealthRanges "?"}} on-error={}; :do {:set ppHealthUsed [/ip p` +
			`ool get $i used]} on-error={:do {:set ppHealthUsed [/ip pool use` +
			`d print count-only where pool=$ppHealthName]} on-error={}}; ` +
			routeros.Emit(`"P\t".[$esc $i]."\t".[$esc $ppHealthName]."\t".[$esc $ppHealthRa`+

				`nges]."\t".[$esc $ppHealthUsed]`) + `} on-error={:set reason "changed"}}}; `
	}
	s += `:foreach i in=$ifs do={:if ($reason="complete") do={:do {:local ppHealthName [/interface get $i name]; `
	fields := []struct{ variable, property string }{
		{"ppHealthKind", "type"}, {"ppHealthLink", "last-link-up-time"},
		{"ppHealthRX", "rx-byte"}, {"ppHealthTX", "tx-byte"},
		{"ppHealthRXE", "rx-error"}, {"ppHealthTXE", "tx-error"},
		{"ppHealthRXD", "rx-drop"}, {"ppHealthTXD", "tx-drop"},
		{"ppHealthRunning", "running"}, {"ppHealthDisabled", "disabled"},
	}
	expr := `"I\t".[$esc $i]."\t".[$esc $ppHealthName]`
	for _, f := range fields {
		getter := `[/interface get $i ` + f.property + `]`
		if f.property == "running" || f.property == "disabled" {
			getter = `[$ppflag ` + getter + `]`
		}
		s += `:local ` + f.variable + ` "?"; :do {:set ` + f.variable + ` ` + getter + `} on-error={}; `
		expr += `."\t".[$esc $` + f.variable + `]`
	}
	s += `:local afterLink "?"; :local afterName "?"; :local afterKind "?"` +
		`; :do {:set afterLink [/interface get $i last-link-up-time]; :se` +
		`t afterName [/interface get $i name]; :set afterKind [/interface` +
		` get $i type]} on-error={}; :if (($afterLink!=$ppHealthLink)||($` +
		`afterName!=$ppHealthName)||($afterKind!=$ppHealthKind)) do={:set` +
		` ppHealthRX "?"; :set ppHealthTX "?"; :set ppHealthLink "?"}; ` +
		routeros.Emit(expr) +
		`} on-error={:set reason "changed"}}}}; ` +
		routeros.Footer()
	return s
}

var PPHealthFirstCommand = buildPPHealthCommand(false)
var PPHealthCommand = buildPPHealthCommand(true)

func ppHealthLink(value string) bool {
	for _, layout := range []string{"Jan/02/2006 15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		// ignored: only timestamp validity matters for the link continuity marker
		if _, e := time.Parse(layout, value); e == nil {
			return true
		}
	}
	return false
}

func (s *ppHealthParser) pool(f []string) equipment.Code {

	if len(f) != 5 || !s.snapshot.PoolsAvailable {
		return equipment.Partial
	}
	used, size := routeros.Counter(f[4]), PPPoolSize(f[3])
	if s.pendingCount != nil && (used == nil || *used != *s.pendingCount) {
		return equipment.Partial
	}
	s.pendingCount = nil
	if used != nil && size != nil && *used > *size {
		used = nil
	}
	s.snapshot.Pools = append(s.snapshot.Pools, equipment.PPReadRow{Kind: "pool", Name: f[2], Used: used, Size: size})

	return equipment.OK
}

func (s *ppHealthParser) uplink(f []string) equipment.Code {

	if len(f) != 13 || !s.snapshot.UplinksAvailable || s.pendingCount != nil {
		return equipment.Partial
	}
	running, ok := routeros.Bool(f[11], true)
	disabled, ok2 := routeros.Bool(f[12], true)
	if !ok {
		running = nil
	}
	if !ok2 {
		disabled = nil
	}
	sample := equipment.PPTunnel{Name: f[2], RX: routeros.Counter(f[5]), TX: routeros.Counter(f[6]), RouterUptime: s.up, Uptime: s.up}
	// A missing link marker cannot prove continuity even with rising counters.
	if ppHealthLink(f[4]) && f[3] != "" && f[3] != "?" {
		sample.ID = f[1] + "\x00" + f[3] + "\x00" + f[4]
	}
	row := equipment.PPReadRow{Kind: "uplink",
		Name:     f[2],
		Running:  running,
		Disabled: disabled,
		RXErrors: routeros.CounterString(f[7]),
		TXErrors: routeros.CounterString(f[8]),
		RXDrops:  routeros.CounterString(f[9]),
		TXDrops:  routeros.CounterString(f[10])}
	s.snapshot.Interfaces = append(s.snapshot.Interfaces, PPHealthInterface{RemoteID: f[1], Sample: sample, Row: row})

	return equipment.OK
}

func ppPoolIPv4Number(raw string) (uint64, bool) {
	ip, e := netip.ParseAddr(strings.TrimSpace(raw))
	if e != nil || !ip.Is4() {
		return 0, false
	}
	v := ip.As4()
	return uint64(binary.BigEndian.Uint32(v[:])), true

}
