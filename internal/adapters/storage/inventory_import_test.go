package storage

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/adapters/importfile"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
)

func importFixture(t *testing.T) (*journey, *inventory.Service) {
	t.Helper()
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	key := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	s := inventory.New(j.svc, j.st, key, routerssh.Adapter{}, equipmentIDs{}, time.Now)
	s.Parser, s.Digester = importfile.Parser{}, importfile.Parser{}
	t.Cleanup(s.Close)
	return j, s
}

func importCall(t *testing.T, s *inventory.Service, q equipment.Command, want equipment.Code) equipment.View {
	t.Helper()
	v, code := s.Handle(context.Background(), q)
	if code != want {
		t.Fatalf("%s got %s want %s", q.Op, code, want)
	}
	return v
}

func TestImportPreviewConfirmReplayAndConflicts(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op = "inventory.import-preview"
	q.Import = &equipment.ImportInput{Filename: "fixture.csv", Data: []byte("Nome,IP,Porta,Papel\nRouter,192.0.2.1,22,Concentrador\n")}
	view := importCall(t, s, q, equipment.OK)
	if len(view.ImportItems) != 1 || view.ImportItems[0].Status != "ready" {
		t.Fatal(view.ImportItems)
	}
	row := view.ImportItems[0].Row
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{row}}
	first := importCall(t, s, q, equipment.OK)
	if first.ImportReceipt == nil || first.ImportReceipt.Created != 1 {
		t.Fatal(first.ImportReceipt)
	}
	replay := importCall(t, s, q, equipment.OK)
	if replay.ImportReceipt == nil || replay.ImportReceipt.Created != 1 {
		t.Fatal(replay.ImportReceipt)
	}
	q.Import.Rows[0].Name = "Alterado"
	importCall(t, s, q, equipment.Revision)
	q.Import.Rows[0].Name = row.Name
	q.Operation = uuid.NewString()
	importCall(t, s, q, equipment.Revision)
	state, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(state.Devices) != 1 || len(state.Operations) != 1 {
		t.Fatal(code, len(state.Devices), len(state.Operations))
	}
}

func TestImportRejectsWholeBatchAfterConflict(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	row := equipment.ImportRow{Line: 2, Name: "Router", Address: "192.0.2.1", Port: "22", Role: "AP"}
	other := row
	other.Line = 3
	q.Import = &equipment.ImportInput{Rows: []equipment.ImportRow{row, other}}
	importCall(t, s, q, equipment.Revision)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 0 || len(st.Operations) != 0 {
		t.Fatal("partial write", code)
	}
}

func TestImportPersistsFiveHundredWithinBudget(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	q.Import = &equipment.ImportInput{}
	for i := 0; i < 500; i++ {
		address := fmt.Sprintf("192.0.2.%d", i)
		if i >= 250 {
			address = fmt.Sprintf("198.51.100.%d", i-250)
		}
		row := equipment.ImportRow{Line: i + 1, Name: fmt.Sprintf("Router %d", i), Address: address, Port: "22", Role: "AP"}
		q.Import.Rows = append(q.Import.Rows, row)
	}
	started := time.Now()
	got := importCall(t, s, q, equipment.OK)
	elapsed := time.Since(started)
	t.Logf("500 registros: %s; orçamento: %s", elapsed, equipment.PPPersistenceBudget)
	if got.ImportReceipt == nil || got.ImportReceipt.Created != 500 || elapsed >= equipment.PPPersistenceBudget {
		t.Fatal("budget or receipt", elapsed, got.ImportReceipt)
	}
	state, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(state.Devices) != 500 {
		t.Fatal(code, len(state.Devices))
	}
}

func TestImportConfirmRejectsMoreThanFiveHundred(t *testing.T) {
	j, s := importFixture(t)
	q := equipmentScope(j)
	q.Op, q.Operation = "inventory.import-confirm", uuid.NewString()
	row := equipment.ImportRow{Line: 1, Name: "A", Address: "192.0.2.1", Port: "22", Role: "AP"}
	q.Import = &equipment.ImportInput{Rows: make([]equipment.ImportRow, 501)}
	for i := range q.Import.Rows {
		q.Import.Rows[i] = row
	}
	importCall(t, s, q, equipment.Invalid)
	st, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || len(st.Devices) != 0 {
		t.Fatal("excess rows wrote devices", code)
	}
}
