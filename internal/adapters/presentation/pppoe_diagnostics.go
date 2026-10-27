package presentation

import "infradesktop/internal/modules/equipment"

func (a *App) StartPPPoECatalog(p, act, device, operation string, revision uint64) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-start", p, act, equipment.PPInput{DeviceID: device, Operation: operation, Revision: revision, Scope: equipment.PDCatalog})
}
func (a *App) StartPPPoERadius(p, act, device, operation string, revision uint64) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-start", p, act, equipment.PPInput{DeviceID: device, Operation: operation, Revision: revision, Scope: equipment.PDRadius})
}
func (a *App) StartPPPoEPing(p, act, device, observation, row, operation string, revision uint64) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-start", p, act, equipment.PPInput{DeviceID: device, Operation: operation, Revision: revision, Scope: equipment.PDPing, ObservationID: observation, RowID: row})
}
func (a *App) PPPoEDiagnosticStatus(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-status", p, act, equipment.PPInput{JobID: job})
}
func (a *App) CancelPPPoEDiagnostic(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-cancel", p, act, equipment.PPInput{JobID: job})
}
func (a *App) GetPPPoEDiagnostic(p, act, device, scope string) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-current", p, act, equipment.PPInput{DeviceID: device, Scope: scope})
}
func (a *App) ListPPPoEDiagnostic(p, act, device, scope, diagnostic, cursor, kind string) PPPoEResult {
	return a.ppCall("pppoe.diagnostic-list", p, act, equipment.PPInput{DeviceID: device, Scope: scope, DiagnosticID: diagnostic, Cursor: cursor, Query: &equipment.PPQuery{Type: kind}})
}
