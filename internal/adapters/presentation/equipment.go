package presentation

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

type EquipmentResult struct {
	Code       string          `json:"code"`
	Profile    string          `json:"profile_id"`
	Activation string          `json:"activation_id"`
	Data       *equipment.View `json:"data,omitempty"`
}
type DeviceInput struct {
	User         string `json:"user"`
	Secret       string `json:"secret"`
	ID           string `json:"id"`
	Operation    string `json:"operation_id"`
	Revision     uint64 `json:"revision"`
	Role         string `json:"role,omitempty"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	Port         int    `json:"port"`
	Transport    string `json:"transport"`
	CredentialID string `json:"credential_id"`
}
type CredentialInput struct {
	ID        string `json:"id"`
	Operation string `json:"operation_id"`
	Revision  uint64 `json:"revision"`
	Alias     string `json:"alias"`
	User      string `json:"user"`
	Secret    string `json:"secret"`
	Affected  int    `json:"affected_links"`
}

func (a *App) equipmentCall(op, profile, activation string, input ipc.EquipmentInput) EquipmentResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	r := a.client.Call(a.ctx, ipc.Request{Op: op, ExpectedProfile: profile, ExpectedActivation: activation, Equipment: &input})
	result := EquipmentResult{Code: string(r.Code), Profile: profile, Activation: activation}
	if r.Data != nil {
		result.Data = r.Data.Equipment
	}
	return result
}
func (a *App) ListEquipment(profile, activation, search string, cursor int, inactive bool) EquipmentResult {
	return a.equipmentCall("inventory.list", profile, activation, ipc.EquipmentInput{Search: search, Cursor: cursor, Inactive: inactive})
}
func (a *App) GetEquipment(profile, activation, id string) EquipmentResult {
	return a.equipmentCall("inventory.get", profile, activation, ipc.EquipmentInput{ID: id})
}
func (a *App) SaveEquipment(profile, activation string, d DeviceInput) EquipmentResult {
	op := "inventory.update"
	if d.Revision == 0 {
		op = "inventory.create"
		if d.User != "" || d.Secret != "" {
			op = "inventory.enroll"
		}
	}
	return a.equipmentCall(op, profile, activation, ipc.EquipmentInput{ID: d.ID, Operation: d.Operation, Revision: d.Revision, Role: d.Role, Name: d.Name, Address: d.Address, Port: d.Port, Transport: d.Transport, CredentialID: d.CredentialID, User: d.User, Secret: d.Secret})
}
func (a *App) SetEquipmentActive(profile, activation, id, operation string, revision uint64, active bool) EquipmentResult {
	return a.equipmentCall("inventory.set-active", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation, Revision: revision, Active: active})
}
func (a *App) ListCredentials(profile, activation string, cursor int) EquipmentResult {
	return a.equipmentCall("credential.list", profile, activation, ipc.EquipmentInput{Cursor: cursor})
}
func (a *App) PutCredential(profile, activation string, d CredentialInput) EquipmentResult {
	return a.equipmentCall("credential.put", profile, activation, ipc.EquipmentInput{ID: d.ID, Operation: d.Operation, Revision: d.Revision, Alias: d.Alias, User: d.User, Secret: d.Secret, Affected: d.Affected})
}
func (a *App) InspectTrust(profile, activation, id, operation string, revision uint64) EquipmentResult {
	return a.equipmentCall("trust.inspect", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation, Revision: revision})
}
func (a *App) ConfirmTrust(profile, activation, id, operation, job, fingerprint string, revision uint64) EquipmentResult {
	return a.equipmentCall("trust.confirm", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation, JobID: job, Fingerprint: fingerprint, Revision: revision})
}
func (a *App) StartQuery(profile, activation, id, operation string, revision uint64) EquipmentResult {
	return a.equipmentCall("device.query-start", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation, Revision: revision})
}
func (a *App) QueryStatus(profile, activation, job string) EquipmentResult {
	return a.equipmentCall("device.query-status", profile, activation, ipc.EquipmentInput{JobID: job})
}
func (a *App) CancelQuery(profile, activation, job string) EquipmentResult {
	return a.equipmentCall("device.query-cancel", profile, activation, ipc.EquipmentInput{JobID: job})
}

func (a *App) EnrollmentStatus(profile, activation, id, operation string) EquipmentResult {
	return a.equipmentCall("inventory.enroll-status", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation})
}

func (a *App) ResetTrust(profile, activation, id, operation string, revision uint64) EquipmentResult {
	return a.equipmentCall("trust.reset", profile, activation, ipc.EquipmentInput{ID: id, Operation: operation, Revision: revision})
}
