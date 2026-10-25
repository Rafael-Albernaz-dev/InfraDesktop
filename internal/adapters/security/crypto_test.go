package security

import (
	"infradesktop/internal/modules/access"
	"testing"
	"time"
)

func TestArgon2(t *testing.T) {
	c := Crypto{}
	began := time.Now()
	v, code := c.Hash("  senha unicode de teste ç  ")
	if code != access.OK {
		t.Fatal(code)
	}
	t.Logf("Argon2id local m=19456KiB t=2 p=1 hash duration=%s; not a 4GB benchmark", time.Since(began))
	if len(v.Salt) != 16 || len(v.Hash) != 32 || !c.Verify("  senha unicode de teste ç  ", v) || c.Verify("senha unicode de teste ç", v) {
		t.Fatal("KDF roundtrip/whitespace policy")
	}
	other, _ := c.Hash("  senha unicode de teste ç  ")
	if c.Equal(v.Salt, other.Salt) || c.Equal(v.Hash, other.Hash) {
		t.Fatal("salt reused")
	}
	v.Memory = 1 << 31
	if c.Verify("  senha unicode de teste ç  ", v) {
		t.Fatal("unsafe parameters accepted")
	}
	token, _ := c.Token()
	if len(token) != 64 || len(c.Digest(token)) != 32 || !c.Equal(c.Digest(token), c.Digest(token)) {
		t.Fatal("token entropy/verification format")
	}
}
