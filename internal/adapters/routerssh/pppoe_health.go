package routerssh

import (
	"bufio"
	"context"
	"encoding/binary"
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
	type span struct{ lo, hi uint64 }
	spans := []span{}
	number := func(raw string) (uint64, bool) {
		ip, e := netip.ParseAddr(strings.TrimSpace(raw))
		if e != nil || !ip.Is4() {
			return 0, false
		}
		v := ip.As4()
		return uint64(binary.BigEndian.Uint32(v[:])), true
	}
	for _, part := range strings.Split(strings.ReplaceAll(value, ";", ","), ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "/") {
			p, e := netip.ParsePrefix(part)
			if e != nil || !p.Addr().Is4() {
				return nil
			}
			a := p.Masked().Addr().As4()
			lo := uint64(binary.BigEndian.Uint32(a[:]))
			spans = append(spans, span{lo, lo + (uint64(1) << uint(32-p.Bits())) - 1})
			continue
		}
		ends := strings.Split(part, "-")
		if len(ends) > 2 {
			return nil
		}
		lo, ok := number(ends[0])
		if !ok {
			return nil
		}
		hi := lo
		if len(ends) == 2 {
			hi, ok = number(ends[1])
			if !ok || hi < lo {
				return nil
			}
		}
		spans = append(spans, span{lo, hi})
	}
	if len(spans) == 0 {
		return nil
	}
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

func ppCounterString(value string) *string {
	n := ppCounter(value)
	if n == nil {
		return nil
	}
	s := strconv.FormatUint(*n, 10)
	return &s
}
func ParsePPHealth(text string) (*PPHealthSnapshot, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, equipment.Limit
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	snapshot := &PPHealthSnapshot{}
	up := equipment.PPInterval{}
	ended := false
	seen := map[string]bool{}
	var pendingCount *uint64
	count := 0
	for scanner.Scan() {
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		f := strings.Split(raw, "\t")
		if ended {
			return nil, equipment.Partial
		}
		if snapshot.Version == "" {
			if len(f) != 6 || f[0] != "N" || f[1] != "1" || len(f[2]) > 128 || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
				return nil, equipment.Partial
			}
			if _, ok := ppUnescape(f[2]); !ok {
				return nil, equipment.Partial
			}
			snapshot.Version = f[2]
			up = ParsePPDuration(f[3])
			p, ok := pdBool(f[4], false)
			i, ok2 := pdBool(f[5], false)
			if !ok || !ok2 {
				return nil, equipment.Partial
			}
			snapshot.PoolsAvailable = *p
			snapshot.UplinksAvailable = *i
			continue
		}
		if f[0] == "Z" {
			if len(f) != 4 {
				return nil, equipment.Partial
			}
			if f[2] == "rows" || f[2] == "wire_bytes" {
				return nil, equipment.Limit
			}
			n, e := strconv.Atoi(f[1])
			final := ParsePPDuration(f[3])
			if pendingCount != nil || e != nil || n != count || f[2] != "complete" || !up.Known || !final.Known || final.Max < up.Min {
				return nil, equipment.Partial
			}
			ended = true
			continue
		}
		// Some v6 commands echo count-only before its projected pool row. It must
		// match that exact row; arbitrary count lines are never silently discarded.
		if len(f) == 1 && ppDecimal(raw) {
			if pendingCount != nil {
				return nil, equipment.Partial
			}
			pendingCount = ppCounter(raw)
			if pendingCount == nil {
				return nil, equipment.Partial
			}
			continue
		}
		if count >= equipment.PPMaxRows {
			return nil, equipment.Limit
		}
		if len(f) < 3 {
			return nil, equipment.Partial
		}
		for i := 1; i < len(f); i++ {
			v, ok := ppUnescape(f[i])
			if !ok || len(v) > 256 {
				return nil, equipment.Partial
			}
			f[i] = v
		}
		key := f[0] + "/" + f[1]
		if f[1] == "" || seen[key] {
			return nil, equipment.Partial
		}
		seen[key] = true
		switch f[0] {
		case "P":
			if len(f) != 5 || !snapshot.PoolsAvailable {
				return nil, equipment.Partial
			}
			used, size := ppCounter(f[4]), PPPoolSize(f[3])
			if pendingCount != nil && (used == nil || *used != *pendingCount) {
				return nil, equipment.Partial
			}
			pendingCount = nil
			if used != nil && size != nil && *used > *size {
				used = nil
			}
			snapshot.Pools = append(snapshot.Pools, equipment.PPReadRow{Kind: "pool", Name: f[2], Used: used, Size: size})
		case "I":
			if len(f) != 13 || !snapshot.UplinksAvailable || pendingCount != nil {
				return nil, equipment.Partial
			}
			running, ok := pdBool(f[11], true)
			disabled, ok2 := pdBool(f[12], true)
			if !ok {
				running = nil
			}
			if !ok2 {
				disabled = nil
			}
			sample := equipment.PPTunnel{Name: f[2], RX: ppCounter(f[5]), TX: ppCounter(f[6]), RouterUptime: up, Uptime: up}
			// A missing link marker cannot prove continuity even with rising counters.
			if ppHealthLink(f[4]) && f[3] != "" && f[3] != "?" {
				sample.ID = f[1] + "\x00" + f[3] + "\x00" + f[4]
			}
			row := equipment.PPReadRow{Kind: "uplink", Name: f[2], Running: running, Disabled: disabled, RXErrors: ppCounterString(f[7]), TXErrors: ppCounterString(f[8]), RXDrops: ppCounterString(f[9]), TXDrops: ppCounterString(f[10])}
			snapshot.Interfaces = append(snapshot.Interfaces, PPHealthInterface{RemoteID: f[1], Sample: sample, Row: row})
		default:
			return nil, equipment.Partial
		}
		count++
	}
	if scanner.Err() != nil || !ended {
		return nil, equipment.Partial
	}
	return snapshot, equipment.OK
}

// Only named scalar properties are fetched. Pool used records never cross SSH.
func buildPPHealthCommand(pools bool) string {
	poolFind := `:local pools [:toarray ""]; :local poolsOK false; `
	if pools {
		poolFind = `:local pools [:toarray ""]; :local poolsOK true; :do {:set pools [/ip pool find]} on-error={:set poolsOK false}; `
	}
	s := ppEscapeScript + `; ` + routerFlagScript + poolFind + `:local ifs [:toarray ""]; :local ifsOK true; :do {:set ifs [/interface find where dynamic!=yes]} on-error={:set ifsOK false}; :local up [/system resource get uptime]; :put ("N\t1\t".[/system resource get version]."\t".$up."\t".$poolsOK."\t".$ifsOK); :local n 0; :local bytes 0; :local reason "complete"; :if (([:len $pools]+[:len $ifs])>2048) do={:set reason "rows"} else={`
	if pools {
		s += `:foreach i in=$pools do={:if ($reason="complete") do={:do {:local ppHealthName [/ip pool get $i name]; :local ppHealthRanges "?"; :local ppHealthUsed "?"; :do {:set ppHealthRanges [/ip pool get $i ranges]; :if ([:len [:tostr $ppHealthRanges]]>256) do={:set ppHealthRanges "?"}} on-error={}; :do {:set ppHealthUsed [/ip pool get $i used]} on-error={:do {:set ppHealthUsed [/ip pool used print count-only where pool=$ppHealthName]} on-error={}}; ` + pdEmit(`"P\t".[$esc $i]."\t".[$esc $ppHealthName]."\t".[$esc $ppHealthRanges]."\t".[$esc $ppHealthUsed]`) + `} on-error={:set reason "changed"}}}; `
	}
	s += `:foreach i in=$ifs do={:if ($reason="complete") do={:do {:local ppHealthName [/interface get $i name]; `
	fields := []struct{ variable, property string }{{"ppHealthKind", "type"}, {"ppHealthLink", "last-link-up-time"}, {"ppHealthRX", "rx-byte"}, {"ppHealthTX", "tx-byte"}, {"ppHealthRXE", "rx-error"}, {"ppHealthTXE", "tx-error"}, {"ppHealthRXD", "rx-drop"}, {"ppHealthTXD", "tx-drop"}, {"ppHealthRunning", "running"}, {"ppHealthDisabled", "disabled"}}
	expr := `"I\t".[$esc $i]."\t".[$esc $ppHealthName]`
	for _, f := range fields {
		getter := `[/interface get $i ` + f.property + `]`
		if f.property == "running" || f.property == "disabled" {
			getter = `[$ppflag ` + getter + `]`
		}
		s += `:local ` + f.variable + ` "?"; :do {:set ` + f.variable + ` ` + getter + `} on-error={}; `
		expr += `."\t".[$esc $` + f.variable + `]`
	}
	s += `:local afterLink "?"; :local afterName "?"; :local afterKind "?"; :do {:set afterLink [/interface get $i last-link-up-time]; :set afterName [/interface get $i name]; :set afterKind [/interface get $i type]} on-error={}; :if (($afterLink!=$ppHealthLink)||($afterName!=$ppHealthName)||($afterKind!=$ppHealthKind)) do={:set ppHealthRX "?"; :set ppHealthTX "?"; :set ppHealthLink "?"}; ` + pdEmit(expr) + `} on-error={:set reason "changed"}}}}; ` + pdFooter()
	return s
}

var PPHealthFirstCommand = buildPPHealthCommand(false)
var PPHealthCommand = buildPPHealthCommand(true)

func readPPHealth(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code) (*equipment.PPReadResult, equipment.Code) {
	start := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	sample := func(command string) (*PPHealthSnapshot, equipment.Code) {
		begin := time.Since(start).Nanoseconds()
		text, c := connection.run(command)
		if c != equipment.OK {
			return nil, c
		}
		snapshot, c := ParsePPHealth(text)
		if c != equipment.OK {
			return nil, c
		}
		window := equipment.PPInterval{Known: true, Min: begin, Max: time.Since(start).Nanoseconds()}
		for i := range snapshot.Interfaces {
			snapshot.Interfaces[i].Sample.Sample = window
		}
		return snapshot, equipment.OK
	}
	first, c := sample(PPHealthFirstCommand)
	if c != equipment.OK {
		return nil, c
	}
	timer := time.NewTimer(time.Second)
	select {
	case <-ctx.Done():
		timer.Stop()
		return nil, equipment.Cancelled
	case <-timer.C:
	}
	second, c := sample(PPHealthCommand)
	if c != equipment.OK {
		return nil, c
	}
	if first.Version != second.Version {
		return nil, equipment.ObservationIncompatible
	}
	previous := map[string]equipment.PPTunnel{}
	for _, row := range first.Interfaces {
		previous[row.RemoteID] = row.Sample
	}
	rows := append([]equipment.PPReadRow(nil), second.Pools...)
	for _, row := range second.Interfaces {
		row.Row.TX, row.Row.RX = equipment.PPBandwidth(previous[row.RemoteID], row.Sample)
		rows = append(rows, row.Row)
	}
	commands, bytes := connection.metrics()
	return ppMeasureRead(&equipment.PPReadResult{Scope: "health", Version: second.Version, Started: equipment.PPTime(start), Ended: equipment.PPTime(time.Now()), Duration: time.Since(start).Nanoseconds(), Rows: rows, PoolsAvailable: &second.PoolsAvailable, UplinksAvailable: &second.UplinksAvailable, PoolTotal: len(second.Pools), UplinkTotal: len(second.Interfaces), Cost: equipment.PPReadCost{Commands: commands, Bytes: bytes, FirstCount: len(first.Interfaces), LastCount: len(second.Interfaces)}})
}

func ppHealthLink(value string) bool {
	for _, layout := range []string{"Jan/02/2006 15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if _, e := time.Parse(layout, value); e == nil {
			return true
		}
	}
	return false
}
