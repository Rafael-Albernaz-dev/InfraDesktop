package equipment

const ImportSecret Code = "IMPORT_PASSWORD_REJECTED"

// ImportRow contains only user-editable fields; it never carries a secret.
type ImportRow struct {
	Line       int    `json:"line"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Port       string `json:"port"`
	Role       string `json:"role"`
	Block      string `json:"block"`
	Credential string `json:"credential"`
	Ignored    bool   `json:"ignored,omitempty"`
}

type ImportInput struct {
	Filename          string      `json:"filename,omitempty"`
	Data              []byte      `json:"data,omitempty"`
	Rows              []ImportRow `json:"rows,omitempty"`
	DefaultBlock      string      `json:"default_block,omitempty"`
	DefaultCredential string      `json:"default_credential,omitempty"`
}

type ImportItem struct {
	Row          ImportRow `json:"row"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason,omitempty"`
	CredentialID string    `json:"credential_id,omitempty"`
}

type ImportReceipt struct {
	Created int `json:"created"`
	Ignored int `json:"ignored"`
}
