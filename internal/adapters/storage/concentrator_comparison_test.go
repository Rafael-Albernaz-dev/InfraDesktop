package storage

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func TestL06T3CrossDuplicatesSavedReadingsOnly(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	other := target.Device
	other.ID = uuid.NewString()
	other.Name = "X-peer"
	other.Role = "concentrator"
	if c := j.st.UpdateEquipment(ctx, companyX, func(s *equipment.State) equipment.Code {
		s.Devices[0].Role = "concentrator"
		s.Devices = append(s.Devices, other)
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	peer, c := j.st.ReadPPTarget(ctx, companyX, other.ID, true)
	if c != equipment.OK {
		t.Fatal(c)
	}
	now := time.Now().UTC()
	publish := func(target equipment.PPTarget) *equipment.PPObservation {
		job := uuid.NewString()
		in := equipment.PPInput{DeviceID: target.Device.ID, Revision: target.Device.Revision, Operation: uuid.NewString()}
		if _, _, c := j.st.ReservePPCollection(ctx, companyX, in, "synthetic", job); c != equipment.OK {
			t.Fatal(c)
		}
		o := monitoringObservation(target, now.Add(-5*time.Second), uuid.NewString(), 1)
		o.Provenance.Started = equipment.PPTime(now.Add(-10 * time.Second))
		o.Rows = []equipment.PPRow{{ID: uuid.NewString(), RemoteID: "*1", SessionID: "1", Login: "X", Address: "198.51.100.1"}}
		if c := j.st.FinishPPCollection(ctx, companyX, target, job, equipment.PPAttempt{JobID: job}, o); c != equipment.OK {
			t.Fatal(c)
		}
		return o
	}
	publish(target)
	b := publish(peer)
	v, c := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "duplicates"})
	if c != equipment.OK || len(v.Read.Rows) != 1 || v.Read.Rows[0].Duplicate == nil || v.Read.Rows[0].Login != "X" {
		t.Fatal("overlap duplicates", c, v.Read)
	}

	for _, changed := range []equipment.Device{target.Device, peer.Device} {
		if _, e := j.st.unit.Exec("UPDATE devices SET address='198.51.100.99',revision=revision+1 WHERE id=?", changed.ID); e != nil {
			t.Fatal(e)
		}
		altered, code := j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "duplicates"})
		if code != equipment.OK || len(altered.Read.Rows) != 1 || altered.Read.Rows[0].Duplicate != nil || altered.Read.Rows[0].Value != "não comparável (contexto alterado)" {
			t.Fatal("old reading relabeled with edited endpoint", code, altered.Read)
		}
		if _, e := j.st.unit.Exec("UPDATE devices SET address=?,revision=? WHERE id=?", changed.Address, changed.Revision, changed.ID); e != nil {
			t.Fatal(e)
		}
	}
	b.Provenance.Started = equipment.PPTime(now.Add(-21 * time.Minute))
	b.Provenance.Ended = equipment.PPTime(now.Add(-20 * time.Minute))
	raw, _ := json.Marshal(b)
	if _, e := j.st.unit.Exec("UPDATE pppoe_observations SET meta_json=? WHERE id=?", string(raw), b.ID); e != nil {
		t.Fatal(e)
	}
	v, c = j.st.DeviceMonitoring(ctx, companyX, equipment.PPInput{DeviceID: target.Device.ID, Scope: "duplicates"})
	if c != equipment.OK || len(v.Read.Rows) != 1 || v.Read.Rows[0].Duplicate != nil || v.Read.Rows[0].Value != "não comparável (leituras distantes)" {
		t.Fatal("stale reading accused duplicate", c, v.Read)
	}
}
