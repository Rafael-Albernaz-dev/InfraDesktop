package ipc

import (
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func TestRadioProjectionRejectsInvalidCPEID(t *testing.T) {
	for _, id := range []string{"", uuid.NewString(), "observed-name"} {
		v := equipment.RadioView{Reading: &equipment.RadioReading{Clients: []equipment.RadioClient{{CPE: id}}}}
		raw, code := encodeRadioProjection(v)
		if id == "observed-name" {
			if code != equipment.Invalid {
				t.Fatal("non UUID CPE escaped IPC", code)
			}
			continue
		}
		if code != equipment.OK {
			t.Fatal("optional/UUID CPE rejected", code)
		}
		if id == "" && strings.Contains(string(raw), "cpe_id") {
			t.Fatal("unbound client gained CPE ID")
		}
	}
}
