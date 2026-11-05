package ipc

import (
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"os"
	"strings"
	"testing"
)

func TestRadioIPCIndependentSidesAndCPERevocation(t *testing.T) {
	s := newRadioIndependentFixture(t)
	s.verifyInitialReadings()
	s.confirmLink()
	s.verifyIndependentFailures()
	before := s.revokeCPEAndRequireExplicitChoice()
	s.recoverExplicitChoice(before)
}

func TestRadioWireRejectsCommandsAndUnknownFields(t *testing.T) {
	base := map[string]any{"version": 1,
		"request_id": "radio-test",
		"op":         "radio.start",
		"capability": strings.Repeat("a",
			64),
		"contract_revision":   ProductRevision,
		"session":             "session",
		"profile_id":          "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa",
		"activation_id":       "activation",
		"expected_profile":    "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa",
		"expected_activation": "activation",
		"radio": map[string]any{"device_id": "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb",
			"revision":     2,
			"operation_id": "cccccccc-cccc-4ccc-cccc-cccccccccccc",
			"family":       equipment.RadioRouterOSWirelessV6,
			"mode":         "once"}}
	b, _ := json.Marshal(base)
	if _, c := validate(b); c != foundation.OK {
		t.Fatal("valid radio envelope", c)
	}
	for _, key := range []string{"command", "shell", "address", "secret", "credential_id"} {
		nested := base["radio"].(map[string]any)
		nested[key] = "forbidden"
		b, _ = json.Marshal(base)
		if _, c := validate(b); c != foundation.Invalid {
			t.Fatal("UI supplied unmanaged field", key, c)
		}
		delete(nested, key)
	}
}

func TestRadioLiveContractRevisionAndTiming(t *testing.T) {
	if ProductRevision != 20 {
		t.Fatal("standalone radio fields require IPC20")
	}
	raw,
		e := json.Marshal(equipment.RadioCost{Measurements: []equipment.RadioMeasurement{{AP: equipment.RadioTiming{CommandSeconds: .4,
		PathSeconds: 2},
		CPE: equipment.RadioTiming{CommandSeconds: .5,
			PathSeconds: 3},
		Seconds: 5}}})
	if e != nil {
		t.Fatal(e)
	}
	var wire map[string]json.RawMessage
	if e = json.Unmarshal(raw, &wire); e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		AP, CPE struct {
			Command float64 `json:"command_seconds"`
			Path    float64 `json:"path_seconds"`
		}
		Seconds float64 `json:"seconds"`
	}
	if e = json.Unmarshal(wire["measurements"],
		&rows); e != nil ||
		len(rows) != 1 ||
		rows[0].AP.Command != .4 ||
		rows[0].CPE.Command != .5 ||
		rows[0].AP.Path != 2 ||
		rows[0].CPE.Path != 3 ||
		rows[0].Seconds != 5 {
		t.Fatal("additive timing contract", string(raw), e)
	}
}

func TestRadioCPULimitContract(t *testing.T) {
	raw, e := os.ReadFile("schema-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Limits map[string]json.RawMessage `json:"radio_limits"`
	}
	if e = json.Unmarshal(raw, &schema); e != nil {
		t.Fatal(e)
	}
	if string(schema.Limits["max_cpu_percent"]) != "90" || schema.Limits["max_cpu_increase_points"] != nil {
		t.Fatal("CPU policy does not match approved absolute90", schema.Limits)
	}
}

func TestRadioReceiptFitsFrameAndCountersStayExact(t *testing.T) {
	series := make([]equipment.RadioTickReceipt, 90)
	for i := range series {
		series[i] = equipment.RadioTickReceipt{At: "2026-10-30T00:00:00.123456789Z",
			CPU:    []int{90},
			AP:     .4,
			CPE:    .5,
			ICMP:   []float64{1},
			Reason: "OK"}
	}
	v := equipment.RadioView{Reading: &equipment.RadioReading{Scope: "link-tick",
		Counters: map[string]string{"tx_bytes": "18446744073709551615"}},
		Cost: &equipment.RadioCost{Series: series}}
	for i := 0; i < 10; i++ {
		v.Cost.Measurements = append(v.Cost.Measurements,
			equipment.RadioMeasurement{AP: equipment.RadioTiming{CommandSeconds: .4,
				PathSeconds: .5},
				CPE: equipment.RadioTiming{CommandSeconds: .4,
					PathSeconds: .5}})
	}
	raw, e := json.Marshal(v)
	if e != nil || len(raw) > 48*1024 {
		t.Fatal("radio receipt exceeds response contract", len(raw), e)
	}
	if !strings.Contains(string(raw), `"tx_bytes":"18446744073709551615"`) || !strings.Contains(string(raw), `"cpu":[90]`) {
		t.Fatal("lossy wire counters/series", string(raw))
	}
	var decoded equipment.RadioView
	if e = json.Unmarshal(raw, &decoded); e != nil || len(decoded.Cost.Series) != 90 || decoded.Cost.Series[89].AP != .4 {
		t.Fatal("receipt round trip", e)
	}
}
