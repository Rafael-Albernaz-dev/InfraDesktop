package radioparse

import "strings"

var radioStationMarkers = []string{
	"L07-AVULSO-RESOURCE", "L07-AVULSO-PACKAGES", "L07-AVULSO-BOARD",
	"L07-AVULSO-HEALTH", "L07-AVULSO-WIRELESS", "L07-AVULSO-REGISTRATION", "L07-AVULSO-DONE",
}

// Complete marker lines delimit sections; a partial marker is never accepted as a boundary.
func radioMarkerSections(text string, markers []string) ([]string, bool) {
	if len(markers) < 2 {
		return nil, false
	}
	rest := strings.TrimSpace(strings.ReplaceAll(text, "\r", ""))
	rest, ok := strings.CutPrefix(rest, markers[0]+"\n")
	if !ok {
		return nil, false
	}
	parts := make([]string, 0, len(markers)-1)
	for i, marker := range markers[1:] {
		before, after, found := strings.Cut(rest, "\n"+marker)
		if !found {
			return nil, false
		}
		if i < len(markers)-2 && !strings.HasPrefix(after, "\n") {
			return nil, false
		}
		parts = append(parts, before)
		rest = strings.TrimPrefix(after, "\n")
	}
	return parts, strings.TrimSpace(rest) == ""
}
