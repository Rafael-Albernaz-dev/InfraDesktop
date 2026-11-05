package presentation

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

type InventoryFilters struct {
	Search     string `json:"search"`
	Cursor     int    `json:"cursor"`
	Inactive   bool   `json:"inactive"`
	Role       string `json:"role"`
	Block      string `json:"block"`
	BlockEmpty bool   `json:"block_empty"`
	Model      string `json:"model"`
}

func (a *App) ListEquipmentFiltered(profile, activation string, f InventoryFilters) EquipmentResult {
	return a.equipmentCall("inventory.list", profile, activation, ipc.EquipmentInput{
		Search: f.Search, Cursor: f.Cursor, IncludeInactive: f.Inactive, Role: equipment.Role(f.Role),
		Block: f.Block, BlockEmpty: f.BlockEmpty, Model: f.Model,
	})
}
