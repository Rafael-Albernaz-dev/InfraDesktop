package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"infradesktop/internal/modules/equipment"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func testVault() *Vault {
	return New(func() ([]byte, error) { return bytes.Repeat([]byte{42}, 32), nil })
}

type brokenRandom struct{}

func (brokenRandom) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func TestEnvelopeAndTampering(t *testing.T) {
	v := testVault()
	b, w, c := v.Seal("install", "X", "cred", 1, "fixture secret", nil)
	if c != equipment.OK {
		t.Fatal(c)
	}
	if s, c := v.Open("install", "X", "cred", 1, b, w); c != equipment.OK || s != "fixture secret" {
		t.Fatal("roundtrip failed")
	}
	b2, _, c := v.Seal("install", "X", "cred", 1, "fixture secret", w)
	if c != equipment.OK || bytes.Equal(b, b2) {
		t.Fatal("nonce reused")
	}
	for _, mutate := range []func(*envelope){func(e *envelope) { e.Version++ }, func(e *envelope) { e.Algorithm = "other" }, func(e *envelope) { e.KeyID = "ffffffffffffffffffffffffffffffff" }, func(e *envelope) { e.Nonce[0] ^= 1 }, func(e *envelope) { e.Nonce = e.Nonce[:11] }, func(e *envelope) { e.Ciphertext[len(e.Ciphertext)-1] ^= 1 }} {
		var env envelope
		_ = json.Unmarshal(b, &env)
		mutate(&env)
		bad, _ := json.Marshal(env)
		if _, c := v.Open("install", "X", "cred", 1, bad, w); c != equipment.VaultUnavailable {
			t.Fatal("tampered envelope accepted")
		}
	}
	for _, args := range []struct {
		i, c, id string
		r        uint64
	}{{"other", "X", "cred", 1}, {"install", "Y", "cred", 1}, {"install", "X", "other", 1}, {"install", "X", "cred", 2}} {
		if _, c := v.Open(args.i, args.c, args.id, args.r, b, w); c != equipment.VaultUnavailable {
			t.Fatal("AAD ignored")
		}
	}
	var env envelope
	_ = json.Unmarshal(w, &env)
	env.Ciphertext[0] ^= 1
	bad, _ := json.Marshal(env)
	if _, c := v.Open("install", "X", "cred", 1, b, bad); c != equipment.VaultUnavailable {
		t.Fatal("wrapped key tamper accepted")
	}
	wrong := New(func() ([]byte, error) { return bytes.Repeat([]byte{43}, 32), nil })
	if _, c := wrong.Open("install", "X", "cred", 1, b, w); c != equipment.VaultUnavailable {
		t.Fatal("wrong master accepted")
	}
	v.Random = brokenRandom{}
	if _, _, c := v.Seal("install", "X", "cred", 2, "replacement", w); c != equipment.VaultUnavailable {
		t.Fatal("RNG error ignored")
	}
	if _, _, c := v.Seal("install", "Y", "cred", 1, "replacement", nil); c != equipment.VaultUnavailable {
		t.Fatal("key RNG error ignored")
	}
	if s, c := v.Open("install", "X", "cred", 1, b, w); c != equipment.OK || s != "fixture secret" {
		t.Fatal("failed replacement destroyed old envelope")
	}
	missing := New(func() ([]byte, error) { return nil, errors.New("missing") })
	if missing.Available() {
		t.Fatal("missing key available")
	}
	if _, _, c := missing.Seal("install", "X", "cred", 1, "secret", w); c != equipment.VaultUnavailable {
		t.Fatal("missing master replaced")
	}
	duplicate := bytes.Replace(b, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1)
	if _, c := v.Open("install", "X", "cred", 1, duplicate, w); c != equipment.VaultUnavailable {
		t.Fatal("duplicate envelope field accepted")
	}
	if bytes.Contains(b, []byte("fixture secret")) || bytes.Contains(w, bytes.Repeat([]byte{42}, 32)) {
		t.Fatal("plaintext envelope")
	}
}
func TestProviderDEV(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, CredentialName)
	key := bytes.Repeat([]byte{21}, 32)
	if e := os.WriteFile(p, key, 0600); e != nil {
		t.Fatal(e)
	}
	if b, e := ReadKey(p); e != nil || !bytes.Equal(b, key) {
		t.Fatal("runtime credential unreadable")
	}
	for _, mode := range []os.FileMode{0644, 0660} {
		os.Chmod(p, mode)
		if _, e := ReadKey(p); e == nil {
			t.Fatal("exposed key accepted")
		}
	}
	os.Chmod(p, 0600)
	os.Remove(p)
	os.Symlink("other", p)
	if _, e := ReadKey(p); e == nil {
		t.Fatal("symlink accepted")
	}
	os.Remove(p)
	os.WriteFile(p, []byte("short"), 0600)
	if _, e := ReadKey(p); e == nil {
		t.Fatal("corrupt key accepted")
	}
	os.Remove(p)
	if _, e := ReadKey(p); e == nil {
		t.Fatal("missing key generated")
	}
}
