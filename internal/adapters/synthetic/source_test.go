package synthetic

import (
	"context"
	"infradesktop/internal/modules/foundation"
	"testing"
)

func TestIndependentOracles(t *testing.T) {
	for _, tc := range []struct {
		label string
		want  int
	}{{"L01", 173}, {"Teste", 517}, {"A", 65}, {"á", 225}} {
		got, c := (Source{}).Read(context.Background(), tc.label)
		if got != tc.want || c != foundation.OK {
			t.Fatalf("%s: %d %s", tc.label, got, c)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, c := (Source{}).Read(ctx, "A"); c != foundation.Deadline {
		t.Fatal(c)
	}
}
