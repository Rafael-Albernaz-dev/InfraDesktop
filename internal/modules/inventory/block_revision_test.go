package inventory

import (
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestBlockOnlyKeepsRevisionAndStillRejectsStaleWrites(t *testing.T) {
	original := Device{ID: uuid.NewString(), Name: "Sintetico", Address: "192.0.2.1", Port: 22,
		Transport: equipment.TransportSSH, Role: equipment.RoleOther, Revision: 7, Active: true}
	for _, mode := range []string{"edit", "bulk"} {
		t.Run(mode, func(t *testing.T) {
			st := State{Devices: []Device{original}}
			if mode == "edit" {
				next := original
				next.Block = "Torre sintetica"
				if code := prepareDeviceUpdate(&st.Devices[0], Command{Op: "inventory.update", Revision: 7}, &next); code != OK {
					t.Fatal(code)
				}
				if next.Revision != 7 {
					t.Fatalf("block changed revision to %d", next.Revision)
				}
				if code := prepareDeviceUpdate(&st.Devices[0], Command{Op: "inventory.update", Revision: 6}, &next); code != Revision {
					t.Fatalf("stale accepted: %s", code)
				}
				return
			}
			block := "Torre sintetica"
			q := Command{Operation: uuid.NewString(), Bulk: &equipment.BulkUpdate{
				Block: &block, Items: []equipment.BulkItem{{ID: original.ID, Revision: 7}},
			}}
			var v View
			changed := false
			if code := applyBulk(&st, q, "synthetic", &v, &changed); code != OK {
				t.Fatal(code)
			}
			if st.Devices[0].Revision != 7 || v.BulkReceipt.Items[0].Revision != 7 {
				t.Fatalf("block changed revision: %d", st.Devices[0].Revision)
			}
			q.Operation = uuid.NewString()
			q.Bulk.Items[0].Revision = 6
			if code := applyBulk(&st, q, "stale", &v, &changed); code != Revision {
				t.Fatalf("stale accepted: %s", code)
			}
		})
	}
}

func TestBlockAndNameStillAdvanceRevision(t *testing.T) {
	old := Device{ID: uuid.NewString(), Name: "Antes", Address: "192.0.2.1", Port: 22,
		Transport: equipment.TransportSSH, Role: equipment.RoleOther, Revision: 7, Active: true}
	next := old
	next.Name = "Depois"
	next.Block = "Torre sintetica"
	if code := prepareDeviceUpdate(&old, Command{Op: "inventory.update", Revision: 7}, &next); code != OK || next.Revision != 8 {
		t.Fatalf("unexpected revision %d: %s", next.Revision, code)
	}
}
