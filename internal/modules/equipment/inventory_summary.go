package equipment

type PPSummary struct {
	Online  int    `json:"online"`
	Offline int    `json:"offline"`
	At      string `json:"at"`
}
