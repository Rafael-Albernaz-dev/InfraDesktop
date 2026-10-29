package routerssh

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"infradesktop/internal/modules/equipment"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func diagnosticCommands(t *testing.T) map[string]string {
	t.Helper()
	band, ok := BuildPPBandCommand([]equipment.PPRow{{Login: "lab-user"}})
	if !ok {
		t.Fatal("band target")
	}
	log, ok := BuildPPLogCommand("lab-user")
	if !ok {
		t.Fatal("log target")
	}
	secret, ok := BuildPPSecretCommand("lab-user")
	if !ok {
		t.Fatal("secret target")
	}
	out := map[string]string{"IdentityCommand": IdentityCommand, "ResourceCommand": ResourceCommand, "InterfacesCommand": InterfacesCommand, "PPPoECommand": PPPoECommand, "PPContextCommand": PPContextCommand, "PDCatalogCommand": PDCatalogCommand, "PDRadiusCommand": PDRadiusCommand, "PPHealthFirstCommand": PPHealthFirstCommand, "PPHealthCommand": PPHealthCommand, "DeviceFastTrackCommand": DeviceFastTrackCommand, "DevicePortsCommand": DevicePortsCommand, "DeviceOperationalCommand": DeviceOperationalCommand, "DeviceAuthFailuresCommand": DeviceAuthFailuresCommand, "BuildPPBandCommand": band, "BuildPPLogCommand": log, "BuildPPSecretCommand": secret}
	for _, family := range []string{"6", "7"} {
		o, c := ParsePPPoE("H\t1\t1\t" + family + ".14.3\t2d1h1s\nR\t*1\t1\tlab-user\t198.51.100.1\t02:00:00:00:00:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n")
		if c != equipment.OK {
			t.Fatal("ping observation")
		}
		body, ok := BuildPDPingCommand(&equipment.PPPingTarget{Observation: *o, Row: o.Rows[0], Age: equipment.PPInterval{Known: true, Max: int64(100 * time.Millisecond)}, Prepared: time.Now()})
		if !ok {
			t.Fatal("ping target", family)
		}
		out["BuildPDPingCommand-v"+family] = body
	}
	return out
}
func TestL06V7EveryExportedCollectorHasDiagnostic(t *testing.T) {
	commands := diagnosticCommands(t)
	files, e := filepath.Glob("*.go")
	if e != nil {
		t.Fatal(e)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, e := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if e != nil {
			t.Fatal(e)
		}
		require := func(name string) {
			if !ast.IsExported(name) || !strings.HasSuffix(name, "Command") {
				return
			}
			if name == "BuildPDPingCommand" {
				name += "-v7"
			}
			if commands[name] == "" {
				t.Error("exported collector lacks generated diagnostic", name)
			}
		}
		for _, d := range f.Decls {
			switch v := d.(type) {
			case *ast.FuncDecl:
				require(v.Name.Name)
			case *ast.GenDecl:
				for _, s := range v.Specs {
					if values, ok := s.(*ast.ValueSpec); ok {
						for _, n := range values.Names {
							require(n.Name)
						}
					}
				}
			}
		}
	}
	out := os.Getenv("L06_DIAGNOSTIC_DIR")
	if out == "" {
		out = t.TempDir()
	} else if e := os.MkdirAll(out, 0700); e != nil {
		t.Fatal(e)
	}
	for name, body := range commands {
		if h, d := collectorSyntaxHazards(body); h || d {
			t.Errorf("unsafe generated collector %s health=%v dollar=%v", name, h, d)
		}
		path := filepath.Join(out, name+".rsc")
		if e := os.WriteFile(path, []byte(body+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
		actual, e := os.ReadFile(path)
		if e != nil || string(actual) != body+"\n" {
			t.Fatal("diagnostic drift", name)
		}
	}
}
func TestL06V7RealSanitizedGoldens(t *testing.T) {
	sessions, e := os.ReadFile("testdata/device_sessions_v7_real_sanitized.txt")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Count(sessions, []byte("\r\n")) != 55 {
		t.Fatal("real CRLF shape changed")
	}
	o, c := ParsePPPoE(string(sessions))
	if c != equipment.OK || o.Quality.Status != "complete" || o.Quality.Eligible != 52 || len(o.Rows) != 52 {
		t.Fatal("real count prelude/sessions rejected", c, o.Quality)
	}
	if o.Rows[0].Login != "lab-session-001" || o.Rows[51].Address != "198.51.100.52" {
		t.Fatal("synthetic identities changed")
	}
	pools, e := os.ReadFile("testdata/device_pools_v7_real_sanitized.txt")
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Count(pools, []byte("\r\n")) != 58 {
		t.Fatal("real pool CRLF shape changed")
	}
	h, c := ParsePPHealth(string(pools))
	if c != equipment.OK || len(h.Interfaces) != 48 || len(h.Pools) != 4 || !h.PoolsAvailable || !h.UplinksAvailable {
		t.Fatal("real pools/interfaces rejected", c)
	}
	for i, line := range strings.Split(string(pools), "\r\n") {
		if strings.HasPrefix(line, "P\t") {
			lines := strings.Split(string(pools), "\r\n")
			lines[i-1] = "999999"
			if _, c := ParsePPHealth(strings.Join(lines, "\r\n")); c != equipment.Partial {
				t.Fatal("unbound real count echo accepted")
			}
			break
		}
	}
}
func TestL06V7SyntaxDiagnosticsAreStructural(t *testing.T) {
	for _, s := range []string{"syntax error (line 1 column 1304)\r\n", "expected end of command (line 1 column 1471)", "bad command name health (line 1 column 1)"} {
		if !collectorSyntaxDiagnostic(s) {
			t.Fatal("known diagnostic missed")
		}
	}
	for _, s := range []string{"F\tsyntax error\t01:00:00\n", "I\tbad command name health\tether\n", "prefix syntax error (line 1 column 1)", strings.Repeat("x", 300)} {
		if collectorSyntaxDiagnostic(s) {
			t.Fatal("projected or arbitrary text misclassified")
		}
	}
}

func TestL06V7AuthRecognizedCount(t *testing.T) {
	for _, body := range []string{"", "F\tlab-user\t01:00:00\nF\tlab-user\t01:00:01\n"} {
		count := 0
		if body != "" {
			count = 2
		}
		text := "A\t1\t7.14.3\t2d1h1s\n" + body + "G\tauth\t0\nZ\t" + fmt.Sprint(count) + "\tcomplete\t2d1h1s\n"
		rows, c := ParseDeviceAuthFailures(text)
		if c != equipment.OK {
			t.Fatal(c)
		}
		last := rows[len(rows)-1]
		if last.Used == nil || *last.Used != uint64(count) {
			t.Fatal("recognized count missing; zero must be explicit", count)
		}
	}
}
