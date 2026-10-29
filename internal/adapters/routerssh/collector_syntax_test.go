package routerssh

import (
	"infradesktop/internal/modules/equipment"
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
func TestL06V7CollectorSyntaxGuard(t *testing.T) {
	for name, script := range map[string]string{"operations": DeviceOperationalCommand, "auth": DeviceAuthFailuresCommand} {
		t.Run(name, func(t *testing.T) {
			health, dollar := collectorSyntaxHazards(script)
			if health || dollar {
				t.Fatalf("version-specific health outside parse=%v; unescaped string dollar=%v", health, dollar)
			}
		})
	}
	for _, bad := range []string{`[/system health get temperature]`, `[/system health get $id value]`, `:put "^[A-Z]+$"`} {
		h, d := collectorSyntaxHazards(bad)
		if !h && !d {
			t.Fatal("guard missed independent negative", bad)
		}
	}
	for _, valid := range []string{`:put ` + ppLiteral(`^[A-Z]+$`), `[:parse ` + ppLiteral(`:return [/system health get temperature]`) + `]`, `[/interface get $id name]`} {
		h, d := collectorSyntaxHazards(valid)
		if h || d {
			t.Fatal("guard rejects escaped/deferred/other menu", valid)
		}
	}
}

// Keep the import contract anchored while subsequent focal cases are added.
var _ = equipment.Partial
