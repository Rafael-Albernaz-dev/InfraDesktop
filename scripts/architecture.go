// Command architecture checks project dependency closure; standard-library internals are leaves.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type pkg struct {
	ImportPath, Name                   string
	Imports, TestImports, XTestImports []string
	Standard                           bool
}
type graph map[string][]string

var neutral = map[string]bool{"context": true, "net/netip": true, "sync": true, "sync/atomic": true, "time": true, "unicode": true, "unicode/utf8": true, "strings": true, "errors": true, "fmt": true, "math": true, "sort": true, "strconv": true, "testing": true, "reflect": true}

func violation(g graph, start string) string {
	var walk func(string, []string, map[string]bool) string
	walk = func(n string, path []string, seen map[string]bool) string {
		if seen[n] {
			return ""
		}
		seen[n] = true
		path = append(path, n)
		if !strings.HasPrefix(n, "infradesktop/") {
			if !neutral[n] {
				return strings.Join(path, " -> ")
			}
			return ""
		}
		if strings.Contains(n, "/adapters/") || strings.Contains(n, "/composition") {
			return strings.Join(path, " -> ")
		}
		for _, to := range g[n] {
			if v := walk(to, path, seen); v != "" {
				return v
			}
		}
		return ""
	}
	return walk(start, nil, map[string]bool{})
}
func main() {
	fixtures := []struct {
		name string
		g    graph
		bad  bool
	}{
		{"direct os", graph{"infradesktop/internal/modules/x": {"os"}}, true},
		{"transitive helper net", graph{"infradesktop/internal/modules/x": {"infradesktop/internal/helper"}, "infradesktop/internal/helper": {"net"}}, true},
		{"transitive Wails", graph{"infradesktop/internal/modules/x": {"infradesktop/internal/helper"}, "infradesktop/internal/helper": {"github.com/wailsapp/wails/v2"}}, true},
		{"test import adapter", graph{"infradesktop/internal/modules/x": {"infradesktop/internal/adapters/ipc"}}, true},
		{"neutral helper", graph{"infradesktop/internal/modules/x": {"infradesktop/internal/helper"}, "infradesktop/internal/helper": {"context", "time"}}, false},
	}
	for _, f := range fixtures {
		v := violation(f.g, "infradesktop/internal/modules/x")
		if (v != "") != f.bad {
			panic("guard fixture failed: " + f.name)
		}
		fmt.Println("guard fixture:", f.name, "rejected=", v != "")
	}
	cmd := exec.Command("go", "list", "-tags", "webkit2_41", "-deps", "-test", "-json", "./...")
	cmd.Stderr = os.Stderr
	b, e := cmd.Output()
	if e != nil {
		panic(e)
	}
	d := json.NewDecoder(strings.NewReader(string(b)))
	g := graph{}
	var packages []pkg
	for {
		var p pkg
		e = d.Decode(&p)
		if e == io.EOF {
			break
		}
		if e != nil {
			panic(e)
		}
		packages = append(packages, p)
		g[p.ImportPath] = append(append(p.Imports, p.TestImports...), p.XTestImports...)
	}
	bad := false
	for _, p := range packages {
		// Go-generated test main imports os; authored test imports remain in the checked graph.
		if p.Name == "main" && strings.HasSuffix(p.ImportPath, ".test") {
			continue
		}
		if !strings.HasPrefix(p.ImportPath, "infradesktop/") {
			continue
		}
		if strings.Contains(p.ImportPath, "/modules/") {
			if v := violation(g, p.ImportPath); v != "" {
				fmt.Fprintln(os.Stderr, "ARQ-001 violation:", v)
				bad = true
			}
		}
		if strings.Contains(p.ImportPath, "/adapters/") {
			for _, to := range p.Imports {
				if strings.Contains(to, "/composition") || to == "infradesktop" {
					fmt.Fprintln(os.Stderr, "composition points outward:", p.ImportPath, to)
					bad = true
				}
			}
		}
		if !strings.Contains(p.ImportPath, " [") {
			fmt.Printf("%s -> %s\n", p.ImportPath, strings.Join(g[p.ImportPath], ", "))
		}
	}
	if bad {
		os.Exit(1)
	}
	fmt.Println("ARQ-001 Go direct/transitive graph: PASS")
}
