package routerssh

import (
	"infradesktop/internal/adapters/routerssh/deviceparse"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
	"time"
)

func diagnosticCommands(t *testing.T) map[string]string {
	t.Helper()
	band, ok := pppoeparse.BuildPPBandCommand([]equipment.PPRow{{Login: "lab-user"}})
	if !ok {
		t.Fatal("band target")
	}
	log, ok := pppoeparse.BuildPPLogCommand("lab-user")
	if !ok {
		t.Fatal("log target")
	}
	secret, ok := pppoeparse.BuildPPSecretCommand("lab-user")
	if !ok {
		t.Fatal("secret target")
	}
	out := map[string]string{"IdentityCommand": deviceparse.IdentityCommand,
		"ResourceCommand": deviceparse.ResourceCommand, "InterfacesCommand": deviceparse.InterfacesCommand,
		"PPPoECommand": pppoeparse.PPPoECommand, "PPContextCommand": pppoeparse.PPContextCommand,
		"PDCatalogCommand": pppoeparse.PDCatalogCommand, "PDRadiusCommand": pppoeparse.PDRadiusCommand,
		"PPHealthFirstCommand": pppoeparse.PPHealthFirstCommand, "PPHealthCommand": pppoeparse.PPHealthCommand,
		"DeviceFastTrackCommand": deviceparse.DeviceFastTrackCommand, "DevicePortsCommand": deviceparse.DevicePortsCommand,
		"DeviceOperationalCommand":  deviceparse.DeviceOperationalCommand,
		"DeviceAuthFailuresCommand": deviceparse.DeviceAuthFailuresCommand, "BuildPPBandCommand": band,
		"BuildPPLogCommand": log, "BuildPPSecretCommand": secret}
	for _, family := range []string{"6", "7"} {
		out["BuildPDPingCommand-v"+family] = diagnosticPingCommand(t, family)
	}
	return out
}

func TestL06V7SyntaxDiagnosticsAreStructural(t *testing.T) {
	for _, s := range []string{"syntax error (line 1 column 1304)\r\n", "expected end of command (line 1 column 1471)",
		"bad command name health (line 1 column 1)"} {
		if !routeros.CollectorSyntaxDiagnostic(s) {
			t.Fatal("known diagnostic missed")
		}
	}
	for _, s := range []string{"F\tsyntax error\t01:00:00\n", "I\tbad command name health\tether\n",
		"prefix syntax error (line 1 column 1)", strings.Repeat("x", 300)} {
		if routeros.CollectorSyntaxDiagnostic(s) {
			t.Fatal("projected or arbitrary text misclassified")
		}
	}
}

func diagnosticPingCommand(t *testing.T, family string) string {
	o, c := pppoeparse.ParsePPPoE("H\t1\t1\t" + family + ".14.3\t2d1h1s\nR\t*1\t1\tlab-user\t198.51.100.1\t02:00:00:00:00:01\t1h1s" +
		"\nZ\t1\tcomplete\t2d1h1s\n")
	if c != equipment.OK {
		t.Fatal("ping observation")
	}
	body, ok := pppoeparse.BuildPDPingCommand(&equipment.PPPingTarget{Observation: *o, Row: o.Rows[0],
		Age: equipment.PPInterval{Known: true, Max: int64(100 * time.Millisecond)}, Prepared: time.Now()})
	if !ok {
		t.Fatal("ping target", family)
	}
	return body
}
