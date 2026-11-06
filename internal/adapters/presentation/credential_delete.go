package presentation

import "infradesktop/internal/adapters/ipc"

func (a *App) DeleteCredential(profile, activation, id, operation string, revision uint64) EquipmentResult {
	return a.equipmentCall("credential.delete", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation, Revision: revision})
}
