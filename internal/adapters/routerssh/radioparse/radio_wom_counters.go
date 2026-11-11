package radioparse

import (
	"regexp"
	"strconv"
	"strings"
)

func radioWOMCounters(text string) map[string]string {
	out := map[string]string{}
	if m := regexp.MustCompile(`(?m)^([0-9]+)(?:\.[0-9]+)? [0-9]+(?:\.[0-9]+)?\s*$`).FindStringSubmatch(text); len(m) == 2 {
		out["uptime_seconds"] = m[1]
	}
	for _, m := range regexp.MustCompile(("(?m)^\\s*(rx bytes|tx bytes|rx packets|tx packets|tx retries|tx failed):\\s*([" +
		"0-9]+)\\s*$")).FindAllStringSubmatch(text,
		-1) {
		if _, e := strconv.ParseUint(m[2], 10, 64); e == nil {
			out[strings.ReplaceAll(m[1], " ", "_")] = m[2]
		}
	}
	return out
}
