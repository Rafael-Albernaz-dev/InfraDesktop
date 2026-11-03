package presentation

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

func (a *App) SetCredentialDefault(p, act, role, id, operation string) EquipmentResult {
	return a.equipmentCall("credential.default-set", p, act, ipc.EquipmentInput{
		ID: id, Operation: operation, Role: equipment.Role(role), CredentialID: id,
	})
}
