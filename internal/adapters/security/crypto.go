// Package security adapts cryptographic libraries to account policy ports.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"golang.org/x/crypto/argon2"
	"infradesktop/internal/modules/access"
)

type Crypto struct{}

func (Crypto) Hash(password string) (access.Verifier, access.Code) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return access.Verifier{}, access.Unavailable
	}
	v := access.Verifier{Algorithm: "argon2id-v19", Memory: 19456, Iterations: 2, Parallelism: 1, Salt: salt}
	v.Hash = argon2.IDKey([]byte(password), salt, v.Iterations, v.Memory, v.Parallelism, 32)
	return v, access.OK
}
func (Crypto) Verify(password string, v access.Verifier) bool {
	// Bound every persisted parameter before invoking the allocation-heavy KDF.
	if v.Algorithm != "argon2id-v19" || v.Memory != 19456 || v.Iterations != 2 || v.Parallelism != 1 || len(v.Salt) != 16 || len(v.Hash) != 32 {
		return false
	}
	h := argon2.IDKey([]byte(password), v.Salt, v.Iterations, v.Memory, v.Parallelism, 32)
	return subtle.ConstantTimeCompare(h, v.Hash) == 1
}
func (Crypto) Token() (string, access.Code) {
	b := make([]byte, 32)
	if _, e := rand.Read(b); e != nil {
		return "", access.Unavailable
	}
	return hex.EncodeToString(b), access.OK
}
func (Crypto) Digest(v string) []byte { h := sha256.Sum256([]byte(v)); return h[:] }
func (Crypto) Equal(a, b []byte) bool { return subtle.ConstantTimeCompare(a, b) == 1 }
