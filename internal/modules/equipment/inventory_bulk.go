package equipment

type BulkItem struct {
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
}
type BulkUpdate struct {
	Items        []BulkItem `json:"items"`
	Block        *string    `json:"block,omitempty"`
	CredentialID *string    `json:"credential_id,omitempty"`
}
type BulkReceipt struct {
	Items []BulkItem `json:"items"`
}
