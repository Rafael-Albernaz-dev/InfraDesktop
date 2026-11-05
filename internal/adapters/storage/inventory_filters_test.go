package storage

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func TestInventoryFiltersCoverAllPagesAndKeepTotal(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	_, base := setupEquipment(t, j, s, 2222)
	if c := j.st.UpdateEquipment(context.Background(), companyX, func(st *equipment.State) equipment.Code {
		for i := 0; i < 21; i++ {
			st.Devices = append(st.Devices, equipment.Device{ID: uuid.NewString(), Name: fmt.Sprintf("Torre %02d", i),
				Address: "192.0.2.1", Port: 22, Transport: equipment.TransportSSH, CredentialID: base.CredentialID,
				Role: equipment.RoleAP, Active: true, Revision: 1, Block: "Norte",
				Result: &equipment.Observation{Board: "Modelo A"}})
		}
		st.Devices = append(st.Devices, equipment.Device{ID: uuid.NewString(), Name: "Torre inativa", Address: "192.0.2.2",
			Port: 22, Transport: equipment.TransportSSH, Role: equipment.RoleCPE, Revision: 1, Block: "Sul",
			Result: &equipment.Observation{Board: "Modelo B"}})
		st.Devices = append(st.Devices, equipment.Device{ID: uuid.NewString(), Name: "Torre CPE", Address: "192.0.2.3",
			Port: 22, Transport: equipment.TransportSSH, Role: equipment.RoleCPE, Revision: 1, Block: "Sul", Active: true,
			Result: &equipment.Observation{Board: "Modelo B"}})
		for _, d := range []equipment.Device{
			{Role: equipment.RoleCPE, Block: "Norte", Result: &equipment.Observation{Board: "Modelo A"}},
			{Role: equipment.RoleAP, Block: "Sul", Result: &equipment.Observation{Board: "Modelo A"}},
			{Role: equipment.RoleAP, Block: "Norte", Result: &equipment.Observation{Board: "Modelo B"}},
		} {
			d.ID, d.Name, d.Address = uuid.NewString(), "Torre extra", "192.0.2.4"
			d.Port, d.Transport, d.Active, d.Revision = 22, equipment.TransportSSH, true, 1
			st.Devices = append(st.Devices, d)
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	q := equipmentScope(j)
	q.Op = "inventory.list"
	q.Search = "torre"
	q.Role = equipment.RoleAP
	q.Block = "Norte"
	q.Model = "Modelo A"
	first := equipmentCall(t, s, q, equipment.OK)
	if len(first.Devices) != 16 || first.Next != 16 || first.Total != 27 {
		t.Fatalf("first page: %+v", first)
	}
	q.Cursor = first.Next
	second := equipmentCall(t, s, q, equipment.OK)
	if len(second.Devices) != 5 || second.Next != 0 || second.Total != 27 {
		t.Fatalf("second page: %+v", second)
	}
	if fmt.Sprint(first.Blocks) != "[Norte Sul]" || fmt.Sprint(first.Models) != "[Modelo A Modelo B]" {
		t.Fatalf("options %v %v", first.Blocks, first.Models)
	}
	verifyInactiveAndEmptyBlockFilters(t, s, q)
}

func verifyInactiveAndEmptyBlockFilters(t *testing.T, s *equipment.Service, q equipment.Command) {
	t.Helper()
	q.Cursor = 0
	q.Role = equipment.RoleCPE
	q.Block = "Sul"
	q.Model = "Modelo B"
	q.IncludeInactive = true
	if got := equipmentCall(t, s, q, equipment.OK); len(got.Devices) != 2 || got.Total != 27 {
		t.Fatalf("inactive filter: %+v", got)
	}
	q.Role, q.Block, q.Model, q.Search = "", "", "", ""
	q.BlockEmpty = true
	if got := equipmentCall(t, s, q, equipment.OK); len(got.Devices) != 1 || got.Devices[0].Block != "" {
		t.Fatal("empty-block filter did not select the device without a block")
	}
}
