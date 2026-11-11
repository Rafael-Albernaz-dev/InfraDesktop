package radioparse

import (
	"os"
	"strings"
	"testing"
)

func TestWOMFingerprintRejectsLookalikesAndFailedProbe(t *testing.T) {
	raw, err := os.ReadFile("testdata/radio_wom_library_identity.txt")
	if err != nil {
		t.Fatal(err)
	}
	golden := string(raw)
	cases := map[string]string{
		"generic Linux":            "Linux 3.10.49\nphy#0\n",
		"prefix lookalike":         strings.ReplaceAll(golden, "WOM5A23", "WOM5A230"),
		"word lookalike":           strings.ReplaceAll(golden, "WOM5A23", "WOMBAT"),
		"error carrying token":     strings.ReplaceAll(golden, "WOM5A23", "error WOM5A23"),
		"failed grep":              strings.ReplaceAll(golden, "L07-GREP-EXIT=0", "L07-GREP-EXIT=1"),
		"library missing":          strings.ReplaceAll(golden, "\nEXISTE\n", "\nNAO\n"),
		"later error status":       golden + "L07-GREP-EXIT=1\n",
		"duplicate success status": golden + "L07-GREP-EXIT=0\n",
		"trailing error":           golden + "probe failed\n",
		"no command envelope":      "WOM5A23\nWOM5A23\n",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			if IdentifiedWOMLibrary(text) {
				t.Fatal("unproved software family accepted")
			}
		})
	}
}
