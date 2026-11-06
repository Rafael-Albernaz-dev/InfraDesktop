package storage

import (
	"bytes"
	"context"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func TestCredentialKeepSecretAndDeleteTransaction(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	q := equipmentScope(j)
	q.Op, q.ID, q.Operation = "credential.put", uuid.NewString(), uuid.NewString()
	q.Alias, q.User, q.Secret = "Fixture", "reader", "synthetic secret"
	equipmentCall(t, s, q, equipment.OK)
	edit := q
	edit.Operation, edit.Revision, edit.Secret, edit.Alias = uuid.NewString(), 1, "", "Edited"
	v := equipmentCall(t, s, edit, equipment.OK)
	if v.Credential == nil || v.Credential.Revision != 1 {
		t.Fatal("alias-only rename did not retain revision")
	}
	st, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	safeVault := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	secret, code := safeVault.Open(st.Installation, st.Company, q.ID, 1, st.Credentials[0].Envelope, st.WrappedKey)
	if code != equipment.OK || secret != q.Secret {
		t.Fatal("saved secret not preserved")
	}
	fresh := q
	fresh.ID, fresh.Operation, fresh.Secret = uuid.NewString(), uuid.NewString(), ""
	equipmentCall(t, s, fresh, equipment.InvalidCredentialSecret)
	del := equipmentScope(j)
	del.Op, del.ID, del.Operation, del.Revision = "credential.delete", q.ID, uuid.NewString(), 1
	stale := del
	stale.Operation, stale.Revision = uuid.NewString(), 3
	equipmentCall(t, s, stale, equipment.Revision)
	assertCredentialDeleteRollback(t, j, s, del, len(st.Operations))
	equipmentCall(t, s, del, equipment.OK)
	equipmentCall(t, s, del, equipment.OK)
	after, _ := j.st.ReadEquipment(context.Background(), companyX)
	if len(after.Credentials) != 0 || len(after.Operations) != len(st.Operations)+1 {
		t.Fatal("delete or replay failed")
	}
	var count int
	if err := j.st.unit.QueryRow("SELECT count(*) FROM credentials WHERE id=?", q.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("secret row remains", err)
	}
	changed := del
	changed.Revision++
	equipmentCall(t, s, changed, equipment.Revision)
}

func assertCredentialDeleteRollback(t *testing.T, j *journey, s *equipment.Service, del equipment.Command, operations int) {
	t.Helper()
	for _, item := range []struct{ name, event, table string }{
		{"reject_delete", "DELETE", "credentials"},
		{"reject_receipt", "INSERT", "equipment_operations"},
	} {
		trigger := "CREATE TRIGGER " + item.name + " BEFORE " + item.event + " ON " + item.table +
			" BEGIN SELECT RAISE(ABORT,'test'); END"
		if _, err := j.st.unit.Exec(trigger); err != nil {
			t.Fatal(err)
		}
		equipmentCall(t, s, del, equipment.Unavailable)
		after, _ := j.st.ReadEquipment(context.Background(), companyX)
		if len(after.Credentials) != 1 || len(after.Operations) != operations {
			t.Fatal("failed transaction changed secret or receipt", item.name)
		}
		if _, err := j.st.unit.Exec("DROP TRIGGER " + item.name); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCredentialDeleteRejectsLinksAndDefaults(t *testing.T) {
	for _, mode := range []string{"active", "inactive", "default"} {
		t.Run(mode, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			q, device := setupEquipment(t, j, s, 22)
			if mode == "inactive" {
				device.Op, device.Operation, device.Revision, device.Active = "inventory.set-active", uuid.NewString(), 1, false
				equipmentCall(t, s, device, equipment.OK)
			}
			if mode == "default" {
				// An independent credential has no device links, but is a role default.
				q.ID, q.Operation = uuid.NewString(), uuid.NewString()
				equipmentCall(t, s, q, equipment.OK)
				def := equipmentScope(j)
				def.Op, def.ID, def.CredentialID, def.Operation, def.Role = "credential.default-set", q.ID, q.ID, uuid.NewString(), equipment.RoleAP
				equipmentCall(t, s, def, equipment.OK)
			}
			del := equipmentScope(j)
			del.Op, del.ID, del.Operation, del.Revision = "credential.delete", q.ID, uuid.NewString(), 1
			equipmentCall(t, s, del, equipment.Code("CREDENTIAL_IN_USE"))
			st, _ := j.st.ReadEquipment(context.Background(), companyX)
			found := false
			for _, item := range st.Credentials {
				found = found || item.Metadata.ID == q.ID
			}
			if !found {
				t.Fatal("credential removed despite reference")
			}
		})
	}
}
