package storage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestBlockOnlyUpdatePreservesSavedReadings(t *testing.T) {
	for _, mode := range []string{"bulk", "edit"} {
		t.Run(mode, func(t *testing.T) {
			j, target := ppStoreSetup(t)
			ctx := context.Background()
			d := target.Device
			d.Role = equipment.RoleAP
			if c := j.st.UpdateEquipment(ctx, companyX, func(st *equipment.State) equipment.Code {
				for i := range st.Devices {
					if st.Devices[i].ID == d.ID {
						st.Devices[i].Role = d.Role
					}
				}
				return equipment.OK
			}); c != equipment.OK {
				t.Fatal(c)
			}
			saveSyntheticInventoryRadios(t, j, []equipment.Device{d}, target.Credential.Metadata.Revision)
			stamp := time.Now().UTC().Format(time.RFC3339Nano)
			_, err := j.st.unit.Exec(`INSERT INTO device_equipment_snapshot VALUES(?,?,?,?,?,?,?,?)`,
				d.ID, d.Revision, target.Credential.Metadata.Revision, equipment.PPConnection(d), time.Now().Unix(), stamp, `{}`, `[]`)
			if err != nil {
				t.Fatal(err)
			}
			s := newEquipment(t, j)
			q := equipmentScope(j)
			q.Operation = uuid.NewString()
			if mode == "bulk" {
				block := "Torre sintetica"
				q.Op = "inventory.bulk-update"
				q.Bulk = &equipment.BulkUpdate{Items: []equipment.BulkItem{{ID: d.ID, Revision: d.Revision}}, Block: &block}
			} else {
				q.Op, q.ID, q.Revision = "inventory.update", d.ID, d.Revision
				q.Name, q.Address, q.Port, q.Transport = d.Name, d.Address, d.Port, d.Transport
				q.Role, q.CredentialID, q.Block = d.Role, d.CredentialID, "Torre sintetica"
			}
			equipmentCall(t, s, q, equipment.OK)
			rows, err := readEquipmentInsights(ctx, j.st.unit, d.ID, "health", time.Now())
			if err != nil || len(rows) == 0 {
				t.Fatalf("health snapshot missing after block edit: %v %v", rows, err)
			}
			radios, code := j.st.RadioAPReadings(ctx, companyX)
			if code != equipment.OK || len(radios) != 1 {
				t.Fatalf("AP reading missing: %v %s", radios, code)
			}
			radio, _, code := j.st.RadioCurrent(ctx, companyX, d.ID)
			if code != equipment.OK || radio == nil || !radio.Compatible {
				t.Fatalf("radio incompatible: %v %s", radio, code)
			}
			q.Operation = uuid.NewString()
			equipmentCall(t, s, q, equipment.Revision)
		})
	}
}

func TestBlockWithNameChangeDoesNotPreserveReadings(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	d := target.Device
	d.Role = equipment.RoleAP
	if c := j.st.UpdateEquipment(ctx, companyX, func(st *equipment.State) equipment.Code {
		for i := range st.Devices {
			if st.Devices[i].ID == d.ID {
				st.Devices[i].Role = d.Role
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	saveSyntheticInventoryRadios(t, j, []equipment.Device{d}, target.Credential.Metadata.Revision)
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := j.st.unit.Exec(`INSERT INTO device_equipment_snapshot VALUES(?,?,?,?,?,?,?,?)`,
		d.ID, d.Revision, target.Credential.Metadata.Revision, equipment.PPConnection(d), time.Now().Unix(), stamp, `{}`, `[]`)
	if err != nil {
		t.Fatal(err)
	}
	s := newEquipment(t, j)
	q := equipmentScope(j)
	q.Op, q.ID, q.Revision, q.Operation = "inventory.update", d.ID, d.Revision, uuid.NewString()
	q.Name, q.Address, q.Port, q.Transport = "Outro nome", d.Address, d.Port, d.Transport
	q.Role, q.CredentialID, q.Block = d.Role, d.CredentialID, "Torre sintetica"
	equipmentCall(t, s, q, equipment.OK)
	rows, err := readEquipmentInsights(ctx, j.st.unit, d.ID, "health", time.Now())
	if err != nil || len(rows) != 0 {
		t.Fatalf("old health visible after name change: %v %v", rows, err)
	}
	radios, code := j.st.RadioAPReadings(ctx, companyX)
	if code != equipment.OK || len(radios) != 0 {
		t.Fatalf("old radio visible after name change: %v %s", radios, code)
	}
}

func TestBlockReadingFailureRollsBackDeviceAndSnapshot(t *testing.T) {
	j, target := ppStoreSetup(t)
	d := target.Device
	stamp := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := j.st.unit.Exec(`INSERT INTO device_equipment_snapshot VALUES(?,?,?,?,?,?,?,?)`,
		d.ID, d.Revision, target.Credential.Metadata.Revision, equipment.PPConnection(d), time.Now().Unix(), stamp, `{}`, `[]`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = j.st.unit.Exec(`CREATE TRIGGER fail_snapshot_revision BEFORE UPDATE ON device_equipment_snapshot
		BEGIN SELECT RAISE(ABORT, 'synthetic failure'); END`)
	if err != nil {
		t.Fatal(err)
	}
	s := newEquipment(t, j)
	q := equipmentScope(j)
	block := "Torre sintetica"
	q.Op, q.Operation = "inventory.bulk-update", uuid.NewString()
	q.Bulk = &equipment.BulkUpdate{Items: []equipment.BulkItem{{ID: d.ID, Revision: d.Revision}}, Block: &block}
	equipmentCall(t, s, q, equipment.Unavailable)
	state, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(state.Devices) != 1 || state.Devices[0].Revision != d.Revision || state.Devices[0].Block != d.Block {
		t.Fatal("device changed despite snapshot failure", code, state.Devices)
	}
	var revision uint64
	err = j.st.unit.QueryRow(`SELECT revision FROM device_equipment_snapshot WHERE device_id=?`, d.ID).Scan(&revision)
	if err != nil || revision != d.Revision {
		t.Fatal("snapshot changed despite transaction failure", revision, err)
	}
}
