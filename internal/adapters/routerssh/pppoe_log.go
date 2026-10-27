package routerssh

import (
	"bufio"
	"context"
	"infradesktop/internal/modules/equipment"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Login is an exact token or exact generated tunnel marker, never a regex from
// the UI. RouterOS receives a quoted literal containing the escaped pattern.
type ppLogContext struct{ prefix, bodyPattern string }

func ppLogContexts(login string) []ppLogContext {
	contexts := []ppLogContext{}
	if !strings.ContainsAny(login, "<>") {
		contexts = append(contexts, ppLogContext{"<pppoe-" + login + ">: ", `[^<>]*`})
	}
	if !strings.Contains(login, "'") {
		contexts = append(contexts, ppLogContext{"user '" + login + "' ", `[^'<>]*`})
	}
	if !strings.Contains(login, `"`) {
		contexts = append(contexts, ppLogContext{`user "` + login + `" `, `[^"<>]*`})
	}
	events := []string{"logged in", "logged out", "authentication failed", "authentication failure", "authenticated", "connected", "disconnected", "terminating", "terminated", "timed out", "timeout", "peer is not responding", "could not determine remote address", "no more free addresses", "no free addresses", "RADIUS server is not responding", "radius server is not responding"}
	for i := range events {
		events[i] = regexp.QuoteMeta(events[i])
	}
	// Unquoted identities require a complete recognized suffix, not an internal
	// delimiter. Address/caller tokens cannot contain another event or username.
	events = append(events, `logged in, [0-9.]+ from [0-9A-Fa-f:]+`)
	contexts = append(contexts, ppLogContext{login + " ", "(" + strings.Join(events, "|") + ")"})
	return contexts
}
func PPLogPattern(login string) string {
	patterns := []string{}
	for _, item := range ppLogContexts(login) {
		patterns = append(patterns, regexp.QuoteMeta(item.prefix)+item.bodyPattern)
	}
	return "^(" + strings.Join(patterns, "|") + ")$"
}
func BuildPPLogCommand(login string) (string, bool) {
	if login == "" || len(login) > 256 || !utf8.ValidString(login) {
		return "", false
	}
	filter := `topics~"ppp" and !(topics~"debug") and message~$pattern`
	s := ppEscapeScript + `; :local pattern ` + ppLiteral(PPLogPattern(login)) + `; :do {:local count [/log print count-only where ` + filter + `]; :local up [/system resource get uptime]; :put ("L\t1\t".[/system resource get version]."\t".$up."\t".$count); :local n 0; :local bytes 0; :local reason "complete"; :if ($count>2048) do={:set reason "rows"} else={:local ids [/log find where ` + filter + `]; :if ([:len $ids]>2048) do={:set reason "rows"} else={:if ([:len $ids]>0) do={:local begin ([:len $ids]-50); :if ($begin<0) do={:set begin 0}; :for pos from=([:len $ids]-1) to=$begin step=-1 do={:if ($reason="complete") do={:do {:local i [:pick $ids $pos]; :local message [/log get $i message]; `
	rules := []struct {
		event   string
		phrases []string
	}{
		{"auth_failed", []string{"authentication failed", "authentication failure", "failed to authenticate", "user not found", "invalid username"}},
		{"address_failed", []string{"could not determine remote address", "no more free addresses", "no free addresses"}},
		{"radius_timeout", []string{"RADIUS server is not responding", "radius server is not responding"}},
		{"timeout", []string{"timed out", "timeout", "peer is not responding"}},
		{"disconnected", []string{"disconnected", "terminating", "terminated", "logged out"}},
		{"connected", []string{"connected", "logged in", "authenticated"}},
	}
	s += `:local body ""; :local recognized false; `
	for i, item := range ppLogContexts(login) {
		variable := "prefix" + strconv.Itoa(i)
		s += `:local ` + variable + ` ` + ppLiteral(item.prefix) + `; :if (($recognized=false)&&([:pick $message 0 [:len $` + variable + `]]=$` + variable + `)) do={:set body [:pick $message [:len $` + variable + `] [:len $message]]; :set recognized true}; `
	}
	s += `:if ($recognized=false) do={:set reason "unsupported"}; :local event "other"; `
	for _, rule := range rules {
		parts := []string{}
		for _, phrase := range rule.phrases {
			parts = append(parts, `([:typeof [:find $body `+ppLiteral(phrase)+`]]!="nil")`)
		}
		s += `:if (` + strings.Join(parts, "||") + `) do={:set event "` + rule.event + `"} else={`
	}
	s += strings.Repeat("}", len(rules)) + `; `
	// No raw message, topic map, log ID, address or username is transmitted.
	emit := pdEmit(`"E\t".[$esc [/log get $i time]]."\t".$event`)
	s += strings.ReplaceAll(emit, "458752", "32768") + `} on-error={:set reason "changed"}}}}}}; ` + pdFooter() + `} on-error={:put "U\tlog"}`
	return s, true
}
func ppLogTime(value string) string {
	if len(value) > 64 {
		return ""
	}
	for _, layout := range []string{"15:04:05", "Jan/02/2006 15:04:05", "Jan/02 15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if _, e := time.Parse(layout, value); e == nil {
			return value
		}
	}
	return ""
}
func ParsePPLog(text string) ([]equipment.PPReadRow, string, equipment.Code) {
	if len(text) > 32*1024 {
		return nil, "", equipment.Limit
	}
	text, counts, ok := ppCountPrelude(text, 1)
	if !ok {
		return nil, "", equipment.Partial
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	rows := []equipment.PPReadRow{}
	version := ""
	ended := false
	up := equipment.PPInterval{}
	for scanner.Scan() {
		f := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		if ended {
			return nil, version, equipment.Partial
		}
		if f[0] == "U" {
			return nil, version, "UNSUPPORTED"
		}
		if version == "" {
			if len(f) != 5 || f[0] != "L" || f[1] != "1" || len(f[2]) > 128 || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
				return nil, version, equipment.Partial
			}
			if _, ok := ppUnescape(f[2]); !ok {
				return nil, version, equipment.Partial
			}
			version = f[2]
			up = ParsePPDuration(f[3])
			count, ok := pdNumber(f[4], 1000000)
			if !ok || len(counts) > 0 && counts[0] != count {
				return nil, version, equipment.Partial
			}
			continue
		}
		if f[0] == "Z" {
			if len(f) != 4 {
				return nil, version, equipment.Partial
			}
			n, e := strconv.Atoi(f[1])
			final := ParsePPDuration(f[3])
			if f[2] == "rows" || f[2] == "wire_bytes" {
				return nil, version, equipment.Limit
			}
			if f[2] == "unsupported" {
				return nil, version, "UNSUPPORTED"
			}
			if e != nil || n != len(rows) || f[2] != "complete" || !up.Known || !final.Known || final.Max < up.Min {
				return nil, version, equipment.Partial
			}
			ended = true
			continue
		}
		if len(f) != 3 || f[0] != "E" || len(rows) >= 50 {
			return nil, version, equipment.Partial
		}
		at, ok := ppUnescape(f[1])
		if !ok || len(at) > 256 {
			return nil, version, equipment.Partial
		}
		event, ok := ppUnescape(f[2])
		if !ok || len(event) > 256 {
			return nil, version, equipment.Partial
		}
		switch event {
		case "auth_failed", "address_failed", "radius_timeout", "timeout", "disconnected", "connected", "other":
		default:
			event = "other"
		}
		rows = append(rows, equipment.PPReadRow{Kind: "log", At: ppLogTime(at), Event: event})
	}
	if scanner.Err() != nil || !ended {
		return nil, version, equipment.Partial
	}
	return rows, version, equipment.OK
}
func readPPLog(ctx context.Context, d equipment.Device, user, secret string, target *equipment.PPReadTarget, check func() equipment.Code) (*equipment.PPReadResult, equipment.Code) {
	if target == nil || len(target.Rows) != 1 {
		return nil, equipment.Invalid
	}
	command, ok := BuildPPLogCommand(target.Rows[0].Login)
	if !ok {
		return nil, equipment.Invalid
	}
	start := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	connection.tighten(32 * 1024)
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	rows, version, c := ParsePPLog(text)
	if c != equipment.OK {
		return nil, c
	}
	if version != target.Observation.Provenance.Version {
		return nil, equipment.ObservationIncompatible
	}
	commands, bytes := connection.metrics()
	return ppMeasureRead(&equipment.PPReadResult{Scope: "log", Version: version, Started: equipment.PPTime(start), Ended: equipment.PPTime(time.Now()), Duration: time.Since(start).Nanoseconds(), Rows: rows, Cost: equipment.PPReadCost{Commands: commands, Bytes: bytes}})
}
