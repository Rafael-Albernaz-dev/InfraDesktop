package presentation

import (
	"infradesktop/internal/modules/equipment"
)

func (a *App) GetRadioIndexPage(p, act, device, observation, row string, offset int) RadioResult {
	return a.radioCall("radio.index-current", p, act, equipment.RadioInput{
		DeviceID: device, ObservationID: observation, RowID: row, Offset: offset,
	})
}
