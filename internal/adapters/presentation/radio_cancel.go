package presentation

import (
	"infradesktop/internal/modules/equipment"
)

func (a *App) CancelRadioOperation(p, act, device, operation string) RadioResult {
	return a.radioCall("radio.cancel", p, act, equipment.RadioInput{DeviceID: device, Operation: operation})
}
