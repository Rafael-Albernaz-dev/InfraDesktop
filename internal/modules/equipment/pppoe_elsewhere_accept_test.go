package equipment

import "testing"

const (
	saiu   = "2026-11-06T10:00:00Z"
	antes  = "2026-11-06T09:59:59Z"
	depois = "2026-11-06T10:00:01Z"
)

func leitura() *PPObservation {
	return &PPObservation{
		Online: 1, Offline: 2, Count: 3,
		Rows: []PPRow{
			{Login: "fica", State: "ONLINE"},
			{Login: "mudou", State: "OFFLINE", AbsentStart: saiu},
			{Login: "caiu", State: "OFFLINE", AbsentStart: saiu},
		},
	}
}

func logins(o *PPObservation) []string {
	out := []string{}
	for _, r := range o.Rows {
		out = append(out, r.Login)
	}
	return out
}

func TestOfflineLeavesWhenConnectedElsewhereAfterLeaving(t *testing.T) {
	o := leitura()
	if got := DropOfflineSeenElsewhere(o, map[string]string{"mudou": depois}); got != 1 {
		t.Fatalf("removed = %d", got)
	}
	if len(o.Rows) != 2 || o.Rows[0].Login != "fica" || o.Rows[1].Login != "caiu" {
		t.Fatalf("rows = %v", logins(o))
	}
	if o.Offline != 1 || o.Count != 2 || o.Online != 1 {
		t.Fatalf("offline=%d count=%d online=%d", o.Offline, o.Count, o.Online)
	}
}

func TestReadingAtTheSameMomentCounts(t *testing.T) {
	o := leitura()
	if DropOfflineSeenElsewhere(o, map[string]string{"mudou": saiu}) != 1 {
		t.Fatalf("rows = %v", logins(o))
	}
}

func TestOlderReadingElsewhereProvesNothing(t *testing.T) {
	o := leitura()
	if got := DropOfflineSeenElsewhere(o, map[string]string{"mudou": antes}); got != 0 || o.Offline != 2 {
		t.Fatalf("removed=%d rows=%v", got, logins(o))
	}
}

func TestOnlineRowIsNeverRemoved(t *testing.T) {
	o := leitura()
	o.Rows[0].AbsentStart = saiu
	if got := DropOfflineSeenElsewhere(o, map[string]string{"fica": depois}); got != 0 || len(o.Rows) != 3 {
		t.Fatalf("removed=%d rows=%v", got, logins(o))
	}
}

func TestBadTimeKeepsTheRow(t *testing.T) {
	for _, seen := range []string{"", "ontem"} {
		o := leitura()
		if got := DropOfflineSeenElsewhere(o, map[string]string{"mudou": seen}); got != 0 {
			t.Fatalf("seen %q removed %d", seen, got)
		}
	}
	o := leitura()
	o.Rows[1].AbsentStart = "sem-hora"
	if got := DropOfflineSeenElsewhere(o, map[string]string{"mudou": depois}); got != 0 {
		t.Fatalf("bad start removed %d", got)
	}
}

func TestEmptyLoginAndEmptyMapChangeNothing(t *testing.T) {
	o := leitura()
	o.Rows[1].Login = ""
	if got := DropOfflineSeenElsewhere(o, map[string]string{"": depois}); got != 0 {
		t.Fatalf("empty login removed %d", got)
	}
	o = leitura()
	if got := DropOfflineSeenElsewhere(o, nil); got != 0 || len(o.Rows) != 3 {
		t.Fatalf("nil map removed %d", got)
	}
}

func TestDifferentTimeZoneStillCompares(t *testing.T) {
	o := leitura()
	if DropOfflineSeenElsewhere(o, map[string]string{"mudou": "2026-11-06T07:00:01-03:00"}) != 1 {
		t.Fatalf("rows = %v", logins(o))
	}
}
