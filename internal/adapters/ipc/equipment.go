package ipc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
)

var equipmentFields = map[string][]string{
	"credential.default-set":   {"id", "operation_id", "role", "credential_id"},
	"trust.reset":              {"operation_id", "id", "revision"},
	"inventory.enroll":         {"operation_id", "id", "name", "role", "address", "port", "transport", "user", "secret", "block"},
	"inventory.enroll-status":  {"operation_id", "id"},
	"inventory.list":           {"search", "cursor", "inactive", "include_inactive", "role", "block", "block_empty", "model"},
	"inventory.get":            {"id"},
	"inventory.import-preview": {"import"},
	"inventory.import-confirm": {"operation_id", "import"},
	"inventory.bulk-update":    {"operation_id", "bulk"},
	"inventory.create":         {"operation_id", "id", "name", "role", "address", "port", "transport", "credential_id", "block"},
	"inventory.update":         {"operation_id", "id", "revision", "name", "role", "address", "port", "transport", "credential_id", "block"},
	"inventory.set-active":     {"operation_id", "id", "revision", "active"},
	"credential.list":          {"cursor"},
	"credential.put":           {"operation_id", "id", "revision", "alias", "user", "secret", "affected_links"},
	"trust.inspect":            {"operation_id", "id", "revision"},
	"trust.confirm":            {"operation_id", "job_id", "id", "revision", "fingerprint"},
	"device.query-start":       {"operation_id", "id", "revision"},
	"device.query-status":      {"job_id"},
	"device.query-cancel":      {"job_id"},
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
	if !validEquipmentEnvelope(q, len(fields), len(b)) {
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
	v, c := s.equipment.Handle(ctx, equipmentCommand(uid, owner, q))
	r.Code = foundation.Code(c)
	if c != equipment.OK {
		return r
	}
	b, err := json.Marshal(v)
	if err != nil || len(b) > equipmentResponseLimit(q.Op) {
		r.Code = foundation.Code(equipment.Limit)
		return r
	}
	r.Data = &Data{ContractRevision: ProductRevision, ProductVersion: ProductVersion, Instance: s.identity.Instance, Profile: q.Profile, Activation: q.Activation, Authenticated: true, Equipment: &v}
	return r
}
