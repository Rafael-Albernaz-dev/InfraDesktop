// Package ipc implements the Linux transport and versioned presentation schema.
package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"io"
	"time"
	"unicode/utf8"
)

const ProductRevision = 24
const ProductVersion = "0.6.0"
const MaxFrame = 65536
const MaxBrandFrame = 2863104
const Budget = 3 * time.Second

var errProtocol = errors.New("invalid frame")

type Request struct {
	Columns            []string              `json:"columns,omitempty"`
	Radio              *equipment.RadioInput `json:"radio,omitempty"`
	PPPoE              *equipment.PPInput    `json:"pppoe,omitempty"`
	Equipment          *EquipmentInput       `json:"equipment,omitempty"`
	ContractRevision   int                   `json:"contract_revision,omitempty"`
	Version            int                   `json:"version"`
	RequestID          string                `json:"request_id"`
	Op                 string                `json:"op"`
	Capability         string                `json:"capability,omitempty"`
	Session            string                `json:"session,omitempty"`
	Profile            string                `json:"profile_id,omitempty"`
	Activation         string                `json:"activation_id,omitempty"`
	Label              string                `json:"label,omitempty"`
	Name               string                `json:"name,omitempty"`
	Password           string                `json:"password,omitempty"`
	ResetNonce         string                `json:"reset_nonce,omitempty"`
	Recovery           string                `json:"recovery,omitempty"`
	Grant              string                `json:"grant,omitempty"`
	Theme              string                `json:"theme,omitempty"`
	Operation          string                `json:"operation_id,omitempty"`
	Replace            bool                  `json:"replace_pending,omitempty"`
	Remember           bool                  `json:"remember,omitempty"`
	ExpectedProfile    string                `json:"expected_profile,omitempty"`
	ExpectedActivation string                `json:"expected_activation,omitempty"`
	Brand              *Branding             `json:"brand,omitempty"`
}
type Branding struct {
	Name   string `json:"name"`
	Logo   string `json:"logo"`
	Accent string `json:"accent"`
}
type Company struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type Data struct {
	Columns          []string             `json:"columns,omitempty"`
	Radio            *equipment.RadioView `json:"radio,omitempty"`
	VaultAvailable   bool                 `json:"vault_available"`
	PPPoE            *equipment.PPView    `json:"pppoe,omitempty"`
	Equipment        *equipment.View      `json:"equipment,omitempty"`
	ContractRevision int                  `json:"contract_revision,omitempty"`
	ProductVersion   string               `json:"product_version,omitempty"`
	Brand            *Branding            `json:"brand,omitempty"`
	Setup            bool                 `json:"setup"`
	Authenticated    bool                 `json:"authenticated"`
	Name             string               `json:"name,omitempty"`
	Theme            string               `json:"theme,omitempty"`
	Recovery         string               `json:"recovery,omitempty"`
	ResetNonce       string               `json:"reset_nonce,omitempty"`
	Grant            string               `json:"grant,omitempty"`
	Companies        []Company            `json:"companies,omitempty"`
	Session          string               `json:"session,omitempty"`
	Expires          string               `json:"expires_at,omitempty"`
	Instance         string               `json:"instance_id"`
	Activation       string               `json:"activation_id"`
	Profile          string               `json:"profile_id"`
	Heartbeat        int64                `json:"heartbeat"`
	Observations     uint64               `json:"observations"`
	Label            string               `json:"label,omitempty"`
	Value            *int                 `json:"value,omitempty"`
	At               string               `json:"observed_at,omitempty"`
	Source           string               `json:"source,omitempty"`
	Quality          string               `json:"quality,omitempty"`
}
type ErrorDetail struct {
	Reason string `json:"reason,omitempty"`
	Entity string `json:"entity_id,omitempty"`
}
type Response struct {
	ErrorDetail *ErrorDetail    `json:"error_detail,omitempty"`
	Version     int             `json:"version"`
	RequestID   string          `json:"request_id"`
	Code        foundation.Code `json:"code"`
	Data        *Data           `json:"data,omitempty"`
}

func readFrame(r io.Reader) ([]byte, error) { return readFrameLimit(r, MaxFrame) }
func readFrameLimit(r io.Reader, limit uint32) ([]byte, error) {
	var h [4]byte
	if _, e := io.ReadFull(r, h[:]); e != nil {
		return nil, e
	}
	n := binary.BigEndian.Uint32(h[:])
	if n&(1<<31) != 0 {
		if limit <= MaxFrame {
			return nil, errProtocol
		}
		n &^= 1 << 31
		if n <= MaxFrame {
			return nil, errProtocol
		}
	} else {
		limit = MaxFrame
	}
	if n < 1 || n > limit {
		return nil, errProtocol
	}
	b := make([]byte, int(n))
	_, e := io.ReadFull(r, b)
	return b, e
}
func writeFrame(w io.Writer, v any) error { return writeFrameLimit(w, v, MaxFrame) }
func writeFrameLimit(w io.Writer, v any, limit int) error {
	b, e := json.Marshal(v)
	if e != nil {
		return e
	}
	if len(b) < 1 || len(b) > limit {
		return errProtocol
	}
	var h [4]byte
	length := uint32(len(b))
	if len(b) > MaxFrame {
		length |= 1 << 31
	}
	binary.BigEndian.PutUint32(h[:], length)
	for _, p := range [][]byte{h[:], b} {
		for len(p) > 0 {
			n, e := w.Write(p)
			if e != nil {
				return e
			}
			if n == 0 {
				return io.ErrShortWrite
			}
			p = p[n:]
		}
	}
	return nil
}

// Reject duplicate keys and null, which encoding/json otherwise silently accepts.
func strict(b []byte, v any) error {
	if !utf8.Valid(b) {
		return errProtocol
	}
	d := json.NewDecoder(bytes.NewReader(b))
	t, e := d.Token()
	if e != nil || t != json.Delim('{') {
		return errProtocol
	}
	seen := map[string]bool{}
	for d.More() {
		t, e = d.Token()
		if e != nil {
			return errProtocol
		}
		k, ok := t.(string)
		if !ok || seen[k] {
			return errProtocol
		}
		seen[k] = true
		var raw json.RawMessage
		if d.Decode(&raw) != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return errProtocol
		}
	}
	if _, e = d.Token(); e != nil {
		return errProtocol
	}
	if _, e = d.Token(); e != io.EOF {
		return errProtocol
	}
	d = json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(v)
}
func validID(s string) bool {
	if len(s) < 1 || len(s) > 64 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}
func validate(b []byte) (Request, foundation.Code) {
	var q Request
	if strict(b, &q) != nil {
		return q, foundation.Invalid
	}
	var required map[string]json.RawMessage
	_ = json.Unmarshal(b, &required)
	for _, field := range []string{"version", "request_id", "op"} {
		if _, ok := required[field]; !ok {
			return q, foundation.Invalid
		}
	}
	if q.Version != 1 {
		return q, foundation.Version
	}
	if !validID(q.RequestID) {
		return q, foundation.Invalid
	}
	// Presence, not only zero values, is operation-specific.
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(b, &fields)
	allowed := map[string]bool{"version": true, "request_id": true, "op": true, "profile_id": true}
	switch q.Op {
	case "handshake":
		allowed["capability"] = true
		if len(q.Capability) != 64 {
			return q, foundation.Unauthenticated
		}
	case "foundation.status", "foundation.observe":
		allowed["session"] = true
		allowed["activation_id"] = true
		if q.Op == "foundation.observe" {
			allowed["label"] = true
		}
	case "access.bootstrap", "access.register", "access.login", "access.recover", "access.resume",
		"access.reset-begin", "access.reset-complete",
		"access.snapshot", "access.logout", "company.create", "company.select", "preferences.theme",
		"preferences.columns", "access.renew-recovery", "access.ack-recovery", "access.check-issued",
		"access.ack-issued", "company.brand-get", "company.brand-set":
		var code foundation.Code
		allowed, code = accessAllowed(q)
		if code != foundation.OK {
			return q, code
		}
	default:
		if isRadio(q.Op) {
			return validateRadio(b, q, fields)
		}
		if isPPPoE(q.Op) {
			return validatePPPoE(b, q, fields)
		}
		if isEquipment(q.Op) {
			return validateEquipment(b, q, fields)
		}
		return q, foundation.Invalid
	}
	return validateEnvelope(q, b, fields, allowed)
}
