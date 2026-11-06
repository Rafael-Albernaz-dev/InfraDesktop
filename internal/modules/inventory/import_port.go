package inventory

// ImportRow is an untrusted file row; validation and credential resolution happen later.
type ImportRow struct {
	Line       int    `json:"line"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Port       string `json:"port"`
	Role       string `json:"role"`
	Block      string `json:"block"`
	Credential string `json:"credential"`
}

// ImportParser reads an untrusted file without accessing inventory or the vault.
type ImportParser interface {
	Parse(name string, data []byte) ([]ImportRow, error)
}
