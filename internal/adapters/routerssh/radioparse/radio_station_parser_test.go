package radioparse

import (
	"infradesktop/internal/modules/equipment"
	"os"
	"testing"
)

func TestStandaloneStationGoldenV6(t *testing.T) {
	b, e := os.ReadFile("testdata/l07_avulso_station_v6.txt")
	if e != nil {
		t.Fatal(e)
	}
	r, c := ParseRadioStationV6(string(b))
	if c != equipment.OK {
		t.Fatalf("golden estação v6 recusado: %s", c)
	}
	if r.Version != "6.49.18 (long-term)" ||
		r.Model != "RBLHG-5HPnD" ||
		r.WirelessMAC != "02:00:00:00:20:01" ||
		r.APBSSID != "02:00:00:00:20:02" {
		t.Fatalf("identificação incorreta: %+v", r)
	}
	if r.Mode != "station-bridge" || r.WirelessProtocol != "nv2-nstreme-802.11" {
		t.Fatalf("modo/protocolo: %+v", r)
	}
	want := map[string]float64{"chain0": -81, "chain1": -85, "snr": 40, "ccq_tx": 64, "ccq_rx": 55, "rate_tx": 28.8, "rate_rx": 30}
	for _, m := range r.Metrics {
		if v, ok := want[m.Name]; ok && m.Value == v {
			delete(want, m.Name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("métricas ausentes ou incorretas: %v", want)
	}
	assertEmptyRadioHealth(t, r.Metrics)
}
