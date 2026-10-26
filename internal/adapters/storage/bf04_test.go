package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
	"strings"
	"testing"
	"time"
)

func testLogo(t *testing.T, max bool) string {
	t.Helper()
	var buf bytes.Buffer
	if e := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	raw := buf.Bytes()
	if max {
		raw = append(raw, make([]byte, access.MaxLogoBytes-len(raw))...)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
}
func TestBF04BrandIsolationAuthorizationAndReopen(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "Same", access.OK)
	j.create(companyY, "Same", access.OK)
	j.selectCompany(companyX, access.OK)
	before, _ := j.st.Read(context.Background())
	q := j.q
	q.Op = "company.brand-set"
	q.ExpectedProfile = companyX
	q.ExpectedActivation = q.Activation
	q.Brand = access.Branding{Name: "Visual X", Logo: testLogo(t, true), Accent: "rose"}
	v := j.call(q, access.OK)
	if v.Brand == nil || *v.Brand != q.Brand {
		t.Fatal("save mismatch")
	}
	stale := q
	j.selectCompany(companyY, access.OK)
	stale.Session = j.q.Session
	stale.Profile = j.q.Profile
	stale.Activation = j.q.Activation
	j.call(stale, access.Changed)
	q = j.q
	q.Op = "company.brand-get"
	q.ExpectedProfile = companyY
	q.ExpectedActivation = q.Activation
	if b := j.call(q, access.OK).Brand; b == nil || b.Name != "" || b.Logo != "" {
		t.Fatal("X leaked into Y")
	}
	q.Op = "company.brand-set"
	q.Brand = access.Branding{Name: "Visual Y", Accent: "blue"}
	j.call(q, access.OK)
	q.Owner = "foreign"
	j.call(q, access.Unauthorized)
	j.selectCompany(companyX, access.OK)
	after, _ := j.st.Read(context.Background())
	if before.Account.Name != after.Account.Name || before.Account.Theme != after.Account.Theme || !bytes.Equal(before.Account.Password.Hash, after.Account.Password.Hash) || !bytes.Equal(before.Account.Recovery, after.Account.Recovery) || len(after.Grants) != len(before.Grants) || after.Companies[0].Name != "Same" {
		t.Fatal("branding changed account/catalog")
	}
	dir := j.st.dir
	j.st.Close()
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	j.st = s
	b, c := s.ReadBrand(context.Background(), companyX)
	if c != access.OK || b.Name != "Visual X" || len(b.Logo) < access.MaxLogoBytes {
		t.Fatal("reopen lost X")
	}
	svc := access.New(s, security.Crypto{}, time.Now, 1000, "new", s)
	login, c := svc.Handle(context.Background(), 1000, access.Command{Op: "access.login", Owner: "new-owner", Name: "technical", Password: password})
	if c != access.OK || login.Profile != companyX {
		t.Fatal("restart lost exact company")
	}
}
func TestBF04BrandValidationAndFailedTransaction(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	base := access.Branding{Name: "Stable", Logo: testLogo(t, false), Accent: "teal"}
	if c := j.st.SaveBrand(context.Background(), companyX, base); c != access.OK {
		t.Fatal(c)
	}
	for _, b := range []access.Branding{{Name: strings.Repeat("x", 49), Accent: "teal"}, {Name: "x\n", Accent: "teal"}, {Accent: "free-color"}, {Accent: "teal", Logo: "data:image/svg+xml;base64,PHN2Zy8+"}, {Accent: "teal", Logo: "data:image/png;base64,eA=="}, {Accent: "teal", Logo: "https://remote/image.png"}, {Accent: "teal", Logo: "data:image/png;base64," + base64.StdEncoding.EncodeToString(make([]byte, access.MaxLogoBytes+1))}} {
		if j.st.SaveBrand(context.Background(), companyX, b) == access.OK {
			t.Fatal("invalid brand committed")
		}
	}
	_, e := j.st.unit.Exec("CREATE TRIGGER deny_brand BEFORE UPDATE ON branding BEGIN SELECT RAISE(ABORT,'test'); END;")
	if e != nil {
		t.Fatal(e)
	}
	if j.st.SaveBrand(context.Background(), companyX, access.Branding{Name: "Lost", Accent: "blue"}) != access.Unavailable {
		t.Fatal("failed transaction accepted")
	}
	got, c := j.st.ReadBrand(context.Background(), companyX)
	if c != access.OK || got != base {
		t.Fatal("partial brand update")
	}
}
func TestBF04EnterpriseV1MigrationAtomicAndChecksum(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "migrate", true: "rollback"}[fail], func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			dropPPPoEFixtureSchema(t, j)
			_, e := j.st.unit.Exec("DROP TABLE devices; DROP TABLE credentials; DROP TABLE vault_key; DROP TABLE equipment_operations; DROP TABLE branding; DELETE FROM schema_migrations WHERE version>=2; CREATE TABLE preserve(value TEXT); INSERT INTO preserve VALUES('old-data');")
			if e != nil {
				t.Fatal(e)
			}
			if fail {
				if _, e = j.st.unit.Exec("CREATE TABLE branding(block TEXT)"); e != nil {
					t.Fatal(e)
				}
			}
			err := j.st.validateUnit(j.st.unit, companyX)
			var count int
			j.st.unit.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count)
			if fail {
				if err == nil || count != 1 {
					t.Fatal("partial migration ledger")
				}
				j.st.unit.Exec("DROP TABLE branding")
				if e = j.st.validateUnit(j.st.unit, companyX); e != nil {
					t.Fatal(e)
				}
			} else if err != nil || count != 6 {
				t.Fatal("migration failed", err)
			}
			var old string
			j.st.unit.QueryRow("SELECT value FROM preserve").Scan(&old)
			if old != "old-data" {
				t.Fatal("legacy data changed")
			}
			var h string
			j.st.unit.QueryRow("SELECT checksum FROM schema_migrations WHERE version=1").Scan(&h)
			if h != checksum([]byte(companySchema)) {
				t.Fatal("schema1 changed")
			}
			j.st.unit.Exec("UPDATE schema_migrations SET checksum='bad' WHERE version=1")
			if j.st.validateUnit(j.st.unit, companyX) == nil {
				t.Fatal("checksum corruption accepted")
			}
		})
	}
}

func TestBF04WebPFullDecode(t *testing.T) {
	valid, _ := base64.StdEncoding.DecodeString("UklGRjgAAABXRUJQVlA4ICwAAACQAQCdASoCAAIAAUAmJaACdLoAA5gA/vBTH+j64RZGX/7Gb/56P/z0f6hgAA==")
	if !validWebP(valid) {
		t.Fatal("valid WebP rejected")
	}
	for _, raw := range [][]byte{valid[:len(valid)-1], append([]byte("RIFF"), []byte{18, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', 'L', 5, 0, 0, 0, 0x2f, 0, 0, 0, 0, 0}...)} {
		if validWebP(raw) {
			t.Fatal("incomplete WebP accepted")
		}
	}
}
