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

func importFiveHundredForList(t *testing.T, client *AccountClient) {
	t.Helper()
	var body strings.Builder
	body.WriteString("Nome,IP,Porta,Papel\n")
	for i := 0; i < 500; i++ {
		network, n := "192.0.2", i+1
		if i >= 250 {
			network, n = "198.51.100", i-249
		}
		fmt.Fprintf(&body, "Roteador %03d,%s.%d,22,AP\n", i, network, n)
	}
	q := Request{Op: "inventory.import-preview", ExpectedProfile: client.profile, ExpectedActivation: client.activation,
		Equipment: &EquipmentInput{Import: &equipment.ImportInput{Filename: "fixture.csv", Data: []byte(body.String())}}}
	r := client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data == nil || r.Data.Equipment == nil || len(r.Data.Equipment.ImportItems) != 500 {
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

func TestIPCListsAndBulkUpdatesFiveHundredDevices(t *testing.T) {
	j := newPPJourney(t)
	if !jobprobe.SetParser(j.svc, importfile.Parser{}) {
		t.Fatal("fixture parser unavailable")
	}
	client := j.client
	selectEmptyImportCompany(t, client)
	before := j.peer.Auth.Load()
	importFiveHundredForList(t, client)
	items := listFiveHundredForBulk(t, client)
	block := "Torre sintetica"
	q := Request{Op: "inventory.bulk-update", ExpectedProfile: client.profile, ExpectedActivation: client.activation,
		Equipment: &EquipmentInput{Operation: uuid.NewString(), Bulk: &equipment.BulkUpdate{Items: items, Block: &block}}}
	r := client.Call(context.Background(), q)
	if r.Code != foundation.OK || r.Data == nil || r.Data.Equipment.BulkReceipt == nil || len(r.Data.Equipment.BulkReceipt.Items) != 500 {
		t.Fatal("bulk", r.Code)
	}
	for _, item := range r.Data.Equipment.BulkReceipt.Items {
		if item.Revision != 1 {
			t.Fatal("block changed revision", item.Revision)
		}
	}
	if j.peer.Auth.Load() != before {
		t.Fatal("listing or bulk initiated a connection")
	}
}

func listFiveHundredForBulk(t *testing.T, client *AccountClient) []equipment.BulkItem {
	t.Helper()
	items := make([]equipment.BulkItem, 0, 500)
	cursor := 0
	for {
		q := Request{Op: "inventory.list", ExpectedProfile: client.profile, ExpectedActivation: client.activation,
			Equipment: &EquipmentInput{Cursor: cursor}}
		r := client.Call(context.Background(), q)
		if r.Code != foundation.OK || r.Data == nil || r.Data.Equipment == nil {
			t.Fatal("list", r.Code)
		}
		for _, d := range r.Data.Equipment.Devices {
			items = append(items, equipment.BulkItem{ID: d.ID, Revision: d.Revision})
		}
		cursor = r.Data.Equipment.Next
		if cursor == 0 {
			break
		}
	}
	if len(items) != 500 {
		t.Fatalf("listed %d devices", len(items))
	}
	return items
}
