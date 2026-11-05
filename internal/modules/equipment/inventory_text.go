package equipment

import (
	"net/netip"
	"strings"
	"unicode"
	"unicode/utf8"
)

func ValidText(s string, max int) bool {
	if strings.TrimSpace(s) != s || !utf8.ValidString(s) {
		return false
	}
	n := utf8.RuneCountInString(s)
	if n < 1 || n > max {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// ValidBlock accepts empty or bounded text without control characters.
func ValidBlock(value string) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 80 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validateDeviceEndpoint(d Device) Code {
	ip, err := netip.ParseAddr(d.Address)
	if err != nil || ip.Zone() != "" || ip.String() != d.Address || ip.IsUnspecified() || ip.IsMulticast() {
		return InvalidDeviceAddress
	}
	if d.Port < 1 || d.Port > 65535 {
		return InvalidDevicePort
	}
	return OK
}
