package ipc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
)

// EquipmentInput is operation-specific on the wire; presence allowlists below
// reject unrelated fields even when their values are zero.
type EquipmentInput struct {
	ID           string `json:"id,omitempty"`
	Operation    string `json:"operation_id,omitempty"`
	Revision     uint64 `json:"revision,omitempty"`
	Role         string `json:"role,omitempty"`
	Name         string `json:"name,omitempty"`
	Address      string `json:"address,omitempty"`
	Port         int    `json:"port,omitempty"`
	Transport    string `json:"transport,omitempty"`
	CredentialID string `json:"credential_id,omitempty"`
	Active       bool   `json:"active,omitempty"`
	Inactive     bool   `json:"inactive,omitempty"`
	Search       string `json:"search,omitempty"`
	Cursor       int    `json:"cursor,omitempty"`
	Alias        string `json:"alias,omitempty"`
	User         string `json:"user,omitempty"`
	Secret       string `json:"secret,omitempty"`
	Affected     int    `json:"affected_links,omitempty"`
	JobID        string `json:"job_id,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
}

var equipmentFields = map[string][]string{
	"trust.reset":             {"operation_id", "id", "revision"},
	"inventory.enroll":        {"operation_id", "id", "name", "role", "address", "port", "transport", "user", "secret"},
	"inventory.enroll-status": {"operation_id", "id"},
	"inventory.list":          {"search", "cursor", "inactive"},
	"inventory.get":           {"id"},
	"inventory.create":        {"operation_id", "id", "name", "role", "address", "port", "transport", "credential_id"},
	"inventory.update":        {"operation_id", "id", "revision", "name", "role", "address", "port", "transport", "credential_id"},
	"inventory.set-active":    {"operation_id", "id", "revision", "active"},
	"credential.list":         {"cursor"},
	"credential.put":          {"operation_id", "id", "revision", "alias", "user", "secret", "affected_links"},
	"trust.inspect":           {"operation_id", "id", "revision"},
	"trust.confirm":           {"operation_id", "job_id", "id", "revision", "fingerprint"},
	"device.query-start":      {"operation_id", "id", "revision"},
	"device.query-status":     {"job_id"},
	"device.query-cancel":     {"job_id"},
}

func isEquipment(op string) bool { _, ok := equipmentFields[op]; return ok }
func validateEquipment(b []byte, q Request, fields map[string]json.RawMessage) (Request, foundation.Code) {
	if q.ContractRevision != ProductRevision {
		return q, foundation.Version
	}
	for _, key := range []string{"version", "request_id", "op", "capability", "contract_revision", "session", "profile_id", "activation_id", "expected_profile", "expected_activation", "equipment"} {
		if _, ok := fields[key]; !ok {
			return q, foundation.Invalid
		}
	}
	if len(fields) != 11 || len(b) > MaxFrame || q.Equipment == nil || !access.ValidCompanyID(q.ExpectedProfile) || !validID(q.ExpectedActivation) {
		return q, foundation.Invalid
	}
	var input EquipmentInput
	if strict(fields["equipment"], &input) != nil {
		return q, foundation.Invalid
	}
	allowed := map[string]bool{}
	for _, k := range equipmentFields[q.Op] {
		allowed[k] = true
	}
	var nested map[string]json.RawMessage
	_ = json.Unmarshal(fields["equipment"], &nested)
	for k := range nested {
		if !allowed[k] {
			return q, foundation.Invalid
		}
	}
	// Required nonzero semantic fields are checked again by the capability.
	return q, foundation.OK
}
func AttachEquipment(s *Server, service *equipment.Service) { s.equipment = service }
func (s *Server) dispatchEquipment(ctx context.Context, uid uint32, q Request, owner string) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.Invalid}
	if s.equipment == nil || q.Equipment == nil {
		return r
	}
	if subtle.ConstantTimeCompare([]byte(q.Capability), []byte(s.capability)) != 1 {
		r.Code = foundation.Unauthenticated
		return r
	}
	s.account.Reap(ownerAlive)
	e := q.Equipment
	v, c := s.equipment.Handle(ctx, equipment.Command{Op: q.Op, UID: uid, Scope: access.Command{Session: q.Session, Owner: owner, Profile: q.Profile, Activation: q.Activation, ExpectedProfile: q.ExpectedProfile, ExpectedActivation: q.ExpectedActivation}, ID: e.ID, Operation: e.Operation, Revision: e.Revision, Role: e.Role, Name: e.Name, Address: e.Address, Port: e.Port, Transport: e.Transport, CredentialID: e.CredentialID, Active: e.Active, Inactive: e.Inactive, Search: e.Search, Cursor: e.Cursor, Alias: e.Alias, User: e.User, Secret: e.Secret, Affected: e.Affected, JobID: e.JobID, Fingerprint: e.Fingerprint})
	r.Code = foundation.Code(c)
	if c != equipment.OK {
		return r
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) > 48*1024 {
		r.Code = foundation.Code(equipment.Limit)
		return r
	}
	r.Data = &Data{ContractRevision: ProductRevision, ProductVersion: ProductVersion, Instance: s.identity.Instance, Profile: q.Profile, Activation: q.Activation, Authenticated: true, Equipment: &v}
	return r
}
