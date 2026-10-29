package ipc

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestL07RadioIPCIndependentSidesAndCPERevocation(t *testing.T) {
	ctx := context.Background()
	j := newPPJourney(t, sshfixture.Config{Family: "v6", PPPoE: func() string {
		return "H\t1\t1\t6.49.19 (long-term)\t2d1h1s\nR\t*1\t1\tsynthetic-radio\t127.0.0.1\t58:10:8C:75:E3:08\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
	}})
	if c := j.store.UpdateEquipment(ctx, j.client.profile, func(st *equipment.State) equipment.Code {
		for i := range st.Devices {
			if st.Devices[i].ID == j.device {
				st.Devices[i].Role = "concentrator"
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	observation := j.collect(uuid.NewString())
	rows := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: observation.ID}, foundation.OK).Data.PPPoE.Rows
	if len(rows) != 1 {
		t.Fatal("source row", rows)
	}
	read := func(name string) string {
		t.Helper()
		b, e := os.ReadFile(filepath.Join("..", "routerssh", "testdata", name))
		if e != nil {
			t.Fatal(e)
		}
		return string(b)
	}
	apGolden, womGolden := read("l07_ap_v6_real.txt"), read("l07_wom_iw_real.txt")
	var apFail, cpeFail, holdCPE atomic.Bool
	entered, release := make(chan struct{}, 1), make(chan struct{})
	apPeer, e := sshfixture.Start(sshfixture.Config{Command: func(cmd string) (string, bool) {
		if cmd != routerssh.RadioAPCommand {
			return "", false
		}
		if apFail.Load() {
			return "failure: not enough permissions\n", true
		}
		return apGolden, true
	}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(apPeer.Close)
	cpePeer, e := sshfixture.Start(sshfixture.Config{Command: func(cmd string) (string, bool) {
		if cmd != routerssh.RadioWOMCommand {
			return "", false
		}
		if holdCPE.Load() {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-release
		}
		if cpeFail.Load() {
			return "failure: not enough permissions\n", true
		}
		return womGolden, true
	}})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(cpePeer.Close)
	t.Cleanup(func() { close(release) })
	enroll := func(role string, peer *sshfixture.Server) string {
		t.Helper()
		id := uuid.NewString()
		r := j.client.Call(ctx, Request{Op: "inventory.enroll", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, Equipment: &EquipmentInput{ID: id, Operation: uuid.NewString(), Role: role, Name: "Synthetic " + role, Address: "127.0.0.1", Port: peer.Port(), Transport: "ssh", User: "reader", Secret: sshfixture.Password}})
		if r.Code != foundation.OK {
			t.Fatal("enroll", role, r.Code)
		}
		if c := j.store.UpdateEquipment(ctx, j.client.profile, func(st *equipment.State) equipment.Code {
			for i := range st.Devices {
				if st.Devices[i].ID == id {
					st.Devices[i].Fingerprint = peer.Fingerprint()
					st.Devices[i].Revision = 2
				}
			}
			return equipment.OK
		}); c != equipment.OK {
			t.Fatal(c)
		}
		return id
	}
	ap, cpe := enroll("ap", apPeer), enroll("cpe", cpePeer)
	call := func(op string, in equipment.RadioInput, want foundation.Code) *equipment.RadioView {
		t.Helper()
		r := j.client.Call(ctx, Request{Op: op, ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, Radio: &in})
		if r.Code != want {
			t.Fatalf("%s=%s want %s", op, r.Code, want)
		}
		if r.Data == nil {
			return nil
		}
		return r.Data.Radio
	}
	finish := func(in equipment.RadioInput) *equipment.RadioView {
		t.Helper()
		v := call("radio.start", in, foundation.OK)
		id := v.Job.ID
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			v = call("radio.status", equipment.RadioInput{JobID: id}, foundation.OK)
			if v.Job.State != "RUNNING" {
				return v
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("radio job did not finish")
		return nil
	}
	for _, target := range []struct{ id, family string }{{ap, "routeros-wireless-v6"}, {cpe, "intelbras-wom"}} {
		v := finish(equipment.RadioInput{DeviceID: target.id, Revision: 2, Family: target.family, Mode: "once", Operation: uuid.NewString()})
		if v.Job.Code != equipment.OK || v.Reading == nil {
			t.Fatal("initial radio", v)
		}
		call("radio.confirm-family", equipment.RadioInput{DeviceID: target.id, Revision: 2, Family: target.family, Warning: 15, Critical: 20}, foundation.OK)
	}
	choice := &equipment.RadioLinkChoice{AP: ap, CPE: cpe, MAC: "02:00:00:00:10:03", Interface: "wlan1", BSSID: "02:00:00:00:10:05", Evidence: "synthetic confirmed MAC association", Origin: "manual"}
	input := equipment.RadioInput{DeviceID: j.device, ObservationID: observation.ID, RowID: rows[0].ID, Revision: 2, Mode: "once", Operation: uuid.NewString()}
	call("radio.link-confirm", equipment.RadioInput{DeviceID: j.device, ObservationID: observation.ID, RowID: rows[0].ID, Operation: uuid.NewString(), Link: choice}, foundation.OK)
	apFail.Store(true)
	v := finish(input)
	if v.Job.Code != equipment.Partial || v.APCode != "REMOTE_PERMISSION_DENIED" || v.CPECode != equipment.OK || v.CPEReading == nil {
		t.Fatal("AP failure erased independently pinned CPE", v)
	}
	apFail.Store(false)
	cpeFail.Store(true)
	input.Operation = uuid.NewString()
	v = finish(input)
	if v.Job.Code != equipment.Partial || v.APCode != equipment.OK || v.Reading == nil || len(v.Reading.Clients) != 1 || v.CPECode != "REMOTE_PERMISSION_DENIED" {
		t.Fatal("CPE failure erased AP or exposed other clients", v)
	}
	cpeFail.Store(false)
	holdCPE.Store(true)
	input.Operation = uuid.NewString()
	job := call("radio.start", input, foundation.OK).Job.ID
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("CPE command not entered")
	}
	r := j.client.Call(ctx, Request{Op: "credential.put", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, Equipment: &EquipmentInput{ID: cpe, Revision: 1, Operation: uuid.NewString(), Alias: "Synthetic CPE", User: "reader", Secret: "replacement-synthetic-only", Affected: 1}})
	if r.Code != foundation.OK {
		t.Fatal("credential revoke", r.Code)
	}
	until := time.Now().Add(time.Second)
	for time.Now().Before(until) {
		v = call("radio.status", equipment.RadioInput{JobID: job}, foundation.OK)
		if v.Job.State == "CANCELLED" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if v.Job.State != "CANCELLED" {
		t.Fatal("CPE credential did not cancel the link job", v.Job)
	}
	current := call("radio.current", equipment.RadioInput{DeviceID: cpe}, foundation.OK)
	if current.Profile.Confirmed || current.Reading.Compatible {
		t.Fatal("revoked CPE context remained current", current)
	}
	before := cpePeer.Auth.Load()
	input.Operation = uuid.NewString()
	v = finish(input)
	if cpePeer.Auth.Load() != before || v.CPECode == equipment.OK {
		t.Fatal("changed CPE credential was retried silently", v)
	}
}
func TestL07RadioWireRejectsCommandsAndUnknownFields(t *testing.T) {
	base := map[string]any{"version": 1, "request_id": "radio-test", "op": "radio.start", "capability": strings.Repeat("a", 64), "contract_revision": ProductRevision, "session": "session", "profile_id": "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", "activation_id": "activation", "expected_profile": "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", "expected_activation": "activation", "radio": map[string]any{"device_id": "bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb", "revision": 2, "operation_id": "cccccccc-cccc-4ccc-cccc-cccccccccccc", "family": "routeros-wireless-v6", "mode": "once"}}
	b, _ := json.Marshal(base)
	if _, c := validate(b); c != foundation.OK {
		t.Fatal("valid radio envelope", c)
	}
	for _, key := range []string{"command", "shell", "address", "secret", "credential_id"} {
		nested := base["radio"].(map[string]any)
		nested[key] = "forbidden"
		b, _ = json.Marshal(base)
		if _, c := validate(b); c != foundation.Invalid {
			t.Fatal("UI supplied unmanaged field", key, c)
		}
		delete(nested, key)
	}
}
