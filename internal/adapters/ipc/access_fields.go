package ipc

import (
	"encoding/json"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
)

func accessAllowed(q Request) (map[string]bool, foundation.Code) {
	if q.ContractRevision != ProductRevision {
		return nil, foundation.Version
	}
	a := map[string]bool{"version": true, "request_id": true, "op": true, "capability": true, "contract_revision": true}
	switch q.Op {
	case "access.check-issued", "access.ack-issued":
		a["recovery"] = true
	case "access.register":
		a["name"], a["password"] = true, true
	case "access.login":
		a["name"], a["password"], a["remember"] = true, true, true
	case "access.recover":
		a["name"], a["password"], a["recovery"] = true, true, true
	case "access.resume":
		a["grant"] = true
	case "access.snapshot", "access.logout", "company.create", "company.select", "preferences.theme",
		"preferences.columns", "access.renew-recovery", "access.ack-recovery", "company.brand-get", "company.brand-set":
		a["session"], a["profile_id"], a["activation_id"] = true, true, true
		addAuthenticatedAccessFields(q.Op, a)
	}
	return a, foundation.OK
}

func addAuthenticatedAccessFields(op string, a map[string]bool) {
	if op == "company.brand-get" || op == "company.brand-set" {
		a["expected_profile"], a["expected_activation"] = true, true
		if op == "company.brand-set" {
			a["brand"] = true
		}
	}
	if op == "access.renew-recovery" {
		a["password"], a["replace_pending"] = true, true
	}
	if op == "company.create" {
		a["operation_id"] = true
	}
	if op == "company.create" || op == "company.select" {
		a["name"] = true
	}
	if op == "preferences.theme" {
		a["theme"] = true
	}
	if op == "preferences.columns" {
		a["columns"] = true
	}
}

func validateEnvelope(q Request, b []byte, fields map[string]json.RawMessage, allowed map[string]bool) (Request, foundation.Code) {
	for k := range fields {
		if !allowed[k] {
			return q, foundation.Invalid
		}
	}
	if len(b) > MaxFrame && q.Op != "company.brand-set" {
		return q, foundation.Invalid
	}
	if q.Op == "company.brand-get" || q.Op == "company.brand-set" {
		if !access.ValidCompanyID(q.ExpectedProfile) || !validID(q.ExpectedActivation) {
			return q, foundation.Invalid
		}
		if q.Op == "company.brand-set" {
			var brand Branding
			if q.Brand == nil || strict(fields["brand"], &brand) != nil {
				return q, foundation.Invalid
			}
		}
	}
	if q.Op == "company.create" && !access.ValidCompanyID(q.Operation) {
		return q, foundation.Invalid
	}
	return q, foundation.OK
}
