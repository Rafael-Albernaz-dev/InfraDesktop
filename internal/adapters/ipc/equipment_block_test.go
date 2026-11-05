package ipc

import (
	"encoding/json"
	"strings"
	"testing"

	"infradesktop/internal/modules/foundation"
)

func TestEquipmentBlockWireFieldsAreOperationScoped(t *testing.T) {
	for _, test := range []struct {
		op   string
		want foundation.Code
	}{
		{"inventory.create", foundation.OK}, {"inventory.update", foundation.OK}, {"trust.inspect", foundation.Invalid},
	} {
		q := Request{Version: 1, ContractRevision: ProductRevision, RequestID: "block", Op: test.op, Capability: strings.Repeat("a", 64),
			Session: "s", Profile: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", Activation: "a",
			ExpectedProfile: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", ExpectedActivation: "a", Equipment: &EquipmentInput{}}
		raw, err := json.Marshal(q)
		if err != nil {
			t.Fatal(err)
		}
		body := strings.Replace(string(raw), `"equipment":{}`, `"equipment":{"block":"Norte"}`, 1)
		_, got := validate([]byte(body))
		if got != test.want {
			t.Errorf("%s got %s want %s", test.op, got, test.want)
		}
		if got == foundation.OK {
			decoded, code := validate([]byte(body))
			if code != foundation.OK || equipmentCommand(1, "owner", decoded).Block != "Norte" {
				t.Fatal("block lost crossing the IPC boundary")
			}
		}
	}
}
