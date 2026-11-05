package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func TestBulkUpdateAtomicRevisionCompanyAndReplay(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	credential, first := setupEquipment(t, j, s, 2222)
	second := first
	second.ID = uuid.NewString()
	second.Operation = uuid.NewString()
	equipmentCall(t, s, second, equipment.OK)
	j.selectCompany(companyY, access.OK)
	_, foreign := setupEquipment(t, j, s, 2222)
	j.selectCompany(companyX, access.OK)
	q := equipmentScope(j)
	q.Op = "inventory.bulk-update"
	q.Operation = uuid.NewString()
	block := "Torre Norte"
	q.Bulk = &equipment.BulkUpdate{Items: []equipment.BulkItem{{ID: first.ID, Revision: 1}, {ID: second.ID, Revision: 9}}, Block: &block}
	equipmentCall(t, s, q, equipment.Revision)
	assertBulkUnchanged(t, j, first.ID, second.ID)
	q.Bulk.Items[1] = equipment.BulkItem{ID: foreign.ID, Revision: 1}
	equipmentCall(t, s, q, equipment.Revision)
	assertBulkUnchanged(t, j, first.ID, second.ID)
	q.Bulk.Items[1] = equipment.BulkItem{ID: second.ID, Revision: 1}
	missing := uuid.NewString()
	q.Bulk.Block = nil
	q.Bulk.CredentialID = &missing
	equipmentCall(t, s, q, equipment.MissingCredential)
	assertBulkUnchanged(t, j, first.ID, second.ID)
	q.Bulk.Block = &block
	q.Bulk.CredentialID = nil
	result := equipmentCall(t, s, q, equipment.OK)
	if result.BulkReceipt == nil || len(result.BulkReceipt.Items) != 2 || result.BulkReceipt.Items[0].Revision != 2 {
		t.Fatal("missing receipt", result.BulkReceipt)
	}
	replayed := equipmentCall(t, s, q, equipment.OK)
	if replayed.BulkReceipt == nil || replayed.BulkReceipt.Items[0].Revision != 2 {
		t.Fatal("invalid replay", replayed.BulkReceipt)
	}
	block = "different"
	equipmentCall(t, s, q, equipment.Revision)
	block = "Torre Norte"
	q.Operation = uuid.NewString()
	q.Bulk.Block = nil
	q.Bulk.CredentialID = &credential.ID
	q.Bulk.Items[0].Revision = 2
	q.Bulk.Items[1].Revision = 2
	equipmentCall(t, s, q, equipment.OK)
}

func assertBulkUnchanged(t *testing.T, j *journey, ids ...string) {
	t.Helper()
	st, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	for _, id := range ids {
		found := false
		for _, d := range st.Devices {
			if d.ID == id {
				found = true
				if d.Block != "" || d.Revision != 1 {
					t.Fatal("partial bulk write", d)
				}
			}
		}
		if !found {
			t.Fatal("fixture device missing")
		}
	}
}
