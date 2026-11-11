package routerssh

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"strings"
	"testing"
)

// A bounded lint for known RouterOS hazards, not a RouterOS parser. It scans
// generated code: ppLiteral strings already escape dollars, including nested code.
func collectorSyntaxHazards(script string) (rawHealth, rawDollar bool) {
	masked := []byte(script)
	quoted, escaped := false, false
	for i, c := range []byte(script) {
		if quoted {
			masked[i] = ' '
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
			} else if c == '"' {
				quoted = false
			} else if c == '$' {
				rawDollar = true
			}
		} else if c == '"' {
			quoted = true
			masked[i] = ' '
		}
	}
	return strings.Contains(string(masked), "/system health"), rawDollar
}

func assertCollectorSyntaxExamples(t *testing.T) {
	for _, bad := range []string{`[/system health get temperature]`, `[/system health get $id value]`, `:put "^[A-Z]+$"`} {
		h, d := collectorSyntaxHazards(bad)
		if !h && !d {
			t.Fatal("guard missed independent negative", bad)
		}
	}
	for _, valid := range []string{
		`:put ` + routeros.Literal(`^[A-Z]+$`),
		`[:parse ` + routeros.Literal(`:return [/system health get temperature]`) + `]`,
		`[/interface get $id name]`,
	} {
		h, d := collectorSyntaxHazards(valid)
		if h || d {
			t.Fatal("guard rejects escaped/deferred/other menu", valid)
		}
	}
}
