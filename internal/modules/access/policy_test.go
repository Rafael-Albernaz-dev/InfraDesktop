package access

import (
	"strings"
	"testing"
	"time"
)

func TestPasswordAndNamePolicy(t *testing.T) {
	for _, x := range []struct {
		v  string
		ok bool
	}{{strings.Repeat("á", 15), true}, {strings.Repeat("界", 128), true}, {strings.Repeat("a", 14), false}, {strings.Repeat("a", 129), false}, {"  espaços valem  ", true}, {string([]byte{0xff}) + strings.Repeat("a", 20), false}} {
		if ValidPassword(x.v) != x.ok {
			t.Errorf("password policy mismatch for rune boundary")
		}
	}
	for _, n := range []string{"", " a", "a\n", strings.Repeat("x", 81)} {
		if ValidName(n) {
			t.Error("invalid name accepted")
		}
	}
	if !ValidName("Empresa ç — Centro") {
		t.Error("unicode company rejected")
	}
	for n := uint32(1); n < 12; n++ {
		d := Backoff(n)
		if d < time.Second || d > 30*time.Second {
			t.Fatal("unbounded lockout")
		}
	}
}
