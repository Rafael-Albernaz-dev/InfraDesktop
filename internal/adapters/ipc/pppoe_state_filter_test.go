package ipc

import (
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"strings"
	"testing"
)

func TestPPPoEStateFilterIsClosedAndBoundToCursor(t *testing.T) {
	profile := uuid.NewString()
	for _, op := range []string{"pppoe.sessions-list", "pppoe.compare"} {
		q := Request{Version: 1, RequestID: "state", ContractRevision: ProductRevision, Op: op,
			Capability: strings.Repeat("a", 64), Session: "s", Profile: profile, Activation: "active",
			ExpectedProfile: profile, ExpectedActivation: "active", PPPoE: &equipment.PPInput{DeviceID: uuid.NewString(),
				Query: &equipment.PPQuery{}, ObservationID: uuid.NewString()}}
		if op == "pppoe.compare" {
			q.PPPoE.ObservationID = ""
			q.PPPoE.BaseID = uuid.NewString()
			q.PPPoE.TargetID = uuid.NewString()
		}
		for _, state := range []string{"", "ONLINE", "OFFLINE", "online", "UNKNOWN", "' OR 1=1"} {
			q.PPPoE.Query.State = state
			raw, err := json.Marshal(q)
			if err != nil {
				t.Fatal(err)
			}
			_, code := validate(raw)
			want := foundation.Invalid
			if state == "" || state == "ONLINE" || state == "OFFLINE" {
				want = foundation.OK
			}
			if code != want {
				t.Fatalf("%s state=%q got=%s want=%s", op, state, code, want)
			}
		}
		q.PPPoE.Query.State = "ONLINE"
		server := &Server{}
		q.PPPoE.Cursor = server.ppCursor(q, 16)
		if offset, ok := server.ppOffset(q); !ok || offset != 16 {
			t.Fatal("valid state cursor refused")
		}
		q.PPPoE.Query.State = "OFFLINE"
		if _, ok := server.ppOffset(q); ok {
			t.Fatal("cursor reused across state filter")
		}
	}
}
