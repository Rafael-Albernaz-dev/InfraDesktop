package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestRepeatedBlockPreservesSavedReading(t *testing.T) {
	for _, mode := range []string{"bulk", "edit"} {
		t.Run(mode, func(t *testing.T) {
			j, target := ppStoreSetup(t)
			d := target.Device
			stamp := time.Now().UTC().Format(time.RFC3339Nano)
			_, err := j.st.unit.Exec(`INSERT INTO device_equipment_snapshot VALUES(?,?,?,?,?,?,?,?)`,
				d.ID, d.Revision, target.Credential.Metadata.Revision,
				equipment.PPConnection(d), time.Now().Unix(), stamp, `{}`, `[]`)
			if err != nil {
				t.Fatal(err)
			}
			q := equipmentScope(j)
			q.Operation = uuid.NewString()
			if mode == "bulk" {
				block := d.Block
				q.Op = "inventory.bulk-update"
				q.Bulk = &equipment.BulkUpdate{Items: []equipment.BulkItem{{ID: d.ID, Revision: d.Revision}}, Block: &block}
			} else {
				q.Op, q.ID, q.Revision = "inventory.update", d.ID, d.Revision
				q.Name, q.Address, q.Port, q.Transport = d.Name, d.Address, d.Port, d.Transport
				q.Role, q.CredentialID, q.Block = d.Role, d.CredentialID, d.Block
			}
			equipmentCall(t, newEquipment(t, j), q, equipment.OK)
			rows, err := readEquipmentInsights(context.Background(), j.st.unit, d.ID, "health", time.Now())
			if err != nil || len(rows) == 0 {
				t.Fatalf("saved reading lost after repeated block: %v %v", rows, err)
			}
		})
	}
}
