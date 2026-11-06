package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func addImportCredential(t *testing.T, j *journey, alias string) string {
	t.Helper()
	s := newEquipment(t, j)
	q := equipmentScope(j)
	q.Op, q.ID, q.Operation = "credential.put", uuid.NewString(), uuid.NewString()
	q.Alias, q.User, q.Secret = alias, "fixture", "synthetic-only"
	equipmentCall(t, s, q, equipment.OK)
	return q.ID
}

func TestImportReviewCredentialAmbiguityAndStatuses(t *testing.T) {
	j, s := importFixture(t)
	addImportCredential(t, j, "repetida")
	addImportCredential(t, j, "repetida")
	known := addImportCredential(t, j, "unica")
	q := equipmentScope(j)
	q.Op = "inventory.import-preview"
	q.Import = &equipment.ImportInput{Filename: "fixture.json", Data: []byte(`[
		{"name":"A","address":"192.0.2.1","port":22,"role":"AP","credential":"repetida"},
		{"name":"B","address":"192.0.2.2","port":22,"role":"AP","credential":"ausente"},
		{"name":"C","address":"192.0.2.3","port":22,"role":"AP","credential":"unica"},
		{"name":"D","address":"192.0.2.4","port":22,"role":"AP"},
		{"name":"E","address":"192.0.2.4","port":22,"role":"AP"},
		{"name":"F","address":"192.0.2.5","port":22,"role":"desconhecido"}
	]`)}
	got := importCall(t, s, q, equipment.OK).ImportItems
	want := []string{"pending", "pending", "ready", "conflict", "conflict", "pending"}
	if len(got) != len(want) {
		t.Fatal("wrong number of rows")
	}
	for i := range want {
		if got[i].Status != want[i] {
			t.Fatal(i, got[i].Status)
		}
	}
	if got[0].Reason != "Nome de credencial repetido" || got[1].Reason != "Credencial não encontrada" || got[2].CredentialID != known {
		t.Fatal("credential review differs", got[0].Reason, got[1].Reason)
	}
}

func TestImportDoesNotCrossCompanyActivation(t *testing.T) {
	j, s := importFixture(t)
	j.create(companyY, "Y", access.OK)
	old := equipmentScope(j)
	j.selectCompany(companyY, access.OK)
	old.Op, old.Operation = "inventory.import-confirm", uuid.NewString()
	old.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{{Line: 1, Name: "A", Address: "192.0.2.1", Port: "22", Role: "AP"}}}
	_, code := s.Handle(context.Background(), old)
	if code != equipment.Changed {
		t.Fatal("old scope accepted", code)
	}
	current := equipmentScope(j)
	current.Op, current.Operation, current.Import = old.Op, uuid.NewString(), old.Import
	importCall(t, s, current, equipment.OK)
	st, code := j.st.ReadEquipment(context.Background(), companyY)
	if code != equipment.OK || len(st.Devices) != 1 {
		t.Fatal("wrong target company", code)
	}
	j.selectCompany(companyX, access.OK)
	st, code = j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 0 {
		t.Fatal("foreign write", code)
	}
}

func TestImportDatabaseFailureRollsBackWholeBatch(t *testing.T) {
	j, s := importFixture(t)
	_, err := j.st.unit.Exec(`CREATE TRIGGER fail_import_fixture BEFORE INSERT ON devices
		WHEN NEW.name='Falha sintetica' BEGIN SELECT RAISE(ABORT, 'fixture'); END`)
	if err != nil {
		t.Fatal(err)
	}
	q := equipmentScope(j)
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{
		{Line: 1, Name: "Bom", Address: "192.0.2.1", Port: "22", Role: "AP"},
		{Line: 2, Name: "Falha sintetica", Address: "192.0.2.2", Port: "22", Role: "AP"},
	}}
	importCall(t, s, q, equipment.Unavailable)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 0 || len(st.Operations) != 0 {
		t.Fatal("partial write", code)
	}
}

func TestImportDefaultPortAndCorrectedRowsRevalidated(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op = "inventory.import-preview"
	row := equipment.ImportRow{Line: 1, Name: "A", Address: "bad", Role: "AP"}
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{row}}
	if got := importCall(t, s, q, equipment.OK).ImportItems[0]; got.Status != "pending" {
		t.Fatal(got)
	}
	q.Import.Rows[0].Address = "192.0.2.1"
	got := importCall(t, s, q, equipment.OK).ImportItems[0]
	if got.Status != "ready" {
		t.Fatal(got)
	}
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	importCall(t, s, q, equipment.OK)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 1 || st.Devices[0].Port != 22 {
		t.Fatal("default port", code, st.Devices)
	}
}

func TestImportUsesRoleDefaultCredentialWhenBlank(t *testing.T) {
	j, s := importFixture(t)
	credential := addImportCredential(t, j, "unica")
	q := equipmentScope(j)
	q.Op, q.Operation, q.ID = "credential.default-set", uuid.NewString(), uuid.NewString()
	q.CredentialID, q.Role = credential, equipment.RoleAP
	importCall(t, s, q, equipment.OK)
	q = equipmentScope(j)
	q.Op = "inventory.import-preview"
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{{Line: 1, Name: "A", Address: "192.0.2.1", Role: "AP"}}}
	if got := importCall(t, s, q, equipment.OK).ImportItems[0]; got.Status != "ready" || got.CredentialID != credential {
		t.Fatal(got)
	}
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	importCall(t, s, q, equipment.OK)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 1 || st.Devices[0].CredentialID != credential {
		t.Fatal(code)
	}
}
