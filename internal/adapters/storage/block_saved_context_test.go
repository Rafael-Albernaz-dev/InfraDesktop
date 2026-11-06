package storage

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestBlockPreservesRadioReadingAndProfileRevision(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	d := target.Device
	d.Role = equipment.RoleAP
	if code := j.st.UpdateEquipment(ctx, companyX, func(st *equipment.State) equipment.Code {
		st.Devices[0].Role = d.Role
		return equipment.OK
	}); code != equipment.OK {
		t.Fatal(code)
	}
	saveSyntheticInventoryRadios(t, j, []equipment.Device{d}, target.Credential.Metadata.Revision)
	before, profile, code := j.st.RadioCurrent(ctx, companyX, d.ID)
	if code != equipment.OK || before == nil || profile == nil || profile.Revision != d.Revision {
		t.Fatal("fixture radio", code, profile)
	}
	q := equipmentScope(j)
	q.Op = "inventory.bulk-update"
	q.Operation = uuid.NewString()
	block := "Torre sintetica"
	q.Bulk = &equipment.BulkUpdate{Block: &block, Items: []equipment.BulkItem{{ID: d.ID, Revision: d.Revision}}}
	equipmentCall(t, newEquipment(t, j), q, equipment.OK)
	after, afterProfile, code := j.st.RadioCurrent(ctx, companyX, d.ID)
	if code != equipment.OK || after == nil || !after.Compatible || after.Revision != d.Revision || afterProfile.Revision != profile.Revision {
		t.Fatal("radio or profile lost", code, after, afterProfile)
	}
	a, _ := json.Marshal(profile)
	b, _ := json.Marshal(afterProfile)
	if string(a) != string(b) {
		t.Fatal("profile changed after block")
	}
}

func TestBlockPreservesPPPoEObservationRevision(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	before := ppSynthetic(t, j, target, 1)
	before.Provenance.Revision = target.Device.Revision
	_, err := j.st.unit.Exec(`UPDATE pppoe_observations SET meta_json=json_set(meta_json,'$.provenance.revision',?) WHERE id=?`,
		target.Device.Revision, before.ID)
	if err != nil {
		t.Fatal(err)
	}
	q := equipmentScope(j)
	q.Op = "inventory.bulk-update"
	q.Operation = uuid.NewString()
	block := "Torre sintetica"
	q.Bulk = &equipment.BulkUpdate{Block: &block, Items: []equipment.BulkItem{{ID: target.Device.ID, Revision: target.Device.Revision}}}
	equipmentCall(t, newEquipment(t, j), q, equipment.OK)
	current, code := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	if code != equipment.OK || current.Current == nil || current.Current.ID != before.ID ||
		current.Current.Provenance.Revision != before.Provenance.Revision {
		t.Fatal("saved PPPoE changed", code, current.Current)
	}
	saved, code := j.st.ReadPPTarget(ctx, companyX, target.Device.ID, false)
	if code != equipment.OK || saved.Device.Revision != before.Provenance.Revision {
		t.Fatal("saved PPPoE no longer matches device revision", code, saved.Device.Revision, before.Provenance.Revision)
	}
}
