package qualityrules

import (
	"encoding/json"
	"os"
)

func FrontendRatchet() []string {
	raw, err := os.ReadFile("frontend/quality-ratchet.json")
	if err != nil {
		return []string{err.Error()}
	}
	var current, previous map[string]map[string]int
	if err := json.Unmarshal(raw, &current); err != nil {
		return []string{err.Error()}
	}
	old, err := gitOutput("show", "HEAD:frontend/quality-ratchet.json")
	if err != nil {
		return nil
	} // Initial ratchet creation is authorized by L-CONTA-A.
	if err := json.Unmarshal(old, &previous); err != nil {
		return []string{err.Error()}
	}
	previous, err = remapHeadKeys(previous, "frontend/")
	if err != nil {
		return []string{err.Error()}
	}
	return RatchetGrowth(current, previous)
}
