package ipc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/adapters/importfile"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/jobprobe"
)

func TestImportIPCUsesScopedLargeFrameAndDoesNotReadEquipment(t *testing.T) {
	j := newPPJourney(t)
	if !jobprobe.SetParser(j.svc, importfile.Parser{}) {
		t.Fatal("fixture parser unavailable")
	}
	before := j.peer.Auth.Load()
	body := "Nome,IP,Porta,Papel,Nota\nRouter,192.0.2.1,22,AP," + strings.Repeat("x", 70000) + "\n"
	q := Request{Op: "inventory.import-preview", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation,
		Equipment: &EquipmentInput{Import: &equipment.ImportInput{Filename: "fixture.csv", Data: []byte(body)}}}
	r := j.client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data == nil || len(r.Data.Equipment.ImportItems) != 1 {
		t.Fatal("preview failed", r.Code)
	}
	row := r.Data.Equipment.ImportItems[0].Row
	q.Op = "inventory.import-confirm"
	q.Equipment = &EquipmentInput{Operation: uuid.NewString(), Import: &equipment.ImportInput{Rows: []equipment.ImportRow{row}}}
	r = j.client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data.Equipment.ImportReceipt.Created != 1 {
		t.Fatal("confirm failed", r.Code)
	}
	r = j.client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data.Equipment.ImportReceipt.Created != 1 {
		t.Fatal("replay failed", r.Code)
	}
	if j.peer.Auth.Load() != before {
		t.Fatal("import triggered equipment connection")
	}
}

func TestImportIPCRejectsPasswordWithoutEcho(t *testing.T) {
	j := newPPJourney(t)
	if !jobprobe.SetParser(j.svc, importfile.Parser{}) {
		t.Fatal("fixture parser unavailable")
	}
	q := Request{Op: "inventory.import-preview", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation,
		Equipment: &EquipmentInput{Import: &equipment.ImportInput{Filename: "fixture.csv", Data: []byte("Nome,senha\nR,synthetic-secret\n")}}}
	r := j.client.Call(context.Background(), q)
	if r.Code != foundation.Code(equipment.ImportSecret) || r.Data != nil {
		t.Fatal("password refusal missing", r.Code)
	}
}

func TestImportIPCPreviewAndConfirmFiveHundred(t *testing.T) {
	j := newPPJourney(t)
	if !jobprobe.SetParser(j.svc, importfile.Parser{}) {
		t.Fatal("fixture parser unavailable")
	}
	client := j.client
	selectEmptyImportCompany(t, client)
	var csv strings.Builder
	csv.WriteString("Nome,IP,Porta,Papel\n")
	for i := 0; i < 500; i++ {
		ip := fmt.Sprintf("192.0.2.%d", i)
		if i >= 250 {
			ip = fmt.Sprintf("198.51.100.%d", i-250)
		}
		fmt.Fprintf(&csv, "Roteador %d,%s,22,AP\n", i, ip)
	}
	q := Request{Op: "inventory.import-preview", ExpectedProfile: client.profile, ExpectedActivation: client.activation,
		Equipment: &EquipmentInput{Import: &equipment.ImportInput{Filename: "fixture.csv", Data: []byte(csv.String())}}}
	r := client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data == nil || len(r.Data.Equipment.ImportItems) != 500 {
		t.Fatal("preview", r.Code)
	}
	rows := make([]equipment.ImportRow, 0, 500)
	for _, item := range r.Data.Equipment.ImportItems {
		if item.Status != "ready" {
			t.Fatal("not ready", item.Status)
		}
		rows = append(rows, item.Row)
	}
	q.Op = "inventory.import-confirm"
	q.Equipment = &EquipmentInput{Operation: uuid.NewString(), Import: &equipment.ImportInput{Rows: rows}}
	r = client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data.Equipment.ImportReceipt.Created != 500 {
		t.Fatal("confirm", r.Code)
	}
}

func selectEmptyImportCompany(t *testing.T, client *AccountClient) {
	t.Helper()
	created := client.Call(context.Background(), Request{Op: "company.create", Name: "Synthetic second"})
	if created.Code != foundation.OK {
		t.Fatal("company create", created.Code)
	}
	bootstrap := client.Call(context.Background(), Request{Op: "access.bootstrap"})
	company := ""
	for _, c := range bootstrap.Data.Companies {
		if c.Name == "Synthetic second" {
			company = c.ID
		}
	}
	if company == "" {
		t.Fatal("fixture company missing")
	}
	if r := client.Call(context.Background(), Request{Op: "company.select", Name: company}); r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
}
