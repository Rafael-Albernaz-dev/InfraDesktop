package storage

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func TestBulkUpdateRejectsAmbiguityAndRollsBackSQLFailure(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	credential, first := setupEquipment(t, j, s, 2222)
	second := first
	second.ID, second.Operation = uuid.NewString(), uuid.NewString()
	equipmentCall(t, s, second, equipment.OK)
	block := "Norte"
	q := equipmentScope(j)
	q.Op, q.Operation = "inventory.bulk-update", uuid.NewString()
	q.Bulk = &equipment.BulkUpdate{Items: []equipment.BulkItem{{ID: first.ID, Revision: 1}}, Block: &block, CredentialID: &credential.ID}
	equipmentCall(t, s, q, equipment.Invalid)
	q.Bulk.CredentialID = nil
	q.Bulk.Items = append(q.Bulk.Items, q.Bulk.Items[0])
	equipmentCall(t, s, q, equipment.Invalid)
	q.Bulk.Items[1] = equipment.BulkItem{ID: second.ID, Revision: 1}
	trigger := fmt.Sprintf(`CREATE TRIGGER reject_second_bulk BEFORE UPDATE ON devices
		WHEN NEW.id='%s' BEGIN SELECT RAISE(ABORT,'synthetic failure'); END`, second.ID)
	if _, err := j.st.unit.Exec(trigger); err != nil {
		t.Fatal(err)
	}
	equipmentCall(t, s, q, equipment.Unavailable)
	assertBulkUnchanged(t, j, first.ID, second.ID)
	if _, err := j.st.unit.Exec("DROP TRIGGER reject_second_bulk"); err != nil {
		t.Fatal(err)
	}
	equipmentCall(t, s, q, equipment.OK)
}
