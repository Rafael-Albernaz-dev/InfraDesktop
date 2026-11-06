package presentation

import (
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestCredentialDeleteThroughPresentation(t *testing.T) {
	rig := newOperationalRig(t)
	app := rig.app
	if app.Register("technical", "synthetic account password").Code != "OK" || app.AckRecovery().Code != "OK" ||
		app.Login("technical", "synthetic account password", false).Code != "OK" {
		t.Fatal("account fixture")
	}
	created := app.CreateCompany("Synthetic")
	selected := app.SelectCompany(created.Companies[0].ID)
	credential := CredentialInput{ID: uuid.NewString(), Operation: uuid.NewString(),
		Alias: "Fixture", User: "reader", Secret: "synthetic credential"}
	if r := app.PutCredential(selected.Profile, selected.Activation, credential); r.Code != "OK" {
		t.Fatal("create", r.Code)
	}
	device := DeviceInput{ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Synthetic",
		Address: "192.0.2.1", Port: 22, Transport: string(equipment.TransportSSH), CredentialID: credential.ID}
	if r := app.SaveEquipment(selected.Profile, selected.Activation, device); r.Code != "OK" {
		t.Fatal("device", r.Code)
	}
	if r := app.DeleteCredential(selected.Profile, selected.Activation, credential.ID, uuid.NewString(), 1); r.Code != "CREDENTIAL_IN_USE" {
		t.Fatal("in-use code lost across IPC", r.Code)
	}
	another := credential
	another.ID, another.Operation = uuid.NewString(), uuid.NewString()
	if r := app.PutCredential(selected.Profile, selected.Activation, another); r.Code != "OK" {
		t.Fatal("create second", r.Code)
	}
	op := uuid.NewString()
	for i := 0; i < 2; i++ {
		if r := app.DeleteCredential(selected.Profile, selected.Activation, another.ID, op, 1); r.Code != "OK" {
			t.Fatal("delete or replay", r.Code)
		}
	}
	list := app.ListCredentials(selected.Profile, selected.Activation, 0)
	if list.Code != "OK" || len(list.Data.Credentials) != 1 || list.Data.Credentials[0].ID != credential.ID {
		t.Fatal("unexpected catalog", list.Code)
	}
}
