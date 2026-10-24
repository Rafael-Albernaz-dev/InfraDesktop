// Package vault implements authenticated envelope encryption. It never logs keys.
package vault

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"infradesktop/internal/modules/equipment"
	"io"
)

type Provider func() ([]byte, error)
type Vault struct {
	Key    Provider
	Random io.Reader
}
type envelope struct {
	Version    int    `json:"version"`
	Algorithm  string `json:"algorithm"`
	KeyID      string `json:"key_id"`
	Nonce      []byte `json:"nonce"`
	Ciphertext []byte `json:"ciphertext"`
}

func New(key Provider) *Vault { return &Vault{Key: key, Random: rand.Reader} }
func (v *Vault) master() ([]byte, error) {
	if v.Key == nil {
		return nil, errors.New("key unavailable")
	}
	b, e := v.Key()
	if e != nil || len(b) != 32 {
		return nil, errors.New("key unavailable")
	}
	return b, nil
}
func (v *Vault) Available() bool { b, e := v.master(); clear(b); return e == nil }
func (v *Vault) Ready(installation, company string, wrapped []byte) bool {
	if len(wrapped) == 0 {
		return v.Available()
	}
	master, e := v.master()
	if e != nil {
		return false
	}
	defer clear(master)
	key, _, e := v.dataKey(installation, company, wrapped, master)
	clear(key)
	return e == nil
}
func aad(purpose, installation, company, id string, revision uint64) []byte {
	b, _ := json.Marshal([]string{"infradesktop-vault", "1", purpose, installation, company, id, fmt.Sprint(revision)})
	return b
}
func decode(b []byte) (envelope, error) {
	var e envelope
	if len(b) < 1 || len(b) > 4096 {
		return e, errors.New("envelope unavailable")
	}
	tokens := json.NewDecoder(bytes.NewReader(b))
	first, err := tokens.Token()
	if err != nil || first != json.Delim('{') {
		return e, errors.New("envelope unavailable")
	}
	seen := map[string]bool{}
	for tokens.More() {
		name, err := tokens.Token()
		if err != nil {
			return e, err
		}
		k, ok := name.(string)
		if !ok || seen[k] {
			return e, errors.New("envelope unavailable")
		}
		seen[k] = true
		var raw json.RawMessage
		if tokens.Decode(&raw) != nil || bytes.Equal(raw, []byte("null")) {
			return e, errors.New("envelope unavailable")
		}
	}
	if len(seen) != 5 {
		return e, errors.New("envelope unavailable")
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&e) != nil || d.Decode(new(any)) != io.EOF || e.Version != 1 || e.Algorithm != "AES-256-GCM" || len(e.KeyID) != 32 || len(e.Nonce) != 12 || len(e.Ciphertext) < 16 || len(e.Ciphertext) > 1040 {
		return e, errors.New("envelope unavailable")
	}
	return e, nil
}
func (v *Vault) seal(key, plain, aad []byte, id string) ([]byte, error) {
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, g.NonceSize())
	if _, e = io.ReadFull(v.Random, nonce); e != nil {
		return nil, e
	}
	return json.Marshal(envelope{1, "AES-256-GCM", id, nonce, g.Seal(nil, nonce, plain, aad)})
}
func open(key []byte, e envelope, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return g.Open(nil, e.Nonce, e.Ciphertext, aad)
}
func (v *Vault) dataKey(installation, company string, wrapped, master []byte) ([]byte, string, error) {
	e, err := decode(wrapped)
	if err != nil {
		return nil, "", err
	}
	b, err := open(master, e, aad("company-key", installation, company, e.KeyID, 1))
	if err != nil || len(b) != 32 {
		clear(b)
		return nil, "", errors.New("key unavailable")
	}
	return b, e.KeyID, nil
}
func (v *Vault) Seal(installation, company, id string, revision uint64, secret string, wrapped []byte) ([]byte, []byte, equipment.Code) {
	master, err := v.master()
	if err != nil {
		return nil, nil, equipment.VaultUnavailable
	}
	defer clear(master)
	var key []byte
	var keyID string
	if len(wrapped) == 0 {
		key = make([]byte, 32)
		if _, err = io.ReadFull(v.Random, key); err != nil {
			return nil, nil, equipment.VaultUnavailable
		}
		defer clear(key)
		idBytes := make([]byte, 16)
		if _, err = io.ReadFull(v.Random, idBytes); err != nil {
			return nil, nil, equipment.VaultUnavailable
		}
		keyID = hex.EncodeToString(idBytes)
		wrapped, err = v.seal(master, key, aad("company-key", installation, company, keyID, 1), keyID)
	} else {
		key, keyID, err = v.dataKey(installation, company, wrapped, master)
		defer clear(key)
	}
	if err != nil {
		return nil, nil, equipment.VaultUnavailable
	}
	plain := []byte(secret)
	defer clear(plain)
	b, err := v.seal(key, plain, aad("read-credential", installation, company, id, revision), keyID)
	if err != nil {
		return nil, nil, equipment.VaultUnavailable
	}
	return b, wrapped, equipment.OK
}
func (v *Vault) Open(installation, company, id string, revision uint64, b, wrapped []byte) (string, equipment.Code) {
	master, err := v.master()
	if err != nil {
		return "", equipment.VaultUnavailable
	}
	defer clear(master)
	key, keyID, err := v.dataKey(installation, company, wrapped, master)
	if err != nil {
		return "", equipment.VaultUnavailable
	}
	defer clear(key)
	e, err := decode(b)
	if err != nil || e.KeyID != keyID {
		return "", equipment.VaultUnavailable
	}
	plain, err := open(key, e, aad("read-credential", installation, company, id, revision))
	if err != nil {
		return "", equipment.VaultUnavailable
	}
	defer clear(plain)
	return string(plain), equipment.OK
}
func (v *Vault) Stamp(input string) (string, equipment.Code) {
	key, e := v.master()
	if e != nil {
		return "", equipment.VaultUnavailable
	}
	defer clear(key)
	h := hmac.New(sha256.New, key)
	h.Write([]byte("idempotency-v1\x00"))
	h.Write([]byte(input))
	return hex.EncodeToString(h.Sum(nil)), equipment.OK
}
