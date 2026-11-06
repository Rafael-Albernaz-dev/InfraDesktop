package presentation

import "infradesktop/internal/adapters/ipc"

func accountResult(r ipc.Response) Result {
	v := Result{Code: string(r.Code), RequestID: r.RequestID, Companies: []ipc.Company{}}
	if d := r.Data; d != nil {
		v.Instance, v.Activation, v.Profile, v.Heartbeat = d.Instance, d.Activation, d.Profile, d.Heartbeat
		v.Setup, v.Authenticated, v.Name, v.Theme, v.Columns = d.Setup, d.Authenticated, d.Name, d.Theme, d.Columns
		v.Recovery, v.Companies, v.Brand, v.ProductVersion = d.Recovery, d.Companies, d.Brand, d.ProductVersion
	}
	return v
}

func (a *App) SetInventoryColumns(columns []string) Result {
	return a.call(ipc.Request{Op: "preferences.columns", Columns: columns})
}

func (a *App) updateRecoveryAfterCall(q ipc.Request, r ipc.Response) {
	if r.Code == "UNAUTHENTICATED" || r.Code == "PROFILE_CHANGED" {
		a.ppGeneration++
	}
	if a.shouldClearRecovery(r) {
		a.recovery = ""
		a.recoveryGeneration++
	}
	a.updateEmittedRecovery(q, r)
	a.updateRecoveryAcknowledgement(q, r)
}

func (a *App) shouldClearRecovery(r ipc.Response) bool {
	if a.accessRecoveryPending {
		return false
	}
	if r.Code == "UNAUTHENTICATED" || r.Code == "PROFILE_CHANGED" {
		return true
	}
	return r.Data != nil && a.recovery != "" && (r.Data.Instance != a.recoveryInstance ||
		!r.Data.Authenticated || r.Data.Profile != a.recoveryProfile || r.Data.Activation != a.recoveryActivation)
}

func (a *App) updateEmittedRecovery(q ipc.Request, r ipc.Response) {
	if (q.Op == "access.renew-recovery" || q.Op == "access.register" || q.Op == "access.recover" ||
		q.Op == "access.reset-complete") && r.Code == "OK" && r.Data != nil {
		a.recovery = r.Data.Recovery
		a.renewalRecoveryPending = q.Op == "access.renew-recovery" && r.Data.Recovery != ""
		a.recoveryGeneration++
		a.recoveryInstance = r.Data.Instance
		a.recoveryProfile = r.Data.Profile
		a.recoveryActivation = r.Data.Activation
	}
}

func (a *App) updateRecoveryAcknowledgement(q ipc.Request, r ipc.Response) {
	if q.Op == "access.ack-recovery" && r.Code == "OK" {
		a.recovery = ""
		a.renewalRecoveryPending = false
		a.recoveryGeneration++
	}
	if (q.Op == "access.register" || q.Op == "access.recover" || q.Op == "access.reset-complete") && r.Code == "OK" && r.Data != nil {
		a.accessRecoveryPending = r.Data.Recovery != ""
		a.renewalRecoveryPending = false
	}
	if q.Op == "access.bootstrap" && (r.Code == "UNAUTHENTICATED" || (r.Code == "OK" && r.Data != nil && !r.Data.Authenticated)) {
		a.renewalRecoveryPending = false
	}
}
