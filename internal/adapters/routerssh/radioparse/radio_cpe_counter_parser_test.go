package radioparse_test

import (
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"os"
	"testing"
)

func TestRadioFamilyCounterParsers(t *testing.T) {
	for _, tc := range []struct {
		file          string
		parse         func(string) (*equipment.RadioReading, equipment.Code)
		tx, rx, retry string
	}{{"l07_airos_xm_real.txt",
		radioparse.ParseRadioAirOS,
		"138930598",
		"2320629485",
		""},
		{"l07_wom_iw_real.txt",
			radioparse.ParseRadioWOM,
			"209016980",
			"3358212272",
			"105159"}} {
		b, e := os.ReadFile("testdata/" + tc.file)
		if e != nil {
			t.Fatal(e)
		}
		r, c := tc.parse(string(b))
		if c != equipment.OK {
			t.Fatal(c)
		}
		if r.Counters["tx_bytes"] != tc.tx || r.Counters["rx_bytes"] != tc.rx || r.Counters["tx_retries"] != tc.retry {
			t.Fatal(tc.file, r.Counters)
		}
	}
}
