package access

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func ValidPassword(p string) bool {
	n := utf8.RuneCountInString(p)
	return validExistingPassword(p) && n >= 8
}
func ValidName(n string) bool {
	if !utf8.ValidString(n) || strings.TrimSpace(n) != n || utf8.RuneCountInString(n) < 1 || utf8.RuneCountInString(n) > 80 {
		return false
	}
	for _, r := range n {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func Backoff(failures uint32) time.Duration {
	if failures > 5 {
		return 30 * time.Second
	}
	if failures == 0 {
		return 0
	}
	return time.Second * time.Duration(1<<(failures-1))
}

func admitCredentialInput(q Command) Code {
	if ValidName(q.Name) && passwordForOperation(q.Op, q.Password) {
		return OK
	}
	if q.Op == "access.login" {
		return Unauthorized
	}
	return Invalid
}
