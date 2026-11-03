package ipc

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"infradesktop/internal/modules/foundation"
	"strings"
	"testing"
)

func TestBrandFrameLimitsAndNominalFields(t *testing.T) {
	q := Request{
		Version: 1, ContractRevision: ProductRevision, RequestID: "brand", Op: "company.brand-set",
		ExpectedProfile: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", ExpectedActivation: "activation",
		Brand: &Branding{Accent: "teal", Logo: strings.Repeat("a", 2796226)},
	}
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
	brand := `"expected_profile":"aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa","expected_activation":"a","brand":`
	for _, q := range [][2]string{
		{"access.check-issued", `"password":""`},
		{"company.brand-set", brand + `{"name":"","name":"x","accent":"teal","logo":""}`},
		{"company.brand-set", brand + `{"name":null,"accent":"teal","logo":""}`},
	} {
		if _, c := validate(wire(q[0], q[1])); c != foundation.Invalid {
			t.Fatal("non-nominal field accepted", q)
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
