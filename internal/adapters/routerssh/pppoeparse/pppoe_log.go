package pppoeparse

import (
	"bufio"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"regexp"
	"strconv"
	"strings"
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
	events := []string{"logged in",
		"logged out",
		"authentication failed",
		"authentication failure",
		"authenticated",
		"connected",
		"disconnected",
		"terminating",
		"terminated",
		"timed out",
		"timeout",
		"peer is not responding",
		"could not determine remote address",
		"no more free addresses",
		"no free addresses",
		"RADIUS server is not responding",
		"radius server is not responding"}
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
	s := routeros.EscapeScript +
		`; :local pattern ` +
		routeros.Literal(PPLogPattern(login)) +
		`; :do {:local count [/log print count-only where ` +
		filter +
		`]; :local up [/system resource get uptime]; :put ("L\t1\t".[/sys` +

		`tem resource get version]."\t".$up."\t".$count); :local n 0; :lo` +
		`cal bytes 0; :local reason "complete"; :if ($count>2048) do={:se` +
		`t reason "rows"} else={:local ids [/log find where ` + filter + `]; :if ([:len $ids]>2048) do={:set reason "rows"} else={:if ([:l` +
		`en $ids]>0) do={:local begin ([:len $ids]-50); :if ($begin<0) do` +
		`={:set begin 0}; :for pos from=([:len $ids]-1) to=$begin step=-1` +
		` do={:if ($reason="complete") do={:do {:local i [:pick $ids $pos` +
		`]; :local message [/log get $i message]; `
	rules := ppLogEventRules()
	s += `:local body ""; :local recognized false; `
	for i, item := range ppLogContexts(login) {
		variable := "prefix" + strconv.Itoa(i)
		s += `:local ` +
			variable +
			` ` +
			routeros.Literal(item.prefix) +
			`; :if (($recognized=false)&&([:pick $message 0 [:len $` +
			variable +
			`]]=$` +
			variable +
			`)) do={:set body [:pick $message [:len $` +
			variable +
			`] [:len $message]]; :set recognized true}; `
	}
	s += `:if ($recognized=false) do={:set reason "unsupported"}; :local event "other"; `
	for _, rule := range rules {
		parts := []string{}
		for _, phrase := range rule.phrases {
			parts = append(parts, `([:typeof [:find $body `+routeros.Literal(phrase)+`]]!="nil")`)
		}
		s += `:if (` + strings.Join(parts, "||") + `) do={:set event "` + rule.event + `"} else={`
	}
	s += strings.Repeat("}", len(rules)) + `; `
	// No raw message, topic map, log ID, address or username is transmitted.
	emit := routeros.Emit(`"E\t".[$esc [/log get $i time]]."\t".$event`)
	s += strings.ReplaceAll(emit,
		"458752",
		"32768") +
		`} on-error={:set reason "changed"}}}}}}; ` +
		routeros.Footer() +
		`} on-error={:put "U\tlog"}`
	return s, true
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
	s := &ppLogParser{rows: []equipment.PPReadRow{}, counts: counts}
	for scanner.Scan() {
		if code := s.consume(strings.Split(strings.TrimSuffix(scanner.Text(),
			"\r"),
			"\t"),
			""); code != equipment.OK {
			return nil,
				s.version,
				code
		}
	}
	if scanner.Err() != nil || !s.ended {
		return nil, s.version, equipment.Partial
	}
	return s.rows, s.version, equipment.OK
}

type ppLogParser struct {
	rows    []equipment.PPReadRow
	version string
	ended   bool
	up      equipment.PPInterval
	counts  []uint64
}

func (s *ppLogParser) consume(f []string, raw string) equipment.Code {

	if s.ended {
		return equipment.Partial
	}
	if f[0] == "U" {
		return equipment.RadioCodeUnsupported
	}
	if s.version == "" {
		return s.header(f)
	}
	if f[0] == "Z" {
		return s.footer(f)
	}
	if len(f) != 3 || f[0] != "E" || len(s.rows) >= 50 {
		return equipment.Partial
	}
	at, ok := routeros.Unescape(f[1])
	if !ok || len(at) > 256 {
		return equipment.Partial
	}
	event, ok := routeros.Unescape(f[2])
	if !ok || len(event) > 256 {
		return equipment.Partial
	}
	switch event {
	case "auth_failed", "address_failed", "radius_timeout", "timeout", "disconnected", "connected", "other":
	default:
		event = "other"
	}
	s.rows = append(s.rows, equipment.PPReadRow{Kind: "log", At: routeros.LogTime(at), Event: event})
	return equipment.OK
}

func (s *ppLogParser) header(f []string) equipment.Code {

	if s.version == "" {
		if len(f) != 5 || f[0] != "L" || f[1] != "1" || len(f[2]) > 128 || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
			return equipment.Partial
		}
		// ignored: validate escaping while retaining the original version token
		if _, ok := routeros.Unescape(f[2]); !ok {
			return equipment.Partial
		}
		s.version = f[2]
		s.up = ParsePPDuration(f[3])
		count, ok := pdNumber(f[4], 1000000)
		if !ok || len(s.counts) > 0 && s.counts[0] != count {
			return equipment.Partial
		}
		return equipment.OK
	}
	return equipment.OK
}

func (s *ppLogParser) footer(f []string) equipment.Code {

	if f[0] == "Z" {
		if len(f) != 4 {
			return equipment.Partial
		}
		n, e := strconv.Atoi(f[1])
		final := ParsePPDuration(f[3])
		if f[2] == "rows" || f[2] == "wire_bytes" {
			return equipment.Limit
		}
		if f[2] == string(equipment.RadioUnsupported) {
			return equipment.RadioCodeUnsupported
		}
		if e != nil || n != len(s.rows) || f[2] != "complete" || !s.up.Known || !final.Known || final.Max < s.up.Min {
			return equipment.Partial
		}
		s.ended = true
		return equipment.OK
	}
	return equipment.OK
}

func ppLogEventRules() []struct {
	event   string
	phrases []string
} {
	return []struct {
		event   string
		phrases []string
	}{
		{"auth_failed",
			[]string{"authentication failed",
				"authentication failure",
				"failed to authenticate",
				"user not found",
				"invalid username"}},

		{"address_failed", []string{"could not determine remote address", "no more free addresses", "no free addresses"}},
		{"radius_timeout", []string{"RADIUS server is not responding", "radius server is not responding"}},
		{"timeout", []string{"timed out", "timeout", "peer is not responding"}},
		{"disconnected", []string{"disconnected", "terminating", "terminated", "logged out"}},
		{"connected", []string{"connected", "logged in", "authenticated"}},
	}
}
