package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestIdenticalEditPreservesConfirmedRadioProfile(t *testing.T) {
	j, target := ppStoreSetup(t)
	saveConfirmedRadioForEdit(t, j, target)
	d := target.Device
	q := equipmentScope(j)
	q.Op, q.ID, q.Operation, q.Revision = "inventory.update", d.ID, uuid.NewString(), d.Revision
	q.Name, q.Role, q.Address, q.Port, q.Transport = d.Name, d.Role, d.Address, d.Port, d.Transport
	q.CredentialID, q.Block = d.CredentialID, d.Block
	updated := equipmentCall(t, newEquipment(t, j), q, equipment.OK)
	if updated.Device == nil || updated.Device.Revision != d.Revision+1 {
		t.Fatal("fixture did not exercise inherited revision")
	}
	reading, profile, code := j.st.RadioCurrent(context.Background(), companyX, d.ID)
	validReading := reading != nil && reading.Compatible
	validProfile := profile != nil && profile.Confirmed && profile.Revision == updated.Device.Revision
	if code != equipment.OK || !validReading || !validProfile {
		t.Fatal("identical edit lost confirmed radio profile")
	}
	if profile.CredentialRevision != target.Credential.Metadata.Revision || profile.Warning != 10 || profile.Critical != 20 {
		t.Fatal("identical edit changed saved settings")
	}
}
