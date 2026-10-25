package routerssh

import (
	"encoding/json"
	"fmt"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func TestOperationalEscapedInterfaceBudgetAndIncompleteCounts(t *testing.T) {
	var b strings.Builder
	b.WriteString("eligible=128\ndynamic=4096\n")
	for i := 0; i < 128; i++ {
		fmt.Fprintf(&b, "name=%s%03d\ntype=%s\nrunning=true\ndisabled=false\nend\n", strings.Repeat("<", 125), i, strings.Repeat(">", 80))
	}
	b.WriteString("done\n")
	o := &equipment.Observation{}
	if c := parseInterfaces(o, b.String()); c != equipment.OK {
		t.Fatal(c)
	}
	encoded, e := json.Marshal(o.Interfaces)
	if e != nil || len(encoded) > 32*1024 || len(o.Interfaces) == 128 || len(o.Interfaces) == 0 || o.InterfacePart.Status != "limited" || !o.InterfacePart.CountsKnown || o.InterfacePart.Eligible != 128 {
		t.Fatal("escaped output not honestly bounded", len(encoded))
	}
	if c := parseInterfaces(&equipment.Observation{}, "eligible=100\ndynamic=4096\ndone\n"); c != equipment.Partial {
		t.Fatal("missing rows treated as zero", c)
	}
	empty := &equipment.Observation{}
	if c := parseInterfaces(empty, "eligible=0\ndynamic=10000\ndone\n"); c != equipment.OK || len(empty.Interfaces) != 0 || empty.InterfacePart.Total != 10000 {
		t.Fatal("dynamic-only scope rejected", c)
	}
}
