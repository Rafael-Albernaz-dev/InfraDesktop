package presentation

import (
	"reflect"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestInventoryFiltersBulkAndColumnsAcrossBindingIPC(t *testing.T) {
	r := newOperationalRig(t)
	r.enrollDevice()
	columns := []string{"equipment", "block", "cpu"}
	if got := r.app.SetInventoryColumns(columns); got.Code != "OK" {
		t.Fatal("set columns", got.Code)
	}
	if got := r.app.Status(); got.Code != "OK" || !reflect.DeepEqual(got.Columns, columns) {
		t.Fatal("account snapshot", got.Code, got.Columns)
	}
	block := "Norte"
	input := equipment.BulkUpdate{Items: []equipment.BulkItem{{ID: r.device.ID, Revision: 1}}, Block: &block}
	operation := uuid.NewString()
	got := r.app.BulkUpdateEquipment(r.profile, r.activation, operation, input)
	if got.Code != "OK" || got.Data.BulkReceipt == nil || got.Data.BulkReceipt.Items[0].Revision != 1 {
		t.Fatal("bulk", got.Code)
	}
	again := r.app.BulkUpdateEquipment(r.profile, r.activation, operation, input)
	if again.Code != "OK" || again.Data.BulkReceipt.Items[0].Revision != 1 {
		t.Fatal("replay", again.Code)
	}
	listed := r.app.ListEquipmentFiltered(r.profile, r.activation, InventoryFilters{Block: "Norte"})
	if listed.Code != "OK" || len(listed.Data.Devices) != 1 || listed.Data.Devices[0].Block != block {
		t.Fatal("filtered list", listed.Code)
	}
	if r.peer.Auth.Load() != 0 || r.peer.Handshakes.Load() != 0 {
		t.Fatal("inventory update initiated a connection")
	}
}
