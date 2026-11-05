package equipment

import (
	"strings"
	"testing"
)

func TestInventoryBlockBoundaries(t *testing.T) {
	for _, value := range []string{"", strings.Repeat("á", 80)} {
		if !ValidBlock(value) {
			t.Fatalf("valid block rejected: %q", value)
		}
	}
	for _, value := range []string{strings.Repeat("a", 81), "a\nb", string([]byte{0xff})} {
		if ValidBlock(value) {
			t.Fatalf("invalid block accepted: %q", value)
		}
	}
}
