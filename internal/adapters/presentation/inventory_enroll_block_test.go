package presentation

import (
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestEnrollWithCredentialAndBlockThroughBinding(t *testing.T) {
	r := newOperationalRig(t)
	const password = "synthetic account password"
	if r.app.Register("technical", password).Code != "OK" || r.app.AckRecovery().Code != "OK" ||
		r.app.Login("technical", password, false).Code != "OK" {
		t.Fatal("synthetic account")
	}
	created := r.app.CreateCompany("Synthetic")
	selected := r.app.SelectCompany(created.Companies[0].ID)
	input := DeviceInput{ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Synthetic AP", Block: "Norte",
		Role: string(equipment.RoleAP), Address: "192.0.2.1", Port: 22, Transport: string(equipment.TransportSSH),
		User: "reader", Secret: "synthetic password"}
	result := r.app.SaveEquipment(selected.Profile, selected.Activation, input)
	if result.Code != "OK" || result.Data == nil || result.Data.Device == nil || result.Data.Device.Block != "Norte" {
		t.Fatal("atomic enrollment lost block", result.Code)
	}
	got := r.app.GetEquipment(selected.Profile, selected.Activation, input.ID)
	if got.Code != "OK" || got.Data.Device.Block != "Norte" || got.Data.Device.CredentialID == "" {
		t.Fatal("enrolled block or credential missing", got.Code)
	}
}
