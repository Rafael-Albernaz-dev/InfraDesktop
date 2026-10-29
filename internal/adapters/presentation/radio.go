package presentation

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

type RadioResult struct {
	Code       string               `json:"code"`
	Profile    string               `json:"profile_id"`
	Activation string               `json:"activation_id"`
	Data       *equipment.RadioView `json:"data,omitempty"`
}

func (a *App) radioCall(op, p, act string, in equipment.RadioInput) RadioResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.client.Call(a.ctx, ipc.Request{Op: op, ExpectedProfile: p, ExpectedActivation: act, Radio: &in})
	v := RadioResult{Code: string(r.Code), Profile: p, Activation: act}
	if r.Data != nil {
		v.Data = r.Data.Radio
	}
	return v
}
func (a *App) GetRadio(p, act, device, reading string, offset int) RadioResult {
	return a.radioCall("radio.current", p, act, equipment.RadioInput{DeviceID: device, ReadingID: reading, Offset: offset})
}
func (a *App) StartRadio(p, act string, in equipment.RadioInput) RadioResult {
	return a.radioCall("radio.start", p, act, in)
}
func (a *App) RadioStatus(p, act, job, reading string, offset int) RadioResult {
	return a.radioCall("radio.status", p, act, equipment.RadioInput{JobID: job, ReadingID: reading, Offset: offset})
}
func (a *App) CancelRadio(p, act, job string) RadioResult {
	return a.radioCall("radio.cancel", p, act, equipment.RadioInput{JobID: job})
}
func (a *App) CancelRadioOperation(p, act, device, operation string) RadioResult {
	return a.radioCall("radio.cancel", p, act, equipment.RadioInput{DeviceID: device, Operation: operation})
}
func (a *App) ConfirmRadioFamily(p, act, device, family string, revision uint64, warning, critical int) RadioResult {
	return a.radioCall("radio.confirm-family", p, act, equipment.RadioInput{DeviceID: device, Revision: revision, Family: family, Warning: warning, Critical: critical})
}
func (a *App) GetRadioLink(p, act, device, observation, row string) RadioResult {
	return a.radioCall("radio.link-current", p, act, equipment.RadioInput{DeviceID: device, ObservationID: observation, RowID: row})
}
func (a *App) ConfirmRadioLink(p, act, device, observation, row, operation string, link equipment.RadioLinkChoice) RadioResult {
	return a.radioCall("radio.link-confirm", p, act, equipment.RadioInput{DeviceID: device, ObservationID: observation, RowID: row, Operation: operation, Link: &link})
}
