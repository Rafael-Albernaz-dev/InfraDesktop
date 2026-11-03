package ipc

import (
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
)

func validateRadio(b []byte, q Request, fields map[string]json.RawMessage) (Request, foundation.Code) {
	if q.ContractRevision != ProductRevision {
		return q, foundation.Version
	}
	keys := []string{"version",
		"request_id",
		"op",
		"capability",
		"contract_revision",
		"session",
		"profile_id",
		"activation_id",
		"expected_profile",
		"expected_activation",
		"radio"}
	if len(fields) != len(keys) ||
		len(b) > MaxFrame ||
		q.Radio == nil ||
		!access.ValidCompanyID(q.ExpectedProfile) ||
		!validID(q.ExpectedActivation) {
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
	if !validRadioLinkObject(fields["radio"]) {
		return q, foundation.Invalid
	}
	if in.Offset < 0 || in.Offset > equipment.RadioMaxClients {
		return q, foundation.Invalid
	}
	q.Radio = &in
	return q, foundation.OK
}

func validRadioLinkObject(rawRadio json.RawMessage) bool {
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(rawRadio, &raw)
	if link, ok := raw["link"]; ok {
		allowed := []string{"cpe_id", "ap_id", "mac", "interface", "bssid", "evidence", "origin", "credential_id"}
		if !strictPPObject(link, allowed) {
			return false
		}
		var nested map[string]json.RawMessage
		_ = json.Unmarshal(link, &nested)
		for _, required := range allowed[:7] {
			if _, ok := nested[required]; !ok {
				return false
			}
		}
	}
	return true
}
