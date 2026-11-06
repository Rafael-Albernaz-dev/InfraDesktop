package inventory

import "testing"

func TestUnchangedUpdateKeepsPreviousRevisionSemantics(t *testing.T) {
	old := Device{ID: "00000000-0000-4000-8000-000000000001", Name: "Sintetico", Revision: 7}
	next := old
	if code := prepareDeviceUpdate(&old, Command{Op: "inventory.update", Revision: 7}, &next); code != OK {
		t.Fatal(code)
	}
	if next.Revision != 8 {
		t.Fatalf("unchanged edit revision = %d, want 8", next.Revision)
	}
}
