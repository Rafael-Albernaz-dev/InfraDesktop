package equipment

import (
	"fmt"
	"testing"
	"time"
)

func l06cObservation(id string, at time.Time, rows ...PPRow) *PPObservation {
	return &PPObservation{ID: id, DeviceID: "d", Scope: PPScope, Quality: PPQuality{Status: "complete", Terminated: true, MembershipChecked: true}, Provenance: PPProvenance{Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1", Connection: "c", Family: "v7", Version: "7.14.3", Started: PPTime(at), Ended: PPTime(at.Add(time.Second))}, Rows: rows}
}

func TestL06COfflineReconnectionRetention(t *testing.T) {
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	row := PPRow{ID: "a", RemoteID: "*1", SessionID: "s1", Login: "reader", Address: "198.51.100.1", Caller: "caller", Uptime: PPInterval{Min: 3600, Max: 3601, Known: true}}
	a := l06cObservation("a", at, row)
	if c := ReconcilePP(nil, a); c != OK {
		t.Fatal(c)
	}
	a.ContextOmitted = true
	b := l06cObservation("b", at.Add(time.Minute))
	if c := ReconcilePP(a, b); c != OK || b.Online != 0 || b.Offline != 1 || !b.ContextOmitted || b.Rows[0].State != "OFFLINE" || b.Rows[0].Address != row.Address || b.Rows[0].AbsentStart != a.Provenance.Started || b.Rows[0].AbsentEnd != b.Provenance.Ended {
		t.Fatal("absence lost last data/window", b, c)
	}
	if a.Rows[0].State != "ONLINE" {
		t.Fatal("reference input mutated")
	}
	partial := l06cObservation("partial", at.Add(2*time.Minute))
	partial.Quality.Terminated = false
	if ReconcilePP(b, partial) != Partial || len(partial.Rows) != 0 {
		t.Fatal("partial reading reconciled absence")
	}
	later := l06cObservation("later", at.Add(time.Hour))
	ReconcilePP(b, later)
	if later.Rows[0].AbsentEnd != b.Rows[0].AbsentEnd {
		t.Fatal("absence expiration renewed")
	}
	reconnected := row
	reconnected.ID = "new-row"
	reconnected.RemoteID = "*2"
	reconnected.SessionID = "s2"
	online := l06cObservation("online", at.Add(2*time.Hour), reconnected)
	ReconcilePP(later, online)
	if online.Offline != 0 || online.Online != 1 || online.ContextOmitted || online.Rows[0].ID != "new-row" {
		t.Fatal("reconnection retained offline identity", online)
	}
	ds, c := ComparePP(b, online)
	if c != OK || len(ds) != 1 || ds[0].Reason != "reconnected" || ds[0].Base.Login != "reader" || ds[0].Target.Caller != "caller" {
		t.Fatal("reconnection projection", ds, c)
	}
	expired := l06cObservation("expired", at.Add(time.Minute+24*time.Hour))
	ReconcilePP(b, expired)
	if expired.Offline != 0 {
		t.Fatal("24h retention exceeded")
	}
	full := l06cObservation("full", at.Add(time.Hour))
	for i := 0; i < PPMaxRows; i++ {
		full.Rows = append(full.Rows, PPRow{ID: fmt.Sprint(i), RemoteID: fmt.Sprint(i), Login: fmt.Sprint(i)})
	}
	ReconcilePP(b, full)
	if full.Online != PPMaxRows || full.Offline != 0 {
		t.Fatal("online displaced by offline")
	}
	incompatible := l06cObservation("other", at.Add(time.Hour))
	incompatible.Provenance.Connection = "other"
	ReconcilePP(b, incompatible)
	if len(incompatible.Rows) != 0 {
		t.Fatal("mixed connection contexts")
	}
}

func TestL06CSiblingSessionDoesNotHideAbsence(t *testing.T) {
	at := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	x := PPRow{ID: "x", RemoteID: "*1", SessionID: "1", Login: "same"}
	y := PPRow{ID: "y", RemoteID: "*2", SessionID: "2", Login: "same"}
	a := l06cObservation("a", at, x, y)
	ReconcilePP(nil, a)
	b := l06cObservation("b", at.Add(time.Minute), y)
	ReconcilePP(a, b)
	if b.Online != 1 || b.Offline != 1 || b.Rows[1].ID != "x" || b.Rows[1].State != "OFFLINE" || b.Rows[0].Duplicates != 1 {
		t.Fatal("surviving sibling hides missing identity", b)
	}
	c := l06cObservation("c", at.Add(2*time.Minute), y)
	ReconcilePP(b, c)
	if c.Offline != 1 {
		t.Fatal("continuing sibling erased retained absence", c)
	}
	z := PPRow{ID: "z", RemoteID: "*3", SessionID: "3", Login: "same"}
	d := l06cObservation("d", at.Add(3*time.Minute), y, z)
	ReconcilePP(c, d)
	if d.Online != 2 || d.Offline != 0 {
		t.Fatal("new identity did not clear prior offline login", d)
	}
}

func TestL06CPingBoundsAfterLongDelay(t *testing.T) {
	seconds := func(n int64) PPInterval {
		return PPInterval{Min: n * int64(time.Second), Max: (n+1)*int64(time.Second) - 1, Known: true}
	}
	target := &PPPingTarget{Observation: PPObservation{Provenance: PPProvenance{RouterUptime: seconds(10000)}}, Row: PPRow{State: "ONLINE", Uptime: seconds(1000)}, Age: PPInterval{Known: true, Min: int64(120 * time.Second), Max: int64(121 * time.Second)}}
	router, session, age, ok := PPPingBounds(target, 3*time.Second)
	if !ok || age.Min != int64(123*time.Second) || age.Max != int64(144*time.Second) || router.Min != int64(10123*time.Second) || router.Max != int64(10145*time.Second)-1 || session.Min != int64(1123*time.Second) || session.Max != int64(1145*time.Second)-1 {
		t.Fatal("long delay lost exact conservative bounds", router, session, age, ok)
	}
	target.Row.State = "OFFLINE"
	if _, _, _, ok = PPPingBounds(target, 0); ok {
		t.Fatal("offline can be probed")
	}
}
