package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"regexp"
	"strings"
)

// Only complete bounded diagnostic lines are recognized. Projected names/logins
// containing these words, and arbitrary stderr, are never copied into metadata.
var collectorSyntaxLine = regexp.MustCompile(`(?i)^(?:(?:syntax error|expected end of command)(?: \(line [0-9]+ column [0-9]+\))?|bad command name(?: [A-Za-z0-9_-]+)?(?: \(line [0-9]+ column [0-9]+\))?)$`)

func collectorSyntaxDiagnostic(text string) bool {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if len(line) <= 256 && collectorSyntaxLine.MatchString(line) {
			return true
		}
	}
	return false
}
func collectorFailure(code equipment.Code, reason string) (*equipment.PPReadResult, equipment.Code) {
	if equipment.ValidCollectorReason(code, reason) {
		return &equipment.PPReadResult{FailureReason: reason}, code
	}
	return nil, code
}
