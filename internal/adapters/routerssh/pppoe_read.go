package routerssh

import (
	"bufio"
	"context"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
	"time"
)

func ppLiteral(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, `$`, `\$`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(value) + `"`
}

// Selection is generated from rows loaded by the service, never text from UI.
// A name lookup happens only after the monitor supplies the session tuple.
func BuildPPBandCommand(rows []equipment.PPRow) (string, bool) {
	if len(rows) > equipment.PageSize {
		return "", false
	}
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
			selection += "user=" + ppLiteral(row.Login)
			seen[row.Login] = true
		}
	}
	find := `[/interface pppoe-server find where running=yes]`
	if selection != "" {
		find = `[/interface pppoe-server find where running=yes and (` + selection + `)]`
	}
	s := ppEscapeScript + `; :local up [/system resource get uptime]; :local cpu "?"; :do {:set cpu [/system resource get cpu-load]} on-error={}; :put ("B\t1\t".[/system resource get version]."\t".$up."\t".$cpu); :local ids ` + find + `; :local n 0; :local bytes 0; :local reason "complete"; :if ([:len $ids]>2048) do={:set reason "rows"} else={:foreach i in=$ids do={:if ($reason="complete") do={:do {`
	s += `:local m [/interface pppoe-server monitor $i once as-value]; :local ppBandName [/interface pppoe-server get $i name]; :local rx "?"; :local tx "?"; :local ifs [/interface find where name=$ppBandName and type="pppoe-in"]; :if ([:len $ifs]=1) do={:local p [:pick $ifs 0]; :do {:set rx [/interface get $p rx-byte]; :set tx [/interface get $p tx-byte]} on-error={:set rx "?"; :set tx "?"}}; :local after [/interface pppoe-server monitor $i once as-value]; :if ((($after->"user")!=($m->"user"))||(($after->"caller-id")!=($m->"caller-id"))||(($after->"remote-address")!=($m->"remote-address"))||(($after->"uptime")<($m->"uptime"))) do={:set rx "?"; :set tx "?"}; `
	fields := []string{`$i`, `$ppBandName`, `($m->"user")`, `($m->"caller-id")`, `($m->"remote-address")`, `($m->"uptime")`, `$rx`, `$tx`}
	expr := `"T"`
	for _, field := range fields {
		expr += `."\t".[$esc ` + field + `]`
	}
	s += pdEmit(expr) + `} on-error={}}}}; ` + pdFooter()
	return s, true
}
func ppCounter(v string) *uint64 {
	n, e := strconv.ParseUint(v, 10, 64)
	if e != nil || !ppDecimal(v) {
		return nil
	}
	return &n
}
func ParsePPBand(text string) ([]equipment.PPTunnel, string, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, "", equipment.Limit
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	version := ""
	up := equipment.PPInterval{}
	var cpu *uint64
	out := []equipment.PPTunnel{}
	ended := false
	seen := map[string]bool{}
	for scanner.Scan() {
		f := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		if ended {
			return nil, version, equipment.Partial
		}
		if version == "" {
			if (len(f) != 4 && len(f) != 5) || f[0] != "B" || f[1] != "1" || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
				return nil, version, equipment.Partial
			}
			version = f[2]
			if len(version) > 128 {
				return nil, version, equipment.Partial
			}
			if _, ok := ppUnescape(version); !ok {
				return nil, version, equipment.Partial
			}
			up = ParsePPDuration(f[3])
			if len(f) == 5 {
				cpu = ppCounter(f[4])
				if cpu != nil && *cpu > 100 {
					cpu = nil
				}
			}
			continue
		}
		if f[0] == "Z" {
			if len(f) != 4 {
				return nil, version, equipment.Partial
			}
			n, e := strconv.Atoi(f[1])
			final := ParsePPDuration(f[3])
			if e != nil || n != len(out) || f[2] != "complete" || !up.Known || !final.Known || final.Max < up.Min {
				return nil, version, equipment.Partial
			}
			ended = true
			continue
		}
		if len(f) != 9 || f[0] != "T" || len(out) >= equipment.PPMaxRows {
			return nil, version, equipment.Partial
		}
		for i := 1; i < len(f); i++ {
			v, ok := ppUnescape(f[i])
			if !ok || len(v) > 256 {
				return nil, version, equipment.Partial
			}
			f[i] = v
		}
		if f[1] == "" || seen[f[1]] {
			return nil, version, equipment.Partial
		}
		seen[f[1]] = true
		out = append(out, equipment.PPTunnel{ID: f[1], Name: f[2], Login: f[3], Caller: f[4], Address: f[5], Uptime: ParsePPDuration(f[6]), RouterUptime: up, RouterCPU: cpu, RX: ppCounter(f[7]), TX: ppCounter(f[8])})
	}
	if scanner.Err() != nil || !ended {
		return nil, version, equipment.Partial
	}
	return out, version, equipment.OK
}

func (Adapter) ReadPP(ctx context.Context, d equipment.Device, user, secret, scope string, target *equipment.PPReadTarget, publish func(*equipment.PPReadResult) equipment.Code, check func() equipment.Code) (*equipment.PPReadResult, equipment.Code) {
	if scope == "health" {
		return readPPHealth(ctx, d, user, secret, check)
	}
	if scope == "log" {
		return readPPLog(ctx, d, user, secret, target, check)
	}
	if scope == "secret" {
		return readPPSecret(ctx, d, user, secret, target, check)
	}
	if (scope != "bandwidth" && scope != "live" && scope != "top") || target == nil {
		return nil, equipment.Invalid
	}
	origin := time.Now()
	started := equipment.PPTime(origin)
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.PPReadBudget(scope))
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	command, ok := BuildPPBandCommand(target.Rows)
	if !ok {
		return nil, equipment.Invalid
	}
	expectedVersion := target.Observation.Provenance.Version
	sample := func() ([]equipment.PPTunnel, equipment.Code) {
		begin := time.Since(origin).Nanoseconds()
		ageStart := time.Since(target.Prepared).Nanoseconds()
		text, c := connection.run(command)
		if c != equipment.OK {
			return nil, c
		}
		rows, version, c := ParsePPBand(text)
		if c != equipment.OK {
			return nil, c
		}
		if expectedVersion == "" && scope == "top" {
			expectedVersion = version
		}
		if version != expectedVersion {
			return nil, equipment.ObservationIncompatible
		}
		window := equipment.PPInterval{Known: true, Min: begin, Max: time.Since(origin).Nanoseconds()}
		age := equipment.PPInterval{Known: true, Min: target.Age.Min + ageStart, Max: target.Age.Max + time.Since(target.Prepared).Nanoseconds()}
		for i := range rows {
			rows[i].Sample = window
		}
		if scope == "top" {
			return rows, equipment.OK
		}
		selected := equipment.MatchPPRead(target, rows, age)
		return selected, equipment.OK
	}
	first, c := sample()
	if c != equipment.OK {
		return nil, c
	}
	var result *equipment.PPReadResult
	interval := time.Second
	if scope == "live" {
		interval = 2 * time.Second
	}
	for {
		if scope == "live" {
			if deadline, ok := ctx.Deadline(); !ok {
				return nil, equipment.Invalid
			} else if time.Until(deadline) <= interval+time.Second {
				if result == nil {
					return nil, equipment.Partial
				}
				return result, equipment.OK
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, equipment.Cancelled
		case <-timer.C:
		}
		second, c := sample()
		if c != equipment.OK {
			return nil, c
		}
		commands, bytes := connection.metrics()
		result = &equipment.PPReadResult{Version: expectedVersion, Scope: scope, Started: started, Ended: equipment.PPTime(time.Now()), Duration: time.Since(origin).Nanoseconds(), Cost: equipment.PPReadCost{Commands: commands, Bytes: bytes, FirstCount: len(first), LastCount: len(second)}}
		if len(first) > 0 {
			result.Cost.CPUStart = first[0].RouterCPU
		}
		if len(second) > 0 {
			result.Cost.CPUEnd = second[0].RouterCPU
		}
		if scope == "top" {
			previous := map[string]equipment.PPTunnel{}
			for _, row := range first {
				previous[row.ID] = row
			}
			for _, row := range second {
				download, upload := equipment.PPBandwidth(previous[row.ID], row)
				result.Rows = append(result.Rows, equipment.PPReadRow{Kind: "session", Name: row.Name, Login: row.Login, Download: download, Upload: upload, Uptime: row.Uptime})
			}
			equipment.SortPPConsumption(result.Rows)
		} else {
			for i, row := range target.Rows {
				download, upload := equipment.PPBandwidth(first[i], second[i])
				result.Rows = append(result.Rows, equipment.PPReadRow{Kind: "session", RowID: row.ID, Login: row.Login, Download: download, Upload: upload, Uptime: second[i].Uptime})
			}
		}
		result, c = ppMeasureRead(result)
		if c != equipment.OK {
			return nil, c
		}
		if scope != "live" {
			return result, equipment.OK
		}
		if c := publish(result); c != equipment.OK {
			return nil, c
		}
		first = second // Only the most recent pair is retained.
	}
}

// The protocol byte budget belongs at the adapter. Internal measurements are
// carried to the service so it can enforce its neutral capability limits.
func ppMeasureRead(result *equipment.PPReadResult) (*equipment.PPReadResult, equipment.Code) {
	if result == nil {
		return nil, equipment.Partial
	}
	for _, row := range result.Rows {
		body, e := json.Marshal(row)
		if e != nil {
			return nil, equipment.Partial
		}
		result.MaxRowBytes = max(result.MaxRowBytes, len(body))
	}
	metadata := *result
	metadata.Rows = nil
	body, e := json.Marshal(metadata)
	if e != nil {
		return nil, equipment.Partial
	}
	result.MetadataBytes = len(body)
	body, e = json.Marshal(result)
	if e != nil {
		return nil, equipment.Partial
	}
	result.PayloadBytes = len(body)
	if !equipment.PPReadWithinBudget(result) {
		return nil, equipment.Limit
	}
	return result, equipment.OK
}
