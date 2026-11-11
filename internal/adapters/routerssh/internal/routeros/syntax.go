package routeros

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Only complete bounded diagnostic lines are recognized. Projected names/logins
// containing these words, and arbitrary stderr, are never copied into metadata.
var collectorSyntaxLine = regexp.MustCompile(`(?i)^(?:(?:syntax error|expected end of command)(?: \(line [0-9]` +
	`+ column [0-9]+\))?|bad command name(?: [A-Za-z0-9_-]+)?(?: \(li` +
	`ne [0-9]+ column [0-9]+\))?)$`)

func CollectorSyntaxDiagnostic(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if len(line) <= 256 && collectorSyntaxLine.MatchString(line) {
			return true
		}
	}
	return false
}

const EscapeScript = `:local esc do={:local v [:tostr $1]; :local out ""; :if ([:len $` +
	`v]=0) do={:return ""}; :if ([:len $v] > 512) do={:error "field-l` +
	`imit"}; :for n from=0 to=([:len $v]-1) do={:local c [:pick $v $n` +
	` ($n+1)]; :if ($c="\\") do={:set c "\\\\"} else={:if ($c="\t") d` +
	`o={:set c "\\t"} else={:if ($c="\r") do={:set c "\\r"} else={:if` +
	` ($c="\n") do={:set c "\\n"}}}}; :set out ($out.$c)}; :return $o` +
	`ut}`

func Unescape(s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			if s[i] < 32 || s[i] == 127 {
				return "", false
			}
			b.WriteByte(s[i])
			continue
		}
		i++
		if i >= len(s) {
			return "", false
		}
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'n':
			b.WriteByte('\n')
		default:
			return "", false
		}
	}
	v := b.String()
	return v, utf8.ValidString(v)
}

func Emit(expression string) string {
	return `:local line (` + expression + `); :if (($bytes+[:len $line]+2)>458752) do={:set reason "wire_by` +
		`tes"}; :if ($reason="complete") do={:put $line; :set bytes ($byt` +
		`es+[:len $line]+2); :set n ($n+1)}; `
}

func Footer() string {
	return `:if ([/system resource get uptime]<$up) do={:set reason "churn"}` +
		`; :put ("Z\t".$n."\t".$reason."\t".[/system resource get uptime]` +
		`); `
}

func Bool(value string, optional bool) (*bool, bool) {
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

func Decimal(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func Literal(value string) string {
	r := strings.NewReplacer(
		`\`, `\\`, `"`, `\"`, `$`, `\$`, "\n", `\n`, "\r", `\r`, "\t", `\t`,
	)
	return `"` + r.Replace(value) + `"`
}

func Counter(v string) *uint64 {
	n, e := strconv.ParseUint(v, 10, 64)
	if e != nil || !Decimal(v) {
		return nil
	}
	return &n
}

// In fixed RouterOS flag slots an unmarked flag is nil, not an explicit false.
// This is only for documented flags (running/dynamic/invalid), never optional
// configuration. Unexpected values fail the read instead of becoming false.
const FlagScript = `:local ppflag do={:local flagValue $1; :if ([:typeof $flagValue]` +
	`="nil") do={:return "false"}; :local flagText [:tostr $flagValue` +
	`]; :if (($flagText="true")||($flagText="yes")) do={:return "true` +
	`"}; :if (($flagText="false")||($flagText="no")) do={:return "fal` +
	`se"}; :error "flag-type"}; `
