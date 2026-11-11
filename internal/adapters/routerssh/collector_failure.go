package routerssh

import (
	"infradesktop/internal/modules/equipment"
)

func collectorFailure(code equipment.Code, reason string) (*equipment.PPReadResult, equipment.Code) {
	if equipment.ValidCollectorReason(code, reason) {
		return &equipment.PPReadResult{FailureReason: reason}, code
	}
	return nil, code
}
