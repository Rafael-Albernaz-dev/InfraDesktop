package ipc

import (
	"encoding/json"
	"fmt"
	"infradesktop/internal/modules/foundation"
	"os"
	"strings"
	"testing"
)

func TestProductSchemaSnapshot(t *testing.T) {
	b, e := os.ReadFile("schema-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Version    int                 `json:"version"`
		Operations map[string][]string `json:"product_operations"`
	}
	if e = json.Unmarshal(b, &schema); e != nil {
		t.Fatal(e)
	}
	if schema.Version != 1 || len(schema.Operations) != 20 {
		t.Fatal("schema snapshot version/count")
	}
	samples := map[string]any{
		"contract_revision": ProductRevision, "expected_activation": "activation",
		"brand":        Branding{Name: "", Logo: "", Accent: "teal"},
		"operation_id": "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", "expected_profile": "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa",
		"remember": false, "replace_pending": false, "columns": []string{"equipment"},
	}
	for op, fields := range schema.Operations {
		q := map[string]any{"version": 1, "request_id": "snapshot", "op": op}
		for _, f := range fields {
			if _, ok := q[f]; ok {
				continue
			}
			q[f] = ""
			if sample, ok := samples[f]; ok {
				q[f] = sample
			}
		}
		b, _ := json.Marshal(q)
		if _, c := validate(b); c != foundation.OK {
			t.Fatalf("snapshot %s rejected %s", op, c)
		}
		q["unexpected_field"] = ""
		b, _ = json.Marshal(q)
		if _, c := validate(b); c != foundation.Invalid {
			t.Fatalf("unknown %s field accepted", op)
		}
	}
}

func TestAccessSchema(t *testing.T) {
	capability := `"capability":"` + strings.Repeat("a", 64) + `"`
	login := `"name":"technical","password":"valid long test password"`
	for _, test := range []struct {
		op, fields string
		want       foundation.Code
	}{
		{"access.bootstrap", capability, foundation.OK},
		{"access.bootstrap", `"password":"unexpected"`, foundation.Invalid},
		{"access.login", capability + "," + login + `,"remember":true`, foundation.OK},
		{"access.login", login + `,"remember":"true"`, foundation.Invalid},
		{"company.select", `"name":"x","profile_id":"foreign","activation_id":"old",` +
			`"session":"missing","grant":"unexpected"`, foundation.Invalid},
	} {
		if _, got := validate(wire(test.op, test.fields)); got != test.want {
			t.Errorf("schema %s: got %s want %s", test.op, got, test.want)
		}
	}
}

func TestAccessRejectsOldRevisionAndForeignFields(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"request_id":"old","op":"access.bootstrap"}`,
		`{"version":1,"contract_revision":1,"request_id":"old","op":"access.bootstrap"}`,
	} {
		if _, c := validate([]byte(body)); c != foundation.Version {
			t.Fatal("old product contract not rejected", c)
		}
	}
	for _, extra := range []string{
		`"operation_id":"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"`, `"replace_pending":false`,
		`"password":""`, `"owner":"forged"`, `"path":"/tmp/foreign.db"`,
	} {
		if _, c := validate(wire("access.snapshot", extra)); c != foundation.Invalid {
			t.Fatal("cross-operation capability accepted", extra, c)
		}
	}
	for _, operation := range []string{"", "../foreign", "AAAAAAAA-AAAA-4AAA-AAAA-AAAAAAAAAAAA"} {
		q := Request{Version: 1, ContractRevision: ProductRevision, RequestID: "create", Op: "company.create"}
		q.Operation = operation
		b, _ := json.Marshal(q)
		if _, c := validate(b); c != foundation.Invalid {
			t.Fatal("invalid operation identity accepted")
		}
	}
}

// wire builds a literal request on the current product contract, so a revision
// bump cannot turn a field-validation case into a version rejection.
func wire(op, fields string) []byte {
	body := fmt.Sprintf(`{"version":1,"contract_revision":%d,"request_id":"test","op":%q`, ProductRevision, op)
	if fields != "" {
		body += "," + fields
	}
	return []byte(body + "}")
}
