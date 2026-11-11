package routerssh

import (
	"go/ast"
	"go/parser"
	"go/token"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every exported collector command must generate a reviewable diagnostic and pass
// the RouterOS syntax guard, radio commands included.
func TestEveryExportedCollectorHasDiagnostic(t *testing.T) {
	commands := diagnosticCommands(t)
	for name, body := range radioDiagnosticCommands() {
		commands[name] = body
	}
	for _, name := range exportedCollectorNames(t) {
		if commands[name] == "" {
			t.Error("exported collector lacks generated diagnostic", name)
		}
	}
	out := os.Getenv("L06_DIAGNOSTIC_DIR")
	if out == "" {
		out = t.TempDir()
	} else if e := os.MkdirAll(out, 0700); e != nil {
		t.Fatal(e)
	}
	for name, body := range commands {
		assertCollectorSyntax(t, name, body)
		path := filepath.Join(out, name+".rsc")
		if e := os.WriteFile(path, []byte(body+"\n"), 0600); e != nil {
			t.Fatal(e)
		}
		if actual, e := os.ReadFile(path); e != nil || string(actual) != body+"\n" {
			t.Fatal("diagnostic drift", name)
		}
	}
}

func radioDiagnosticCommands() map[string]string {
	return map[string]string{
		"RadioAPCommand": radioparse.RadioAPCommand, "RadioAPTickCommand": radioparse.RadioAPTickCommand("02:00:00:00:00:01"),
		"RadioIndexCommand": radioparse.RadioIndexCommand, "RadioIdentificationCommand": radioparse.RadioIdentificationCommand,
		"RadioStationCommand": radioparse.RadioStationCommand, "RadioAirOSCommand": radioparse.RadioAirOSCommand,
		"RadioWOMCommand": radioparse.RadioWOMCommand, "RadioWOMTickCommand": radioparse.RadioWOMTickCommand,
		"RadioWOMIdentificationCommand": radioparse.RadioWOMIdentificationCommand,
	}
}

// exportedCollectorNames lists exported *Command identifiers of production files.
func exportedCollectorNames(t *testing.T) []string {
	t.Helper()
	var names []string
	err := filepath.WalkDir(".", func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		names = append(names, collectorNamesIn(f)...)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return names
}

func collectorNamesIn(f *ast.File) []string {
	var names []string
	add := func(name string) {
		if !ast.IsExported(name) || !strings.HasSuffix(name, "Command") {
			return
		}
		if name == "BuildPDPingCommand" {
			name += "-v7"
		}
		names = append(names, name)
	}
	for _, d := range f.Decls {
		switch v := d.(type) {
		case *ast.FuncDecl:
			add(v.Name.Name)
		case *ast.GenDecl:
			for _, s := range v.Specs {
				if values, ok := s.(*ast.ValueSpec); ok {
					for _, n := range values.Names {
						add(n.Name)
					}
				}
			}
		}
	}
	return names
}

// linuxShellCollectors are sent to airOS/WOM shells, not to the RouterOS console.
var linuxShellCollectors = map[string]bool{
	"RadioAirOSCommand": true, "RadioWOMCommand": true,
	"RadioWOMTickCommand": true, "RadioWOMIdentificationCommand": true,
}

func assertCollectorSyntax(t *testing.T, name, body string) {
	t.Helper()
	if linuxShellCollectors[name] {
		return // RouterOS quoting rules do not apply to a Linux shell.
	}
	if h, d := collectorSyntaxHazards(body); d || (h && !onlyHealthPrint(name, body)) {
		t.Errorf("unsafe generated collector %s health=%v dollar=%v", name, h, d)
	}
}

// onlyHealthPrint admits the v6 station read, whose `/system health print` was
// observed in the sanitized v6 station golden; `get` forms stay forbidden because
// their grammar differs between RouterOS v6 and v7.
func onlyHealthPrint(name, body string) bool {
	return name == "RadioStationCommand" &&
		strings.Count(body, "/system health") == strings.Count(body, "/system health print")
}
