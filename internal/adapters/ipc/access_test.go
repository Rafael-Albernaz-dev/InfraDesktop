package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
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
	if schema.Version != 1 || len(schema.Operations) != 16 {
		t.Fatal("schema snapshot version/count")
	}
	for op, fields := range schema.Operations {
		q := map[string]any{"version": 1, "request_id": "snapshot", "op": op}
		for _, f := range fields {
			if _, ok := q[f]; ok {
				continue
			}
			q[f] = ""
			if f == "contract_revision" {
				q[f] = ProductRevision
			}
			if f == "expected_activation" {
				q[f] = "activation"
			}
			if f == "brand" {
				q[f] = Branding{Name: "", Logo: "", Accent: "teal"}
			}
			if f == "operation_id" || f == "expected_profile" {
				q[f] = "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"
			}
			if f == "remember" || f == "replace_pending" {
				q[f] = false
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
	for _, test := range []struct {
		body string
		want foundation.Code
	}{
		{`{"version":1,"contract_revision":7,"request_id":"bf01","op":"access.bootstrap","capability":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`, foundation.OK},
		{`{"version":1,"contract_revision":7,"request_id":"bf01","op":"access.bootstrap","password":"unexpected"}`, foundation.Invalid},
		{`{"version":1,"contract_revision":7,"request_id":"bf01","op":"access.login","capability":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","name":"technical","password":"valid long test password","remember":true}`, foundation.OK},
		{`{"version":1,"contract_revision":7,"request_id":"bf01","op":"access.login","name":"technical","password":"valid long test password","remember":"true"}`, foundation.Invalid},
		{`{"version":1,"contract_revision":7,"request_id":"bf01","op":"company.select","name":"x","profile_id":"foreign","activation_id":"old","session":"missing","grant":"unexpected"}`, foundation.Invalid},
	} {
		_, got := validate([]byte(test.body))
		if got != test.want {
			t.Errorf("schema op: got %s want %s", got, test.want)
		}
	}
}

func TestBF03RevisionAndNarrowFields(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"request_id":"old","op":"access.bootstrap"}`,
		`{"version":1,"contract_revision":1,"request_id":"old","op":"access.bootstrap"}`,
	} {
		if _, c := validate([]byte(body)); c != foundation.Version {
			t.Fatal("old product contract not rejected", c)
		}
	}
	for _, extra := range []string{`"operation_id":"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa"`, `"replace_pending":false`, `"password":""`, `"owner":"forged"`, `"path":"/tmp/foreign.db"`} {
		b := []byte(`{"version":1,"contract_revision":7,"request_id":"strict","op":"access.snapshot",` + extra + `}`)
		if _, c := validate(b); c != foundation.Invalid {
			t.Fatal("cross-operation capability accepted", extra, c)
		}
	}
	for _, operation := range []string{"", "../foreign", "AAAAAAAA-AAAA-4AAA-AAAA-AAAAAAAAAAAA"} {
		b, _ := json.Marshal(Request{Version: 1, ContractRevision: ProductRevision, RequestID: "create", Op: "company.create", Operation: operation})
		if _, c := validate(b); c != foundation.Invalid {
			t.Fatal("invalid operation identity accepted")
		}
	}
}

func TestBF04BrandFramesAndNarrowFields(t *testing.T) {
	q := Request{Version: 1, ContractRevision: ProductRevision, RequestID: "brand", Op: "company.brand-set", ExpectedProfile: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", ExpectedActivation: "activation", Brand: &Branding{Accent: "teal", Logo: strings.Repeat("a", 2796226)}}
	var frame bytes.Buffer
	if writeFrameLimit(&frame, q, MaxBrandFrame) != nil {
		t.Fatal("maximum nominal payload rejected")
	}
	b, e := readFrameLimit(&frame, MaxBrandFrame)
	if e != nil {
		t.Fatal(e)
	}
	if _, c := validate(b); c != foundation.OK {
		t.Fatal("nominal maximum rejected", c)
	}
	q.Op = "access.snapshot"
	b, _ = json.Marshal(q)
	if _, c := validate(b); c != foundation.Invalid {
		t.Fatal("branding payload allowed on polling")
	}
	q.Op = "company.brand-set"
	q.Brand.Logo = strings.Repeat("a", MaxBrandFrame)
	if writeFrameLimit(&frame, q, MaxBrandFrame) == nil {
		t.Fatal("oversize accepted")
	}
	for _, body := range []string{
		`{"version":1,"contract_revision":7,"request_id":"bad","op":"access.check-issued","password":""}`,
		`{"version":1,"contract_revision":7,"request_id":"bad","op":"company.brand-set","expected_profile":"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa","expected_activation":"a","brand":{"name":"","name":"x","accent":"teal","logo":""}}`,
		`{"version":1,"contract_revision":7,"request_id":"bad","op":"company.brand-set","expected_profile":"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa","expected_activation":"a","brand":{"name":null,"accent":"teal","logo":""}}`,
	} {
		if _, c := validate([]byte(body)); c != foundation.Invalid {
			t.Fatal("non-nominal field accepted")
		}
	}
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], (1<<31)|1)
	if _, e = readFrameLimit(bytes.NewReader(append(h[:], byte(1))), MaxBrandFrame); e == nil {
		t.Fatal("small extended frame accepted")
	}
	binary.BigEndian.PutUint32(h[:], MaxFrame+1)
	if _, e = readFrameLimit(bytes.NewReader(h[:]), MaxBrandFrame); e == nil {
		t.Fatal("unflagged oversized frame accepted")
	}
	binary.BigEndian.PutUint32(h[:], (1<<31)|MaxBrandFrame+1)
	if _, e = readFrameLimit(bytes.NewReader(h[:]), MaxBrandFrame); e == nil {
		t.Fatal("extended oversized frame accepted")
	}
}
