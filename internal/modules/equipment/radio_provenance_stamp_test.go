package equipment

import (
	"testing"
	"time"
)

func assertRadioMetricProvenance(t *testing.T) {
	t.Helper()
	d := Device{ID: "target",
		Address:      "192.0.2.1",
		Port:         22,
		Transport:    TransportSSH,
		Revision:     7,
		CredentialID: "credential",
		Fingerprint:  "pin"}
	r := &RadioReading{Perspective: "AP",
		Clients: []RadioClient{{Interface: "wlan1",
			Metrics: []RadioMetric{{Name: "chain0",
				Value:     -61,
				Unit:      "dBm",
				Direction: "CPE → AP"}}}}}
	StampRadio(r, d, 3, time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC))
	m := r.Clients[0].Metrics[0]
	if m.Perspective != "AP" ||
		m.DeviceID != d.ID ||
		m.Interface != "wlan1" ||
		m.At != r.At ||
		m.Quality != "observed" ||
		m.Best == nil ||
		*m.Best != -61 ||
		m.Worst == nil ||
		*m.Worst != -61 {
		t.Fatal(m)
	}
}
