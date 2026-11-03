package ipc

import (
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestRadioMeasurementWireKeepsLowercaseSideKeys(t *testing.T) {
	raw, err := json.Marshal(equipment.RadioMeasurement{
		AP: equipment.RadioTiming{CommandSeconds: 1}, CPE: equipment.RadioTiming{CommandSeconds: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["ap"]; !ok {
		t.Fatalf("missing ap contract key: %s", raw)
	}
	if _, ok := fields["cpe"]; !ok {
		t.Fatalf("missing cpe contract key: %s", raw)
	}
	if _, ok := fields["AP"]; ok {
		t.Fatalf("unexpected Go field key: %s", raw)
	}
}
