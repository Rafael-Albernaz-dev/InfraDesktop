package radio

import (
	"math"
	"testing"
	"time"
)

func TestRadioCostAndProvenance(t *testing.T) {
	samples := []float64{1, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 2}
	cpu := make([][]int, 10)
	for i := range cpu {
		cpu[i] = []int{6, 7, 6}
	}
	c := RadioCostGate(samples, cpu)
	if !c.Enabled || c.P95 != 2 || c.Interval != 6 {
		t.Fatal(c)
	}
	for _, change := range []string{"cpu", "missing", "nan"} {
		s := append([]float64{}, samples...)
		p := make([][]int, 10)
		copy(p, cpu)
		switch change {
		case "cpu":
			p[4] = []int{6, 91, 7}
		case "missing":
			p[1] = nil
		case "nan":
			s[9] = math.NaN()
		}
		if RadioCostGate(s, p).Enabled {
			t.Fatal("unsafe live gate", change)
		}
	}
	assertRadioMetricProvenance(t)
}

func TestRadioLinkRequiresExactInterfaceBSSIDAndMAC(t *testing.T) {
	l := RadioLink{AP: "ap", MAC: "02:00:00:00:10:03", Interface: "virtual", BSSID: "02:00:00:00:10:06"}
	r := &RadioReading{Compatible: true,
		DeviceID: "ap",
		Clients: []RadioClient{{MAC: l.MAC,
			Interface: l.Interface,
			BSSID:     l.BSSID,
			Name:      "same SSID or name is irrelevant"}}}
	if !RadioLinkPresent(l, r) {
		t.Fatal("exact association missing")
	}
	for _, field := range []string{"mac", "interface", "bssid", "device"} {
		copy := *r
		copy.Clients = append([]RadioClient{}, r.Clients...)
		switch field {
		case "mac":
			copy.Clients[0].MAC = "02:00:00:00:10:01"
		case "interface":
			copy.Clients[0].Interface = "wlan1"
		case "bssid":
			copy.Clients[0].BSSID = "02:00:00:00:10:05"
		case "device":
			copy.DeviceID = "other"
		}
		if RadioLinkPresent(l, &copy) {
			t.Fatal("name or partial identity was accepted", field)
		}
	}
}

func TestRadioChainExtremaAndLiveLimits(t *testing.T) {
	at := time.Now()
	d := Device{ID: "ap", Revision: 2, Address: "192.0.2.1", Port: 22, Transport: TransportSSH, CredentialID: "credential", Fingerprint: "pin"}
	makeReading := func(value float64) *RadioReading {
		r := &RadioReading{Perspective: "AP", Metrics: []RadioMetric{{Name: "chain0", Unit: "dBm", Value: value, Direction: "CPE → AP"}}}
		StampRadio(r, d, 1, at)
		return r
	}
	a, b := makeReading(-61), makeReading(-70)
	AccumulateRadioChains(a, b)
	if *b.Metrics[0].Best != -61 || *b.Metrics[0].Worst != -70 {
		t.Fatal("observation extrema", b.Metrics)
	}
	c := makeReading(-58)
	AccumulateRadioChains(b, c)
	if *c.Metrics[0].Best != -58 || *c.Metrics[0].Worst != -70 {
		t.Fatal("extrema continuation", c.Metrics)
	}
	c.Connection = "changed"
	fresh := makeReading(-66)
	AccumulateRadioChains(c, fresh)
	if *fresh.Metrics[0].Best != -66 {
		t.Fatal("different connection contaminated extrema")
	}
	rtt := float64(50)
	safe := &RadioReading{CommandSeconds: .4, Seconds: 1, CPU: []int{6, 7, 6}, PingRTT: &rtt}
	cost := RadioCost{Enabled: true, PingBaseline: 45, PingLimit: 90, CPUBaseline: 6}
	if RadioLiveLimit(safe, cost) {
		t.Fatal("healthy read aborted")
	}
	rtt = 91
	if !RadioLiveLimit(safe, cost) {
		t.Fatal("latency increase ignored")
	}
	rtt = 50
	safe.CPU = []int{6, 91, 7}
	if !RadioLiveLimit(safe, cost) {
		t.Fatal("CPU above90 ignored")
	}
}
