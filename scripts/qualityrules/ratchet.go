package qualityrules

import "fmt"

// ExistingTSWithoutBaseline distinguishes historical debt from a new source file.
func ExistingTSWithoutBaseline(inHead, baseline bool) bool { return inHead && !baseline }

func RatchetGrowth(current, previous map[string]map[string]int) []string {
	var failures []string
	for path, rules := range current {
		for rule, ceiling := range rules {
			old, exists := previous[path][rule]
			if !exists || ceiling > old {
				failures = append(failures, fmt.Sprintf("%s: ratchet %s grew or is new", path, rule))
			}
		}
	}
	return failures
}
