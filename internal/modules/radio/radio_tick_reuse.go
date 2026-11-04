package radio

import "strings"

func radioTickNeedsReopen(reading *RadioReading, code Code) bool {
	// A framed AP data diagnostic says nothing about SSH transport health.
	if code == Partial && reading != nil && reading.Failure != nil && strings.HasPrefix(reading.Failure.Reason, "tick-") {
		return false
	}
	return code != OK || (reading != nil && reading.CommandSeconds > 3)
}
