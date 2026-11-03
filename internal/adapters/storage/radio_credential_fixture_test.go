package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
)

func prepareDefaultAPForTOFU(t *testing.T, j *journey, s *equipment.Service, port int) (equipment.Command, equipment.Device) {
	t.Helper()
	cr, d := setupEquipment(t, j, s, port)
	def := equipmentScope(j)
	def.Op = "credential.default-set"
	def.ID = cr.ID
	def.CredentialID = cr.ID
	def.Operation = uuid.NewString()
	def.Role = equipment.RoleAP
	equipmentCall(t, s, def, equipment.OK)
	d.ID = uuid.NewString()
	d.Operation = uuid.NewString()
	d.Role = equipment.RoleAP
	d.CredentialID = ""
	equipmentCall(t, s, d, equipment.OK)
	st, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	var ap equipment.Device
	for _, v := range st.Devices {
		if v.ID == d.ID {
			ap = v
		}
	}
	if ap.CredentialID != cr.ID {
		t.Fatal("class default not applied", ap.CredentialID)
	}
	return d, ap
}
func refuseAssociatedRadioCredential(t *testing.T, j *journey, s *equipment.Service, peer *sshfixture.Server,
	chosen, d equipment.Command) equipment.Command {
	t.Helper()
	// A second credential exists; the first explicit association is deliberately refused.
	other := chosen
	other.ID = uuid.NewString()
	other.Operation = uuid.NewString()
	equipmentCall(t, s, other, equipment.OK)
	chosen.Operation = uuid.NewString()
	chosen.Revision = 1
	chosen.Affected = 1
	chosen.Secret = "synthetic-refused"
	equipmentCall(t, s, chosen, equipment.OK)
	if c := j.st.UpdateEquipment(context.Background(), companyX, func(st *equipment.State) equipment.Code {
		for i := range st.Devices {
			if st.Devices[i].ID == d.ID {
				st.Devices[i].Fingerprint = peer.Fingerprint()
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	return other
}
