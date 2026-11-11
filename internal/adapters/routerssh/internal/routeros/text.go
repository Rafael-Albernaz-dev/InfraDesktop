package routeros

import (
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func CounterString(value string) *string {
	n := Counter(value)
	if n == nil {
		return nil
	}
	s := strconv.FormatUint(*n, 10)
	return &s
}

func LogTime(value string) string {
	if len(value) > 64 {
		return ""
	}
	for _, layout := range []string{"15:04:05", "Jan/02/2006 15:04:05", "Jan/02 15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		// ignored: only timestamp validity matters; preserve the original display token
		if _, e := time.Parse(layout, value); e == nil {
			return value
		}
	}
	return ""
}

func Clean(s string, max int) bool {
	if !utf8.ValidString(s) || len(s) < 1 || len(s) > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func Lines(s string) []string {
	return strings.Split(strings.TrimSuffix(strings.ReplaceAll(s, "\r\n", "\n"), "\n"), "\n")
}
