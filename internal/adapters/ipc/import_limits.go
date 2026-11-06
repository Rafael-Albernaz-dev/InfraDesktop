package ipc

import "infradesktop/internal/modules/access"

// Import is bounded separately because the selected 2 MiB file is base64 on the wire.
func isImport(op string) bool {
	return op == "inventory.import-preview" || op == "inventory.import-confirm"
}

func equipmentFrameLimit(op string) int {
	if isImport(op) {
		return MaxBrandFrame
	}
	return MaxFrame
}

func equipmentResponseLimit(op string) int {
	if op == "inventory.import-preview" {
		return MaxBrandFrame - 4096
	}
	return 48 * 1024
}

func validEquipmentEnvelope(q Request, fields, size int) bool {
	return fields == 11 && size <= equipmentFrameLimit(q.Op) && q.Equipment != nil &&
		access.ValidCompanyID(q.ExpectedProfile) && validID(q.ExpectedActivation)
}
