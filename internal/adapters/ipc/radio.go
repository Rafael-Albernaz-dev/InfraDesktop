package ipc

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
)

var radioFields = map[string][]string{
	"radio.current":        {"device_id", "reading_id", "offset"},
	"radio.start":          {"device_id", "revision", "operation_id", "observation_id", "row_id", "family", "mode"},
	"radio.status":         {"job_id", "reading_id", "offset"},
	"radio.cancel":         {"job_id", "device_id", "operation_id"},
	"radio.confirm-family": {"device_id", "revision", "family", "warning", "critical"},
	"radio.link-current":   {"device_id", "observation_id", "row_id"},
	"radio.link-confirm":   {"device_id", "observation_id", "row_id", "operation_id", "link"},
}

func isRadio(op string) bool { _, ok := radioFields[op]; return ok }
func validateRadio(b []byte, q Request, fields map[string]json.RawMessage) (Request, foundation.Code) {
	if q.ContractRevision != ProductRevision {
		return q, foundation.Version
	}
	keys := []string{"version", "request_id", "op", "capability", "contract_revision", "session", "profile_id", "activation_id", "expected_profile", "expected_activation", "radio"}
	if len(fields) != len(keys) || len(b) > MaxFrame || q.Radio == nil || !access.ValidCompanyID(q.ExpectedProfile) || !validID(q.ExpectedActivation) {
		return q, foundation.Invalid
	}
	for _, key := range keys {
		if _, ok := fields[key]; !ok {
			return q, foundation.Invalid
		}
	}
	if !strictPPObject(fields["radio"], radioFields[q.Op]) {
		return q, foundation.Invalid
	}
	var in equipment.RadioInput
	if strict(fields["radio"], &in) != nil {
		return q, foundation.Invalid
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(fields["radio"], &raw)
	if link, ok := raw["link"]; ok {
		allowed := []string{"cpe_id", "ap_id", "mac", "interface", "bssid", "evidence", "origin"}
		if !strictPPObject(link, allowed) {
			return q, foundation.Invalid
		}
		var nested map[string]json.RawMessage
		_ = json.Unmarshal(link, &nested)
		if len(nested) != len(allowed) {
			return q, foundation.Invalid
		}
	}
	if in.Offset < 0 || in.Offset > equipment.RadioMaxClients {
		return q, foundation.Invalid
	}
	q.Radio = &in
	return q, foundation.OK
}
func (s *Server) dispatchRadio(ctx context.Context, uid uint32, q Request, owner string) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.Invalid}
	if s.equipment == nil || q.Radio == nil {
		return r
	}
	if subtle.ConstantTimeCompare([]byte(q.Capability), []byte(s.capability)) != 1 {
		r.Code = foundation.Unauthenticated
		return r
	}
	s.account.Reap(ownerAlive)
	scope := access.Command{Session: q.Session, Owner: owner, Profile: q.Profile, Activation: q.Activation, ExpectedProfile: q.ExpectedProfile, ExpectedActivation: q.ExpectedActivation}
	v, c := s.equipment.HandleRadio(ctx, equipment.Command{Op: q.Op, UID: uid, Scope: scope}, *q.Radio)
	r.Code = foundation.Code(c)
	if c != equipment.OK {
		return r
	}
	body, err := json.Marshal(v)
	if err != nil || len(body) > 48*1024 {
		r.Code = foundation.Code(equipment.Limit)
		return r
	}
	r.Data = &Data{ContractRevision: ProductRevision, ProductVersion: ProductVersion, Instance: s.identity.Instance, Profile: q.Profile, Activation: q.Activation, Authenticated: true, Radio: &v}
	return r
}
