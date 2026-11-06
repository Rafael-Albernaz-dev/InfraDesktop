package inventory

import (
	"errors"
	"infradesktop/internal/modules/equipment"
)

// ImportRow is an untrusted file row; the service validates it again before writing.
type ImportRow = equipment.ImportRow

// ImportParser reads an untrusted file without accessing inventory or the vault.
type ImportParser interface {
	Parse(name string, data []byte) ([]ImportRow, error)
}

// ErrImportSecret identifies a rejected password column without exposing its value.
var ErrImportSecret = errors.New("password field rejected")

// ImportDigester creates a stable receipt fingerprint without depending on a wire encoding.
type ImportDigester interface {
	Digest(rows []ImportRow) (string, error)
}
