package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"

	"github.com/google/uuid"
)

var elsewhereStart = time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

func elsewherePublish(t *testing.T, j *journey, target equipment.PPTarget, at time.Time, logins ...string) *equipment.PPObservation {
	t.Helper()
	ctx := context.Background()
	job := uuid.NewString()
	in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
	if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "fixture:"+job, job); c != equipment.OK {
		t.Fatal(c)
	}
	o := &equipment.PPObservation{
		ID: uuid.NewString(), DeviceID: target.Device.ID, Scope: equipment.PPScope,
		Quality: equipment.PPQuality{Status: "complete", Terminated: true, MembershipChecked: true},
		Provenance: equipment.PPProvenance{
			Projection: "active-v1", Codec: "pppoe-fields-v1", Capabilities: "base-fields-v1",
			Revision: target.Device.Revision, Connection: equipment.PPConnection(target.Device),
			CredentialRevision: target.Credential.Metadata.Revision,
			Started:            equipment.PPTime(at), Ended: equipment.PPTime(at.Add(time.Second)),
		},
	}
	for _, login := range logins {
		o.Rows = append(o.Rows, equipment.PPRow{ID: uuid.NewString(), RemoteID: "*1", SessionID: "1", Login: login})
	}
	if c := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job}, o); c != equipment.OK {
		t.Fatal(c)
	}
	return o
}

func elsewherePeer(t *testing.T, j *journey, a equipment.PPTarget) equipment.PPTarget {
	t.Helper()
	b := a
	b.Device.ID, b.Device.Name = uuid.NewString(), "Peer fixture"
	b.Device.Role = equipment.RoleConcentrator
	if c := j.st.UpdateEquipment(context.Background(), companyX, func(st *equipment.State) equipment.Code {
		st.Devices[0].Role = equipment.RoleConcentrator
		st.Devices = append(st.Devices, b.Device)
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	return b
}

func elsewhereEditMeta(t *testing.T, j *journey, o *equipment.PPObservation) {
	t.Helper()
	b, err := json.Marshal(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = j.st.unit.Exec("UPDATE pppoe_observations SET meta_json=? WHERE id=?", string(b), o.ID); err != nil {
		t.Fatal(err)
	}
}

func elsewhereCheckViews(t *testing.T, j *journey, o *equipment.PPObservation, want int) {
	t.Helper()
	ctx := context.Background()
	v, c := j.st.PPCurrent(ctx, companyX, o.DeviceID)
	if c != equipment.OK || v.Current.Offline != want || v.Current.Count != want {
		t.Fatal("persisted counters", v.Current, c, want)
	}
	page, c := j.st.PPPage(ctx, companyX, equipment.PPInput{
		DeviceID: o.DeviceID, ObservationID: o.ID, Query: &equipment.PPQuery{State: "OFFLINE"},
	})
	if c != equipment.OK || page.Total != want || len(page.Rows) != want {
		t.Fatal("sessions projection", page, c, want)
	}
	st, c := j.st.ReadEquipment(ctx, companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	for _, d := range st.Devices {
		if d.ID == o.DeviceID {
			if d.PPPoE == nil || d.PPPoE.Offline != want || d.PPPoE.Online != 0 {
				t.Fatal("inventory projection", d.PPPoE, want)
			}
			return
		}
	}
	t.Fatal("origin missing")
}

func TestOfflineElsewhereRequiresValidOnlinePeer(t *testing.T) {
	cases := []string{"later", "equal", "older", "absent", "inactive", "other-role", "incomplete",
		"old-connection", "old-revision", "old-credential", "offline-only", "invalid-end", "single"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			j, a := ppStoreSetup(t)
			if _, err := j.st.unit.Exec("UPDATE devices SET role='concentrator' WHERE id=?", a.Device.ID); err != nil {
				t.Fatal(err)
			}
			elsewherePublish(t, j, a, elsewhereStart, "moved")
			if name != "single" {
				b := elsewherePeer(t, j, a)
				peer := elsewherePublish(t, j, b, elsewhereStart.Add(time.Minute), "moved")
				invalidateElsewherePeer(t, j, peer, name)
			}
			want := 1
			if name == "later" || name == "equal" {
				want = 0
			}
			o := elsewherePublish(t, j, a, elsewhereStart.Add(2*time.Minute))
			elsewhereCheckViews(t, j, o, want)
			// Absence must survive until, but not beyond, the existing 24-hour retention.
			if name == "absent" || name == "single" {
				o = elsewherePublish(t, j, a, elsewhereStart.Add(23*time.Hour))
				elsewhereCheckViews(t, j, o, 1)
				o = elsewherePublish(t, j, a, elsewhereStart.Add(25*time.Hour))
				elsewhereCheckViews(t, j, o, 0)
			}
		})
	}
}

func invalidateElsewherePeer(t *testing.T, j *journey, o *equipment.PPObservation, name string) {
	t.Helper()
	query, value := "", ""
	adjustElsewhereTime(o, name)
	switch name {
	case "inactive":
		query = "UPDATE devices SET active=0 WHERE id=?"
	case "other-role":
		query = "UPDATE devices SET role='ap' WHERE id=?"
	case "incomplete":
		o.Quality.Status = "partial"
	case "old-connection":
		o.Provenance.Connection = "old"
	case "old-revision":
		o.Provenance.Revision++
	case "old-credential":
		o.Provenance.CredentialRevision++
	case "invalid-end":
		o.Provenance.Ended = "bad"
	case "offline-only", "absent":
		row := o.Rows[0]
		row.State = "OFFLINE"
		if name == "absent" {
			row.Login = "someone-else"
		}
		raw, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		query, value = "UPDATE pppoe_rows SET row_json=? WHERE observation_id=?", string(raw)
	}
	if query != "" {
		var err error
		if value != "" {
			_, err = j.st.unit.Exec(query, value, o.ID)
		} else {
			_, err = j.st.unit.Exec(query, o.DeviceID)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	elsewhereEditMeta(t, j, o)
}

func TestOfflineElsewhereUsesLatestPeerAndPreservesOnline(t *testing.T) {
	j, a := ppStoreSetup(t)
	b := elsewherePeer(t, j, a)
	c := elsewherePeer(t, j, a)
	elsewherePublish(t, j, a, elsewhereStart, "moved")
	elsewherePublish(t, j, b, elsewhereStart.Add(time.Minute), "moved")
	elsewherePublish(t, j, c, elsewhereStart.Add(-time.Minute), "moved")
	if _, err := j.st.unit.Exec("UPDATE devices SET name='A' WHERE id=?", b.Device.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := j.st.unit.Exec("UPDATE devices SET name='Z' WHERE id=?", c.Device.ID); err != nil {
		t.Fatal(err)
	}
	removed := elsewherePublish(t, j, a, elsewhereStart.Add(90*time.Second))
	elsewhereCheckViews(t, j, removed, 0)
	online := elsewherePublish(t, j, a, elsewhereStart.Add(2*time.Minute), "moved")
	if online.Online != 1 || online.Count != 1 || online.Rows[0].State != "ONLINE" {
		t.Fatal("online origin removed", online)
	}
	o := elsewherePublish(t, j, a, elsewhereStart.Add(3*time.Minute))
	// A's latest online reading is newer than B: both peer readings are now too old.
	elsewhereCheckViews(t, j, o, 1)
	elsewherePublish(t, j, b, elsewhereStart.Add(4*time.Minute), "moved")
	o = elsewherePublish(t, j, a, elsewhereStart.Add(5*time.Minute))
	elsewhereCheckViews(t, j, o, 0)
}

func TestOfflineElsewhereQueryFailureRollsBackPublication(t *testing.T) {
	for _, failure := range []string{"decode", "query"} {
		t.Run(failure, func(t *testing.T) {
			j, a := ppStoreSetup(t)
			b := elsewherePeer(t, j, a)
			previous := elsewherePublish(t, j, a, elsewhereStart, "moved")
			peer := elsewherePublish(t, j, b, elsewhereStart.Add(time.Minute), "moved")
			if failure == "decode" {
				if _, err := j.st.unit.Exec("UPDATE pppoe_rows SET row_json='[]' WHERE observation_id=?", peer.ID); err != nil {
					t.Fatal(err)
				}
			} else {
				// Only this temporary company DB is affected; the peer SELECT now fails in SQLite.
				if _, err := j.st.unit.Exec("ALTER TABLE credentials RENAME TO synthetic_credentials_unavailable"); err != nil {
					t.Fatal(err)
				}
			}
			candidate := *previous
			candidate.ID, candidate.Rows = uuid.NewString(), nil
			candidate.Provenance.Ended = equipment.PPTime(elsewhereStart.Add(2 * time.Minute))
			code := j.st.ppWrite(context.Background(), companyX, func(tx *sql.Tx) equipment.Code {
				return j.st.persistPPCollection(context.Background(), tx, &candidate)
			})
			if code != equipment.Unavailable {
				t.Fatal("peer error accepted", code)
			}
			cur, code := j.st.PPCurrent(context.Background(), companyX, a.Device.ID)
			if code != equipment.OK || cur.Current.ID != previous.ID {
				t.Fatal("last good replaced", code, cur.Current)
			}
			var count int
			err := j.st.unit.QueryRow("SELECT count(*) FROM pppoe_observations WHERE id=?", candidate.ID).Scan(&count)
			if err != nil || count != 0 {
				t.Fatal("partial candidate persisted", count, err)
			}
		})
	}
}

func adjustElsewhereTime(o *equipment.PPObservation, name string) {
	switch name {
	case "equal":
		o.Provenance.Ended = equipment.PPTime(elsewhereStart)
	case "older":
		o.Provenance.Ended = equipment.PPTime(elsewhereStart.Add(-time.Second))
	}
}
