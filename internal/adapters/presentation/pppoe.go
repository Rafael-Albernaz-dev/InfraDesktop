package presentation

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

type PPPoEResult struct {
	Code       string            `json:"code"`
	Profile    string            `json:"profile_id"`
	Activation string            `json:"activation_id"`
	Data       *equipment.PPView `json:"data,omitempty"`
	Error      *ipc.ErrorDetail  `json:"error,omitempty"`
}

func (a *App) ppCall(op, profile, activation string, in equipment.PPInput) PPPoEResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ppCallLocked(op, profile, activation, in)
}
func (a *App) ppCallLocked(op, profile, activation string, in equipment.PPInput) PPPoEResult {
	r := a.client.Call(a.ctx, ipc.Request{Op: op, ExpectedProfile: profile, ExpectedActivation: activation, PPPoE: &in})
	v := PPPoEResult{Code: string(r.Code), Profile: profile, Activation: activation, Error: r.ErrorDetail}
	if r.Data != nil {
		v.Data = r.Data.PPPoE
	}
	return v
}
func (a *App) StartPPPoECollection(p, act, device, operation string, revision uint64) PPPoEResult {
	return a.ppCall("pppoe.collect-start", p, act, equipment.PPInput{DeviceID: device, Operation: operation, Revision: revision})
}
func (a *App) PPPoECollectionStatus(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.collect-status", p, act, equipment.PPInput{JobID: job})
}
func (a *App) CancelPPPoECollection(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.collect-cancel", p, act, equipment.PPInput{JobID: job})
}
func (a *App) GetPPPoECurrent(p, act, device string) PPPoEResult {
	return a.ppCall("pppoe.current", p, act, equipment.PPInput{DeviceID: device})
}
func (a *App) GetPPPoEObservation(p, act, device, observation string) PPPoEResult {
	return a.ppCall("pppoe.observation-get", p, act, equipment.PPInput{DeviceID: device, ObservationID: observation})
}
func (a *App) ListPPPoESessions(p, act, device, observation string, query equipment.PPQuery, cursor string) PPPoEResult {
	return a.ppCall("pppoe.sessions-list", p, act, equipment.PPInput{DeviceID: device, ObservationID: observation, Query: &query, Cursor: cursor})
}
func (a *App) GetPPPoESession(p, act, device, observation, row string) PPPoEResult {
	return a.ppCall("pppoe.session-get", p, act, equipment.PPInput{DeviceID: device, ObservationID: observation, RowID: row})
}
func (a *App) SavePPPoEReference(p, act, device, observation, reference, operation string) PPPoEResult {
	return a.ppCall("pppoe.reference-save", p, act, equipment.PPInput{DeviceID: device, ObservationID: observation, ReferenceID: reference, Operation: operation})
}
func (a *App) ListPPPoEReferences(p, act, device, cursor string) PPPoEResult {
	return a.ppCall("pppoe.reference-list", p, act, equipment.PPInput{DeviceID: device, Cursor: cursor})
}
func (a *App) DeletePPPoEReference(p, act, device, reference, operation string) PPPoEResult {
	return a.ppCall("pppoe.reference-delete", p, act, equipment.PPInput{DeviceID: device, ReferenceID: reference, Operation: operation})
}
func (a *App) ComparePPPoE(p, act, device, base, target string, query equipment.PPQuery, cursor string) PPPoEResult {
	return a.ppCall("pppoe.compare", p, act, equipment.PPInput{DeviceID: device, BaseID: base, TargetID: target, Query: &query, Cursor: cursor})
}

func (a *App) PPPoEContextCounts(p, act, device, observation, cursor string) PPPoEResult {
	return a.ppCall("pppoe.context-counts", p, act, equipment.PPInput{DeviceID: device, ObservationID: observation, Cursor: cursor})
}

func (a *App) StartPPPoERead(p, act, device, operation, scope, observation string, rows []string, revision uint64) PPPoEResult {
	return a.ppCall("pppoe.read-start", p, act, equipment.PPInput{DeviceID: device, Operation: operation, Scope: scope, ObservationID: observation, RowIDs: rows, Revision: revision})
}
func (a *App) PPPoEReadStatus(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.read-status", p, act, equipment.PPInput{JobID: job})
}
func (a *App) CancelPPPoERead(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.read-cancel", p, act, equipment.PPInput{JobID: job})
}
func (a *App) PPPoEReadPage(p, act, job, cursor string) PPPoEResult {
	return a.ppCall("pppoe.read-page", p, act, equipment.PPInput{JobID: job, Cursor: cursor})
}

func (a *App) CancelPPPoEReadOperation(p, act, device, operation string) PPPoEResult {
	// App's ppCall mutex serializes this behind an uncertain start call.
	return a.ppCall("pppoe.read-cancel", p, act, equipment.PPInput{DeviceID: device, Operation: operation})
}

func (a *App) GetDeviceMonitoring(p, act, device, scope, cursor string) PPPoEResult {
	return a.ppCall("pppoe.monitor-current", p, act, equipment.PPInput{DeviceID: device, Scope: scope, Cursor: cursor})
}

func (a *App) StopPPPoETraffic(p, act, job string) PPPoEResult {
	return a.ppCall("pppoe.read-stop", p, act, equipment.PPInput{JobID: job})
}
