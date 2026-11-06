package storage

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestImportIgnoresInvalidAndDuplicateRowsAtomically(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op = "inventory.import-preview"
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{
		{Line: 1, Name: "Fixture", Address: "192.0.2.31", Role: "AP"},
		{Line: 2, Name: "Ignored duplicate", Address: "192.0.2.31", Role: "AP", Ignored: true},
		{Line: 3, Name: "", Address: "invalid", Role: "unknown", Ignored: true},
	}}
	items := importCall(t, s, q, equipment.OK).ImportItems
	if len(items) != 3 || items[0].Status != "ready" || items[1].Status != "ignored" || items[2].Status != "ignored" {
		t.Fatal("ignored rows interfered with review", items)
	}
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	got := importCall(t, s, q, equipment.OK)
	if got.ImportReceipt == nil || got.ImportReceipt.Created != 1 || got.ImportReceipt.Ignored != 2 {
		t.Fatal("wrong receipt", got.ImportReceipt)
	}
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 1 || st.Devices[0].Address != "192.0.2.31" {
		t.Fatal("wrong committed rows", code, st.Devices)
	}
}

func TestImportCredentialNameIsCaseInsensitive(t *testing.T) {
	j, s := importFixture(t)
	id := addImportCredential(t, j, "Leitura sintética")
	q := equipmentScope(j)
	q.Op = "inventory.import-preview"
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{{
		Line: 1, Name: "Fixture", Address: "192.0.2.31", Role: "AP", Credential: "LEITURA SINTÉTICA",
	}}}
	item := importCall(t, s, q, equipment.OK).ImportItems[0]
	if item.Status != "ready" || item.CredentialID != id {
		t.Fatal("credential was not resolved", item.Status)
	}
}

func TestImportCanonicalDuplicateAndInvalidLine(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op = "inventory.import-preview"
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{
		{Line: 1, Name: "A", Address: "2001:DB8:0:0:0:0:0:1", Role: "AP"},
		{Line: 2, Name: "B", Address: "2001:db8::1", Role: "AP"},
		{Line: 0, Name: "C", Address: "192.0.2.31", Role: "AP"},
	}}
	items := importCall(t, s, q, equipment.OK).ImportItems
	if len(items) != 3 || items[0].Status != "conflict" || items[1].Status != "conflict" || items[2].Status != "pending" {
		t.Fatal("unexpected review", items)
	}
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	importCall(t, s, q, equipment.Revision)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 0 {
		t.Fatal("invalid batch wrote data", code)
	}
}

func TestImportAllIgnoredDoesNotCreateReceipt(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{{Line: 1, Ignored: true}}}
	importCall(t, s, q, equipment.Invalid)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 0 || len(st.Operations) != 0 {
		t.Fatal("empty import persisted", code)
	}
}
