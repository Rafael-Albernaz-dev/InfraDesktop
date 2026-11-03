package ipc

import (
	"infradesktop/internal/adapters/routerssh"
	"os"
	"strings"
	"testing"
)

// Synthetic protocol envelope around the existing station golden. It exercises
// identification and AP fallback; it is not a new physical AP collection.
func radioAPIdentificationFixture(t *testing.T, command string) (string, bool) {
	t.Helper()
	if command == routerssh.RadioIdentificationCommand {
		return "L07-ID-RESOURCE\nplatform: MikroTik\nversion: 6.49.10 (long-term)\n" +
			"L07-ID-PACKAGES\n 0 wireless 6.49.10\nL07-ID-BOARD\nmodel: LHG HP5\nL07-ID-DONE\n", true
	}
	if command != routerssh.RadioStationCommand {
		return "", false
	}
	b, err := os.ReadFile("../routerssh/testdata/l07_avulso_station_v6.txt")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(b), "6.49.18", "6.49.10")
	return strings.ReplaceAll(text, "mode=station-bridge", "mode=ap-bridge"), true
}
