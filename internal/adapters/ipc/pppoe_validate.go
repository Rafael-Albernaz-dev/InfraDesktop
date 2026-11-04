package ipc

import (
	"encoding/base64"
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"strings"
)

func validatePPPoE(
	b []byte, q Request, fields map[string]json.RawMessage,
) (Request, foundation.Code) {
	if q.ContractRevision != ProductRevision {
		return q, foundation.Version
	}
	if !admitPPPoEFrame(b, q, fields) {
		return q, foundation.Invalid
	}
	var input equipment.PPInput
	if strict(fields["pppoe"], &input) != nil {
		return q, foundation.Invalid
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(fields["pppoe"], &raw)
	if !validPPPoEInputFields(q.Op, input, raw) {
		return q, foundation.Invalid
	}
	r, ok := requiredPPPoEFields(q.Op, input, raw)
	if !ok || !hasAllRequiredFields(raw, r) {
		return q, foundation.Invalid
	}
	if q.Op == "pppoe.collect-start" && input.Revision == 0 {
		return q, foundation.Invalid
	}
	if q.Op == "pppoe.export-page" && !validPPExportPageInput(input) {
		return q, foundation.Invalid
	}
	return q, foundation.OK
}

func admitPPPoEFrame(b []byte, q Request, fields map[string]json.RawMessage) bool {
	keys := []string{
		"version", "request_id", "op", "capability", "contract_revision",
		"session", "profile_id", "activation_id",
		"expected_profile", "expected_activation", "pppoe",
	}
	if len(fields) != len(keys) || len(b) > MaxFrame || q.PPPoE == nil {
		return false
	}
	for _, k := range keys {
		if _, ok := fields[k]; !ok {
			return false
		}
	}
	return access.ValidCompanyID(q.ExpectedProfile) &&
		validID(q.ExpectedActivation) &&
		strictPPObject(fields["pppoe"], ppFields[q.Op])
}

func validPPPoEInputFields(
	op string, input equipment.PPInput, raw map[string]json.RawMessage,
) bool {
	if op == "pppoe.monitor-current" && !validMonitorScope(input.Scope) {
		return false
	}
	if query, ok := raw["query"]; ok && !validPPQueryObject(op, query) {
		return false
	}
	if !equipment.PPValidQuery(input.Query) || len(input.Cursor) > 512 {
		return false
	}
	for k, v := range raw {
		if strings.HasSuffix(k, "_id") {
			var id string
			if json.Unmarshal(v, &id) != nil || !access.ValidCompanyID(id) {
				return false
			}
		}
	}
	return true
}

func validMonitorScope(scope string) bool {
	switch scope {
	case "series", "flapping", "versions", "hygiene", "equipment", "port-history", "duplicates":
		return true
	default:
		return false
	}
}

func validPPQueryObject(op string, query json.RawMessage) bool {
	allowedQuery := []string{"search", "duplicates_only", "sort", "desc", "state"}
	if op == "pppoe.diagnostic-list" {
		allowedQuery = []string{"type"}
	}
	if op == "pppoe.compare" {
		allowedQuery = []string{"category", "state"}
	}
	return strictPPObject(query, allowedQuery)
}

var staticRequiredPPFields = map[string][]string{
	"pppoe.context-counts":   {"device_id", "observation_id"},
	"pppoe.collect-start":    {"device_id", "revision", "operation_id"},
	"pppoe.collect-status":   {"job_id"},
	"pppoe.collect-cancel":   {"job_id"},
	"pppoe.observation-get":  {"device_id", "observation_id"},
	"pppoe.sessions-list":    {"device_id", "observation_id"},
	"pppoe.session-get":      {"device_id", "observation_id", "row_id"},
	"pppoe.reference-save":   {"device_id", "observation_id", "reference_id", "operation_id"},
	"pppoe.reference-delete": {"device_id", "reference_id", "operation_id"},
	"pppoe.compare":          {"device_id", "base_id", "target_id"},
	"pppoe.export-page":      {"device_id", "target_id", "export_key"},
}

func requiredPPPoEFields(
	op string, input equipment.PPInput, raw map[string]json.RawMessage,
) ([]string, bool) {
	if strings.HasPrefix(op, "pppoe.read-") {
		return requiredPPReadFields(op, input, raw)
	}
	if strings.HasPrefix(op, "pppoe.diagnostic-") {
		return requiredPPDiagnosticFields(op, input, raw)
	}
	r := staticRequiredPPFields[op]
	if len(r) == 0 {
		r = []string{"device_id"}
	}
	return r, true
}

func requiredPPReadFields(
	op string, input equipment.PPInput, raw map[string]json.RawMessage,
) ([]string, bool) {
	switch op {
	case "pppoe.read-cancel":
		if input.JobID != "" {
			return []string{"job_id"}, len(raw) == 1
		}
		return []string{"device_id", "operation_id"}, len(raw) == 2
	case "pppoe.read-start":
		return validatePPReadStart(input, raw)
	default:
		return []string{"job_id"}, true
	}
}

func validatePPReadStart(
	input equipment.PPInput, raw map[string]json.RawMessage,
) ([]string, bool) {
	if !equipment.PPReadScope(input.Scope) || input.Revision == 0 {
		return nil, false
	}
	if input.Scope == "top" || input.Scope == "health" || input.Scope == "operations" {
		_, hasRows := raw["row_ids"]
		_, hasObs := raw["observation_id"]
		if hasRows || hasObs {
			return nil, false
		}
		return []string{"device_id", "revision", "operation_id", "scope"}, true
	}
	if !validPPReadRowIDs(input.Scope, input.RowIDs) {
		return nil, false
	}
	return []string{
		"device_id", "revision", "operation_id", "scope", "observation_id", "row_ids",
	}, true
}

func validPPReadRowIDs(scope string, rowIDs []string) bool {
	if len(rowIDs) == 0 || len(rowIDs) > equipment.PageSize {
		return false
	}
	if (scope == "log" || scope == "secret") && len(rowIDs) != 1 {
		return false
	}
	seen := map[string]bool{}
	for _, id := range rowIDs {
		if !access.ValidCompanyID(id) || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func requiredPPDiagnosticFields(
	op string, input equipment.PPInput, raw map[string]json.RawMessage,
) ([]string, bool) {
	if op != "pppoe.diagnostic-status" && op != "pppoe.diagnostic-cancel" &&
		!equipment.PDValidScope(input.Scope) {
		return nil, false
	}
	switch op {
	case "pppoe.diagnostic-status", "pppoe.diagnostic-cancel":
		return []string{"job_id"}, true
	case "pppoe.diagnostic-start":
		return validatePPDiagnosticStart(input, raw)
	case "pppoe.diagnostic-current":
		return []string{"device_id", "scope"}, true
	case "pppoe.diagnostic-list":
		return []string{"device_id", "scope", "diagnostic_id"}, true
	default:
		return nil, false
	}
}

func validatePPDiagnosticStart(
	input equipment.PPInput, raw map[string]json.RawMessage,
) ([]string, bool) {
	if input.Revision == 0 {
		return nil, false
	}
	r := []string{"device_id", "revision", "operation_id", "scope"}
	if input.Scope == equipment.PDPing {
		return append(r, "observation_id", "row_id"), true
	}
	_, hasObs := raw["observation_id"]
	_, hasRow := raw["row_id"]
	if hasObs || hasRow {
		return nil, false
	}
	return r, true
}

func hasAllRequiredFields(raw map[string]json.RawMessage, required []string) bool {
	for _, k := range required {
		if _, ok := raw[k]; !ok {
			return false
		}
	}
	return true
}

func validPPExportPageInput(input equipment.PPInput) bool {
	if input.ExportPart != "" && input.ExportPart != "rows" &&
		input.ExportPart != "comparison" && input.ExportPart != "diagnostics" {
		return false
	}
	if input.ExportPart == "comparison" && input.BaseID == "" {
		return false
	}
	if input.ExportPart == "diagnostics" {
		if !equipment.PDValidScope(input.Scope) || !access.ValidCompanyID(input.DiagnosticID) {
			return false
		}
	} else if input.Scope != "" || input.DiagnosticID != "" {
		return false
	}
	key, e := base64.RawStdEncoding.DecodeString(input.ExportKey)
	return e == nil && len(key) == 32
}
