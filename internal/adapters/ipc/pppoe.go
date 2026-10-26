package ipc

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"strconv"
	"strings"
)

var ppFields = map[string][]string{
	"pppoe.diagnostic-start":  {"device_id", "revision", "operation_id", "scope", "observation_id", "row_id"},
	"pppoe.diagnostic-status": {"job_id"}, "pppoe.diagnostic-cancel": {"job_id"},
	"pppoe.diagnostic-current": {"device_id", "scope"},
	"pppoe.diagnostic-list":    {"device_id", "scope", "diagnostic_id", "cursor"},
	"pppoe.collect-start":      {"device_id", "revision", "operation_id"},
	"pppoe.collect-status":     {"job_id"}, "pppoe.collect-cancel": {"job_id"},
	"pppoe.current": {"device_id"}, "pppoe.observation-get": {"device_id", "observation_id"},
	"pppoe.sessions-list":  {"device_id", "observation_id", "query", "cursor"},
	"pppoe.session-get":    {"device_id", "observation_id", "row_id"},
	"pppoe.reference-save": {"device_id", "observation_id", "reference_id", "operation_id"},
	"pppoe.reference-list": {"device_id", "cursor"}, "pppoe.reference-delete": {"device_id", "reference_id", "operation_id"},
	"pppoe.compare":     {"device_id", "base_id", "target_id", "query", "cursor"},
	"pppoe.export-page": {"device_id", "base_id", "target_id", "cursor", "export_key", "export_part", "scope", "diagnostic_id"},
}

func isPPPoE(op string) bool { _, ok := ppFields[op]; return ok }

// Walk every object before decoding: encoding/json otherwise accepts duplicate
// keys in nested query objects. Arrays/null are not part of A's request grammar.
func strictPPObject(b []byte, allowed []string) bool {
	var obj map[string]json.RawMessage
	if json.Unmarshal(b, &obj) != nil || obj == nil {
		return false
	}
	seen := map[string]bool{}
	for _, k := range allowed {
		seen[k] = true
	}
	for k, v := range obj {
		if !seen[k] || string(v) == "null" {
			return false
		}
	}
	// strict supplies token-level duplicate detection at this object's level.
	var raw map[string]json.RawMessage
	return strict(b, &raw) == nil
}
func validatePPPoE(b []byte, q Request, fields map[string]json.RawMessage) (Request, foundation.Code) {
	if q.ContractRevision != ProductRevision {
		return q, foundation.Version
	}
	keys := []string{"version", "request_id", "op", "capability", "contract_revision", "session", "profile_id", "activation_id", "expected_profile", "expected_activation", "pppoe"}
	if len(fields) != len(keys) || len(b) > MaxFrame || q.PPPoE == nil {
		return q, foundation.Invalid
	}
	for _, k := range keys {
		if _, ok := fields[k]; !ok {
			return q, foundation.Invalid
		}
	}
	if !access.ValidCompanyID(q.ExpectedProfile) || !validID(q.ExpectedActivation) || !strictPPObject(fields["pppoe"], ppFields[q.Op]) {
		return q, foundation.Invalid
	}
	var input equipment.PPInput
	if strict(fields["pppoe"], &input) != nil {
		return q, foundation.Invalid
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(fields["pppoe"], &raw)
	if query, ok := raw["query"]; ok {
		allowedQuery := []string{"search", "duplicates_only", "sort", "desc"}
		if q.Op == "pppoe.compare" {
			allowedQuery = []string{"category"}
		}
		if !strictPPObject(query, allowedQuery) {
			return q, foundation.Invalid
		}
	}
	if !equipment.PPValidQuery(input.Query) || len(input.Cursor) > 512 {
		return q, foundation.Invalid
	}
	for k, v := range raw {
		if strings.HasSuffix(k, "_id") {
			var id string
			if json.Unmarshal(v, &id) != nil || !access.ValidCompanyID(id) {
				return q, foundation.Invalid
			}
		}
	}
	required := map[string][]string{"pppoe.collect-start": {"device_id", "revision", "operation_id"}, "pppoe.collect-status": {"job_id"}, "pppoe.collect-cancel": {"job_id"}, "pppoe.observation-get": {"device_id", "observation_id"}, "pppoe.sessions-list": {"device_id", "observation_id"}, "pppoe.session-get": {"device_id", "observation_id", "row_id"}, "pppoe.reference-save": {"device_id", "observation_id", "reference_id", "operation_id"}, "pppoe.reference-delete": {"device_id", "reference_id", "operation_id"}, "pppoe.compare": {"device_id", "base_id", "target_id"}, "pppoe.export-page": {"device_id", "target_id", "export_key"}}
	r := required[q.Op]
	if strings.HasPrefix(q.Op, "pppoe.diagnostic-") {
		switch q.Op {
		case "pppoe.diagnostic-status", "pppoe.diagnostic-cancel":
			r = []string{"job_id"}
		case "pppoe.diagnostic-start":
			r = []string{"device_id", "revision", "operation_id", "scope"}
			if input.Revision == 0 {
				return q, foundation.Invalid
			}
			if input.Scope == equipment.PDPing {
				r = append(r, "observation_id", "row_id")
			} else if _, exists := raw["observation_id"]; exists {
				return q, foundation.Invalid
			} else if _, exists := raw["row_id"]; exists {
				return q, foundation.Invalid
			}
		case "pppoe.diagnostic-current":
			r = []string{"device_id", "scope"}
		case "pppoe.diagnostic-list":
			r = []string{"device_id", "scope", "diagnostic_id"}
		}
		if q.Op != "pppoe.diagnostic-status" && q.Op != "pppoe.diagnostic-cancel" && !equipment.PDValidScope(input.Scope) {
			return q, foundation.Invalid
		}
	}
	if len(r) == 0 {
		r = []string{"device_id"}
	}
	for _, k := range r {
		if _, ok := raw[k]; !ok {
			return q, foundation.Invalid
		}
	}
	if q.Op == "pppoe.collect-start" && input.Revision == 0 {
		return q, foundation.Invalid
	}
	if q.Op == "pppoe.export-page" {
		if input.ExportPart != "" && input.ExportPart != "rows" && input.ExportPart != "comparison" && input.ExportPart != "diagnostics" {
			return q, foundation.Invalid
		}
		if input.ExportPart == "comparison" && input.BaseID == "" {
			return q, foundation.Invalid
		}
		if input.ExportPart == "diagnostics" {
			if !equipment.PDValidScope(input.Scope) || !access.ValidCompanyID(input.DiagnosticID) {
				return q, foundation.Invalid
			}
		} else if input.Scope != "" || input.DiagnosticID != "" {
			return q, foundation.Invalid
		}
		key, e := base64.RawStdEncoding.DecodeString(input.ExportKey)
		if e != nil || len(key) != 32 {
			return q, foundation.Invalid
		}
	}
	return q, foundation.OK
}
func ppDigest(q Request) string {
	p := *q.PPPoE
	p.Cursor = ""
	p.Offset = 0
	b, _ := json.Marshal([]any{q.Op, q.Profile, q.Activation, p})
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func (s *Server) ppCursor(q Request, offset int) string {
	s.ppOnce.Do(func() { _, s.ppKeyError = rand.Read(s.ppKey[:]) })
	if s.ppKeyError != nil {
		return ""
	}
	payload := strconv.Itoa(offset) + ":" + ppDigest(q)
	m := hmac.New(sha256.New, s.ppKey[:])
	m.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + ":" + hex.EncodeToString(m.Sum(nil))))
}
func (s *Server) ppOffset(q Request) (int, bool) {
	if q.PPPoE.Cursor == "" {
		return 0, true
	}
	b, e := base64.RawURLEncoding.DecodeString(q.PPPoE.Cursor)
	if e != nil {
		return 0, false
	}
	f := strings.Split(string(b), ":")
	if len(f) != 3 {
		return 0, false
	}
	n, e := strconv.Atoi(f[0])
	if e != nil || n < 0 || n > 4096 {
		return 0, false
	}
	expected := s.ppCursor(q, n)
	return n, hmac.Equal([]byte(expected), []byte(q.PPPoE.Cursor))
}
func ppToken(key []byte, kind, value string) string {
	if value == "" {
		return ""
	}
	m := hmac.New(sha256.New, key)
	m.Write([]byte(kind + "\x00" + value))
	return hex.EncodeToString(m.Sum(nil))
}
func (s *Server) dispatchPPPoE(ctx context.Context, uid uint32, q Request, owner string) Response {
	r := Response{Version: 1, RequestID: q.RequestID, Code: foundation.Invalid}
	if s.equipment == nil || q.PPPoE == nil {
		return r
	}
	if subtle.ConstantTimeCompare([]byte(q.Capability), []byte(s.capability)) != 1 {
		r.Code = foundation.Unauthenticated
		return r
	}
	s.account.Reap(ownerAlive)
	// Authenticate before even evaluating a cursor, avoiding a cross-company oracle.
	scope := access.Command{Owner: owner, Session: q.Session, Profile: q.Profile, Activation: q.Activation, ExpectedProfile: q.ExpectedProfile, ExpectedActivation: q.ExpectedActivation}
	if c := s.account.WithAuthorization(ctx, uid, scope, func(uint64) access.Code { return access.OK }); c != access.OK {
		r.Code = foundation.Code(c)
		return r
	}
	offset, ok := s.ppOffset(q)
	if !ok {
		r.Code = foundation.Code(equipment.CursorInvalid)
		return r
	}
	input := *q.PPPoE
	input.Offset = offset
	v, c := s.equipment.HandlePP(ctx, equipment.Command{Op: q.Op, UID: uid, Scope: scope}, input)
	r.Code = foundation.Code(c)
	if c != equipment.OK {
		if c == equipment.Limit && v.Reason != "" {
			r.ErrorDetail = &ErrorDetail{Reason: v.Reason}
		}
		if c == "JOB_INTERRUPTED" && v.Entity != "" {
			r.ErrorDetail = &ErrorDetail{Entity: v.Entity}
		}
		return r
	}
	if q.Op == "pppoe.export-page" {
		key, _ := base64.RawStdEncoding.DecodeString(input.ExportKey)
		for _, row := range v.DiagnosticRows {
			id := v.Diagnostic.ID
			row.ID = ppToken(key, "diagnostic-row", id+row.ID)
			if row.RemoteID != "" {
				row.RemoteID = ppToken(key, "diagnostic-remote", input.Scope+"/"+row.RemoteID)
			}
			row.Name = ppToken(key, "diagnostic-name", row.Name)
			row.Parent = ppToken(key, "diagnostic-name", row.Parent)
			if row.ParentID != "" {
				row.ParentID = ppToken(key, "diagnostic-row", id+row.ParentID)
			}
			row.Address = ppToken(key, "address", row.Address)
			switch row.Type {
			case "", "ether", "vlan", "bridge", "pppoe-in", "pppoe-out", "wlan", "wifi", "bonding", "eoip", "veth", "loopback", "gre", "vxlan", "wireguard":
			default:
				row.Type = "unknown"
			}
			v.ExportDiagnostics = append(v.ExportDiagnostics, row)
		}
		for _, row := range v.Rows {
			v.ExportRows = append(v.ExportRows, equipment.PPExportRow{Observation: ppToken(key, "observation", v.Current.ID), Row: ppToken(key, "row", v.Current.ID+row.ID), Login: ppToken(key, "login", row.Login), Address: ppToken(key, "address", row.Address), Caller: ppToken(key, "caller", row.Caller)})
		}
		for _, delta := range v.Deltas {
			v.ExportDeltas = append(v.ExportDeltas, equipment.PPExportDelta{BaseRow: ppToken(key, "row", func() string {
				if delta.BaseRow == "" {
					return ""
				}
				return input.BaseID + delta.BaseRow
			}()), TargetRow: ppToken(key, "row", func() string {
				if delta.TargetRow == "" {
					return ""
				}
				return input.TargetID + delta.TargetRow
			}()), Category: delta.Category, Reason: delta.Reason, AddressChanged: delta.AddressChanged, CallerChanged: delta.CallerChanged})
		}
		v.Deltas = nil
		v.Rows = nil
		v.Current = nil
		v.Diagnostic = nil
		v.DiagnosticRows = nil
	}
	if v.More {
		v.Next = s.ppCursor(q, v.NextOffset)
		if v.Next == "" {
			r.Code = foundation.Unavailable
			return r
		}
	}
	d := &Data{ContractRevision: ProductRevision, ProductVersion: ProductVersion, Instance: s.identity.Instance, Profile: q.Profile, Activation: q.Activation, Authenticated: true, PPPoE: &v}
	r.Data = d
	b, e := json.Marshal(r)
	if e != nil || len(b) > 48*1024 {
		r.Data = nil
		r.Code = foundation.Code(equipment.Limit)
		r.ErrorDetail = &ErrorDetail{Reason: "frame"}
	}
	return r
}
func validErrorDetail(r Response) bool {
	d := r.ErrorDetail
	if d == nil {
		return true
	}
	if r.Code == "LIMIT_EXCEEDED" && d.Entity == "" {
		switch d.Reason {
		case "operation_receipts", "references_device", "references_company", "payload", "frame", "job_slots":
			return true
		}
	}
	return r.Code == "JOB_INTERRUPTED" && d.Reason == "" && access.ValidCompanyID(d.Entity)
}
