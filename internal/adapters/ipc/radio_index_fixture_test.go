package ipc

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"sync/atomic"
	"testing"
	"time"
)

type radioIndexIPCFixture struct {
	t                              *testing.T
	j                              *ppJourney
	observation                    *equipment.PPObservation
	row                            equipment.PPRow
	credential                     string
	peers                          []*sshfixture.Server
	executing, maxExecuting, calls atomic.Int32
	input                          equipment.RadioInput
	current                        *equipment.RadioView
	state                          equipment.State
}

func newRadioIndexIPCFixture(t *testing.T) *radioIndexIPCFixture {
	t.Helper()
	s := &radioIndexIPCFixture{t: t}
	s.j, s.observation, s.row, s.credential = radioIndexIPCSource(t)
	for n := 0; n < 2; n++ {
		s.startAP(n)
	}
	return s
}
func radioIndexIPCSource(t *testing.T) (*ppJourney, *equipment.PPObservation, equipment.PPRow, string) {
	t.Helper()

	ctx := context.Background()
	j := newPPJourney(t, sshfixture.Config{Family: "v6", PPPoE: func() string {
		return "H\t1\t1\t6.49.19 (long-term)\t2d1h1s\nR\t*1\t1\tsynthetic-radio\t127.0.0.1\t02:00:00:00:10:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
	}})
	if c := j.store.UpdateEquipment(ctx, j.client.profile, func(st *equipment.State) equipment.Code {
		for i := range st.Devices {
			if st.Devices[i].ID == j.device {
				st.Devices[i].Role = equipment.RoleConcentrator
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	obs := j.collect(uuid.NewString())
	row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: obs.ID}, foundation.OK).Data.PPPoE.Rows[0]
	st, c := j.store.ReadEquipment(ctx, j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	credential := st.Devices[0].CredentialID
	r := j.client.Call(ctx,
		Request{Op: "credential.default-set",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: credential,
				Operation:    uuid.NewString(),
				Role:         equipment.RoleCPE,
				CredentialID: credential}})
	if r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	return j, obs, row, credential
}
func (s *radioIndexIPCFixture) command(n int, cmd string) (string, bool) {
	executing, maxExecuting, calls := &s.executing, &s.maxExecuting, &s.calls

	if cmd != radioparse.RadioIndexCommand {
		return "unexpected command", true
	}
	active := executing.Add(1)
	defer executing.Add(-1)
	if active > maxExecuting.Load() {
		maxExecuting.Store(active)
	}
	calls.Add(1)
	time.Sleep(20 * time.Millisecond)
	mac, ip := "02:00:00:00:10:01", "127.0.0.1"
	if n == 1 {
		mac, ip = "AA:BB:CC:DD:EE:02", "192.0.2.20"
	}
	return fmt.Sprintf(("L07-INDEX\n 0 mac-address=%s interface=wlan1 bssid=02:00:00:00:10:05 last-ip=%" +
			"s signal-strength=-60\nL07-INDEX-DONE\n"),
			mac,
			ip),
		true

}
func (s *radioIndexIPCFixture) startAP(n int) {
	t, j, ctx, credential := s.t, s.j, context.Background(), s.credential
	peer, err := sshfixture.Start(sshfixture.Config{Command: func(cmd string) (string, bool) { return s.command(n, cmd) }})

	if err != nil {
		t.Fatal(err)
	}
	s.peers = append(s.peers, peer)
	t.Cleanup(peer.Close)
	r := j.client.Call(ctx,
		Request{Op: "inventory.create",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: uuid.NewString(),
				Operation: uuid.NewString(),
				Role:      equipment.RoleAP,
				Name: fmt.Sprintf("synthetic AP%d",
					n),
				Address:      "127.0.0.1",
				Port:         peer.Port(),
				Transport:    equipment.TransportSSH,
				CredentialID: credential}})
	if r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
}
func (s *radioIndexIPCFixture) call(op string, in equipment.RadioInput) *equipment.RadioView {
	t, j, ctx := s.t, s.j, context.Background()

	t.Helper()
	r := j.client.Call(ctx, Request{Op: op, ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, Radio: &in})
	if r.Code != foundation.OK {
		t.Fatalf("%s: %s", op, r.Code)
	}
	return r.Data.Radio
}
func (s *radioIndexIPCFixture) wait(v *equipment.RadioView) *equipment.RadioView {
	call := s.call
	id := v.Job.ID

	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		v = call("radio.status", equipment.RadioInput{JobID: id})
		if v.Job.State != "RUNNING" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return v
}
func (s *radioIndexIPCFixture) verifyInitialProposal() {
	t, j, obs, row, credential := s.t, s.j, s.observation, s.row, s.credential
	call, calls, maxExecuting := s.call, &s.calls, &s.maxExecuting

	in := equipment.RadioInput{DeviceID: j.device,
		Revision:      2,
		ObservationID: obs.ID,
		RowID:         row.ID,
		Operation:     uuid.NewString(),
		Mode:          equipment.RadioModeIndex}
	started := call("radio.start", in)
	id := started.Job.ID
	v := s.wait(started)

	if v.Job.Code != equipment.OK ||
		v.Proposal == nil ||
		v.Proposal.Choice.CPE != "" ||
		v.Proposal.CredentialID != credential ||
		!v.Index.Complete {
		t.Fatalf("index proposal=%+v job=%+v reason=%s", v.Proposal, v.Job, v.MatchReason)
	}
	if calls.Load() != 2 || maxExecuting.Load() != 1 {
		t.Fatal("index not one sequential read/AP", calls.Load(), maxExecuting.Load())
	}
	repeated := call("radio.start", in)
	if repeated.Job.ID != id || calls.Load() != 2 {
		t.Fatal("same operation repeated remote reads")
	}
	current := call("radio.link-current", equipment.RadioInput{DeviceID: j.device, ObservationID: obs.ID, RowID: row.ID})
	if current.Proposal == nil || calls.Load() != 2 {
		t.Fatal("cache did not serve locally")
	}
	s.input, s.current = in, current
}
func (s *radioIndexIPCFixture) replaceDefaultCredential() {
	t, j, ctx := s.t, s.j, context.Background()

	replacement := uuid.NewString()
	rr := j.client.Call(ctx,
		Request{Op: "credential.put",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: replacement,
				Operation: uuid.NewString(),
				Alias:     "replacement class default",
				User:      "reader",
				Secret:    sshfixture.Password}})
	if rr.Code != foundation.OK {
		t.Fatal(rr.Code)
	}

	rr = j.client.Call(ctx,
		Request{Op: "credential.default-set",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: replacement,
				Operation:    uuid.NewString(),
				Role:         equipment.RoleCPE,
				CredentialID: replacement}})
	if rr.Code != foundation.OK {
		t.Fatal(rr.Code)
	}
}

// A concurrent class-default change cannot change the association shown to
// the user when this proposal was returned.
func (s *radioIndexIPCFixture) verifyDefaultRace() {
	t, j, ctx, current, obs, row := s.t, s.j, context.Background(), s.current, s.observation, s.row
	s.replaceDefaultCredential()

	stale := j.client.Call(ctx,
		Request{Op: "radio.link-confirm",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Radio: &equipment.RadioInput{DeviceID: j.device,
				ObservationID: obs.ID,
				RowID:         row.ID,
				Operation:     uuid.NewString(),
				Link:          &current.Proposal.Choice}})
	if stale.Code != foundation.Code(equipment.Revision) {
		t.Fatalf("proposal silently changed credential: %s", stale.Code)
	}
	s.resetDefaultCredential()
}
func (s *radioIndexIPCFixture) resetDefaultCredential() {
	t, j, ctx, credential, obs, row, call := s.t, s.j, context.Background(), s.credential, s.observation, s.row, s.call
	rr := j.client.Call(ctx,
		Request{Op: "credential.default-set",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: credential,
				Operation:    uuid.NewString(),
				Role:         equipment.RoleCPE,
				CredentialID: credential}})
	if rr.Code != foundation.OK {
		t.Fatal(rr.Code)
	}
	current := call("radio.link-current", equipment.RadioInput{DeviceID: j.device, ObservationID: obs.ID, RowID: row.ID})

	s.current = current
}
func (s *radioIndexIPCFixture) confirmAndVerifyReplay() *equipment.RadioView {
	t, j, obs, row, current, call := s.t, s.j, s.observation, s.row, s.current, s.call

	confirmOperation := uuid.NewString()
	confirmed := call("radio.link-confirm",
		equipment.RadioInput{DeviceID: j.device,
			ObservationID: obs.ID,
			RowID:         row.ID,
			Operation:     confirmOperation,
			Link:          &current.Proposal.Choice})
	repeatedConfirmation := call("radio.link-confirm",
		equipment.RadioInput{DeviceID: j.device,
			ObservationID: obs.ID,
			RowID:         row.ID,
			Operation:     confirmOperation,
			Link:          &current.Proposal.Choice})
	if repeatedConfirmation.Link == nil || repeatedConfirmation.Link.CPE != confirmed.Link.CPE {
		t.Fatal("confirmation replay changed association")
	}
	if confirmed.Link == nil || confirmed.Link.CPE == "" {
		t.Fatal("one-click link not persisted")
	}
	return confirmed
}
func (s *radioIndexIPCFixture) verifyAtomicLink(confirmed *equipment.RadioView) {
	t, j, ctx, row, credential, peers := s.t, s.j, context.Background(), s.row, s.credential, s.peers
	var st equipment.State
	var c equipment.Code

	st, c = j.store.ReadEquipment(ctx, j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	found := false
	for _, d := range st.Devices {
		if d.ID == confirmed.Link.CPE {
			found = d.CredentialID == credential && d.Address == row.Address
		}
		if d.Role == equipment.RoleAP && (d.TrustAcceptance == nil || d.TrustAcceptance.Mode != "automatic-tofu") {
			t.Fatal("TOFU not recorded")
		}
	}
	if !found {
		t.Fatal("CPE/default not persisted atomically")
	}
	for _, peer := range peers {
		if peer.Auth.Load() != 1 || peer.Commands.Load() != 1 || peer.GlobalRequests.Load() != 0 {
			t.Fatal("extra credential/command attempts", peer.Auth.Load(), peer.Commands.Load())
		}
	}
	s.state = st
}
func (s *radioIndexIPCFixture) createRefusedCredential() string {
	t, j, ctx := s.t, s.j, context.Background()

	bad := uuid.NewString()
	refusedCredential := j.client.Call(ctx,
		Request{Op: "credential.put",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: bad,
				Operation: uuid.NewString(),
				Alias:     "synthetic refused",
				User:      "reader",
				Secret:    "synthetic-wrong"}})
	if refusedCredential.Code != foundation.OK {
		t.Fatal(refusedCredential.Code)
	}
	return bad
}
func (s *radioIndexIPCFixture) replaceAPCredential(bad string) equipment.Device {
	t, j, ctx, st, peers := s.t, s.j, context.Background(), s.state, s.peers

	var target equipment.Device
	for _, d := range st.Devices {
		if d.Role == equipment.RoleAP && d.Port == peers[0].Port() {
			target = d
		}
	}

	changed := j.client.Call(ctx,
		Request{Op: "inventory.update",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: target.ID,
				Operation:    uuid.NewString(),
				Revision:     target.Revision,
				Role:         target.Role,
				Name:         target.Name,
				Address:      target.Address,
				Port:         target.Port,
				Transport:    target.Transport,
				CredentialID: bad}})
	if changed.Code != foundation.OK {
		t.Fatal(changed.Code)
	}
	return target
}

// A refusal on the NEW index path must expose its target and never try the
// existing good credential. Only the next explicit start may authenticate.
func (s *radioIndexIPCFixture) verifyRefusal() {
	t, peers, call := s.t, s.peers, s.call
	bad := s.createRefusedCredential()
	target := s.replaceAPCredential(bad)
	in := s.input
	in.Operation = uuid.NewString()
	v := s.wait(call("radio.start", in))

	if v.Index == nil ||
		v.Index.Complete ||
		v.Index.Targets[target.ID] != equipment.RadioCodeRemoteAuthFailed ||
		v.Proposal != nil ||
		peers[0].Auth.Load() != 2 {
		t.Fatalf("index refused target missing or fallback attempted: index=%+v auth=%d", v.Index, peers[0].Auth.Load())
	}
	if v.Index.Labels[target.ID] == "" {
		t.Fatal("refused target has no user-readable identity")
	}

	t.Log(("IPC→service→SQLite→SSH: two APs read sequentially once, TOFU recorded, c" +
		"ache/replay no IO, one-click creates CPE+link with selected credential"))

}
