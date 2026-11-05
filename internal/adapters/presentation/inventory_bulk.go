package presentation

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

func (a *App) BulkUpdateEquipment(profile, activation, operation string, input equipment.BulkUpdate) EquipmentResult {
	return a.equipmentCall("inventory.bulk-update", profile, activation, ipc.EquipmentInput{Operation: operation, Bulk: &input})
}
