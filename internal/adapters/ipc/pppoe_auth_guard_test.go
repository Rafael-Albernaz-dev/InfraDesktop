package ipc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"

	"github.com/google/uuid"
)

func TestPPPoERealClientAuthorizationAndCursorBoundaries(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{PPRows: 32})
	a := j.collect(uuid.NewString())
	q := equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, Query: &equipment.PPQuery{Search: "synthetic"}}
	page := j.call("pppoe.sessions-list", q, foundation.OK).Data.PPPoE
	if page.Next == "" || len(page.Rows) != 16 {
		t.Fatal("positive page control")
	}
	q.Cursor = page.Next
	q.Query.Search = "other"
	j.call("pppoe.sessions-list", q, foundation.Code(equipment.CursorInvalid))
	q.Query.Search = "synthetic"
	second := j.collect(uuid.NewString())
	q.ObservationID = second.ID
	j.call("pppoe.sessions-list", q, foundation.Code(equipment.CursorInvalid))
	auths := j.peer.Auth.Load()
	request := Request{
		Op: "pppoe.current", ExpectedProfile: uuid.NewString(), ExpectedActivation: j.client.activation,
		PPPoE: &equipment.PPInput{DeviceID: j.device},
	}
	if r := j.client.Call(context.Background(), request); r.Code != foundation.ProfileChanged {
		t.Fatal("context bypass", r.Code)
	}
	j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK)
	for op, allFields := range ppFields {
		for _, fields := range ppFieldShapes(op, allFields) {
			in := ppGuardInput(t, j, op, fields, second.ID)
			guarded := Request{Op: op, ExpectedProfile: uuid.NewString(), ExpectedActivation: j.client.activation, PPPoE: &in}
			if r := j.client.Call(context.Background(), guarded); r.Code != foundation.ProfileChanged {
				t.Fatal("operation escaped common auth guard", op, fields, r.Code)
			}
		}
	}
	if j.peer.Auth.Load() != auths {
		t.Fatal("negative/local operation connected")
	}
}

// ppGuardInput fills one accepted request shape with valid values, so a rejection
// can only come from the common profile guard.
func ppGuardInput(t *testing.T, j *ppJourney, op string, fields []string, observation string) equipment.PPInput {
	t.Helper()
	raw := map[string]any{}
	for _, field := range fields {
		raw[field] = ppGuardValue(op, field, fields, j.device, observation)
	}
	b, _ := json.Marshal(raw)
	var in equipment.PPInput
	if e := json.Unmarshal(b, &in); e != nil {
		t.Fatal("test input", op, fields, e)
	}
	return in
}

func ppGuardValue(op, field string, fields []string, device, observation string) any {
	switch field {
	case "revision":
		return 2
	case "query":
		return map[string]any{}
	case "export_key":
		return base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
	case "export_part":
		return "diagnostics"
	case "scope":
		return ppGuardScope(op, fields)
	case "row_ids":
		return []string{uuid.NewString()}
	case "cursor":
		return ""
	case "device_id":
		return device
	case "observation_id", "base_id", "target_id":
		return observation
	default:
		return uuid.NewString()
	}
}

func ppGuardScope(op string, fields []string) string {
	switch op {
	case "pppoe.diagnostic-start":
		return equipment.PDPing
	case "pppoe.monitor-current":
		return "equipment"
	case "pppoe.read-start":
		if len(fields) == 4 {
			return "operations"
		}
		return "bandwidth"
	default:
		return equipment.PDCatalog
	}
}

// ppFieldShapes returns every accepted request shape of op. pppoe.read-cancel takes
// either job_id alone or device_id+operation_id; pppoe.read-start has a device-wide
// shape and a row-scoped shape. Every shape must reach the auth guard.
func ppFieldShapes(op string, fields []string) [][]string {
	switch op {
	case "pppoe.read-cancel":
		return [][]string{{"job_id"}, {"device_id", "operation_id"}}
	case "pppoe.read-start":
		return [][]string{{"device_id", "revision", "operation_id", "scope"}, fields}
	default:
		return [][]string{fields}
	}
}
