package ipc

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type radioIndependentFixture struct {
	t                        *testing.T
	j                        *ppJourney
	observation              *equipment.PPObservation
	row                      equipment.PPRow
	apPeer, cpePeer          *sshfixture.Server
	lastView                 *equipment.RadioView
	ap, cpe                  string
	apFail, cpeFail, holdCPE atomic.Bool
	entered, release         chan struct{}
	releaseOnce              sync.Once
	input                    equipment.RadioInput
}

func newRadioIndependentFixture(t *testing.T) *radioIndependentFixture {
	t.Helper()
	s := &radioIndependentFixture{t: t, entered: make(chan struct{}, 1), release: make(chan struct{})}
	s.j, s.observation, s.row = radioIndependentSource(t)
	apGolden, womGolden := readRadioIPCFixture(t, "l07_ap_v6_real.txt"), readRadioIPCFixture(t, "l07_wom_iw_real.txt")
	s.apPeer = s.startAP(apGolden)
	s.cpePeer = s.startCPE(womGolden)
	t.Cleanup(func() { s.releaseOnce.Do(func() { close(s.release) }) })
	s.ap, s.cpe = s.enroll(equipment.RoleAP, s.apPeer), s.enroll(equipment.RoleCPE, s.cpePeer)
	return s
}
func radioIndependentSource(t *testing.T) (*ppJourney, *equipment.PPObservation, equipment.PPRow) {
	t.Helper()

	ctx := context.Background()
	j := newPPJourney(t, sshfixture.Config{Family: "v6", PPPoE: func() string {
		return "H\t1\t1\t6.49.19 (long-term)\t2d1h1s\nR\t*1\t1\tsynthetic-radio\t127.0.0.1\t58:10:8C:75:E3:08\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
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
	observation := j.collect(uuid.NewString())
	rows := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: observation.ID}, foundation.OK).Data.PPPoE.Rows
	if len(rows) != 1 {
		t.Fatal("source row", rows)
	}
	return j, observation, rows[0]
}
func readRadioIPCFixture(t *testing.T, name string) string {

	t.Helper()
	b, e := os.ReadFile(filepath.Join("..", "routerssh", "radioparse", "testdata", name))
	if e != nil {
		t.Fatal(e)
	}
	return string(b)
}
func (s *radioIndependentFixture) startAP(apGolden string) *sshfixture.Server {
	t, apFail := s.t, &s.apFail

	apPeer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		if text, ok := radioAPIdentificationFixture(t, cmd); ok {
			return text, true
		}
		if cmd != radioparse.RadioAPCommand {
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
	return apPeer

}
func (s *radioIndependentFixture) startCPE(womGolden string) *sshfixture.Server {
	t, cpeFail, holdCPE := s.t, &s.cpeFail, &s.holdCPE
	entered, release := s.entered, s.release
	read := func(name string) string { return readRadioIPCFixture(t, name) }

	cpePeer, e := sshfixture.Start(sshfixture.Config{Command: func(cmd string) (string, bool) {
		if strings.Contains(cmd, "mca-status") {
			return "L07-DRIVER-ABSENT\n", true
		}
		if cmd == radioparse.RadioWOMIdentificationCommand {
			return read("radio_wom_library_identity.txt"), true
		}
		if cmd != radioparse.RadioWOMCommand {
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
	return cpePeer

}
func (s *radioIndependentFixture) enroll(role equipment.Role, peer *sshfixture.Server) string {
	t, j, ctx := s.t, s.j, context.Background()

	t.Helper()
	id := uuid.NewString()
	r := j.client.Call(ctx,
		Request{Op: "inventory.enroll",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: id,
				Operation: uuid.NewString(),
				Role:      role,
				Name:      "Synthetic " + string(role),
				Address:   "127.0.0.1",
				Port:      peer.Port(),
				Transport: equipment.TransportSSH,
				User:      "reader",
				Secret:    sshfixture.Password}})
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
func (s *radioIndependentFixture) call(op string, in equipment.RadioInput, want foundation.Code) *equipment.RadioView {
	t, j, ctx := s.t, s.j, context.Background()

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
func (s *radioIndependentFixture) finish(in equipment.RadioInput) *equipment.RadioView {
	t, call := s.t, s.call

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
func (s *radioIndependentFixture) verifyInitialReadings() {
	t, ap, cpe, finish, call := s.t, s.ap, s.cpe, s.finish, s.call

	for _, target := range []struct {
		id     string
		family equipment.RadioFamily
	}{{ap, equipment.RadioRouterOSWirelessV6}, {cpe, equipment.RadioIntelbrasWOM}} {
		v := finish(equipment.RadioInput{DeviceID: target.id,
			Revision:  2,
			Family:    target.family,
			Mode:      equipment.RadioOnce,
			Operation: uuid.NewString()})
		if v.Job.Code != equipment.OK || v.Reading == nil || v.Reading.CommandSeconds <= 0 || v.Reading.Seconds < v.Reading.CommandSeconds {
			t.Fatal("initial radio", v)
		}
		saved := call("radio.current", equipment.RadioInput{DeviceID: target.id}, foundation.OK)
		if saved.Reading == nil ||
			saved.Reading.CommandSeconds != v.Reading.CommandSeconds ||
			saved.Reading.Seconds < saved.Reading.CommandSeconds {
			t.Fatal("timing lost across SSH/store/IPC", saved)
		}
		call("radio.confirm-family",
			equipment.RadioInput{DeviceID: target.id,
				Revision: 2,
				Family:   target.family,
				Warning:  15,
				Critical: 20},
			foundation.OK)
	}
}
func (s *radioIndependentFixture) confirmLink() {
	j, observation, ap, cpe, call := s.j, s.observation, s.ap, s.cpe, s.call
	rows := []equipment.PPRow{s.row}

	choice := &equipment.RadioLinkChoice{AP: ap,
		CPE:       cpe,
		MAC:       "02:00:00:00:10:03",
		Interface: "wlan1",
		BSSID:     "02:00:00:00:10:05",
		Evidence:  "synthetic confirmed MAC association",
		Origin:    "manual"}
	input := equipment.RadioInput{DeviceID: j.device,
		ObservationID: observation.ID,
		RowID:         rows[0].ID,
		Revision:      2,
		Mode:          equipment.RadioOnce,
		Operation:     uuid.NewString()}
	call("radio.link-confirm",
		equipment.RadioInput{DeviceID: j.device,
			ObservationID: observation.ID,
			RowID:         rows[0].ID,
			Operation:     uuid.NewString(),
			Link:          choice},
		foundation.OK)
	s.input = input

}
func (s *radioIndependentFixture) verifyIndependentFailures() {
	t, apFail, cpeFail, finish := s.t, &s.apFail, &s.cpeFail, s.finish
	input := s.input

	apFail.Store(true)
	v := finish(input)
	if v.Job.Code != equipment.Partial || v.APCode != equipment.RadioCodeRemotePermissionDenied ||
		v.CPECode != equipment.OK || v.CPEReading == nil {
		t.Fatal("AP failure erased independently pinned CPE", v)
	}
	apFail.Store(false)
	cpeFail.Store(true)
	input.Operation = uuid.NewString()
	v = finish(input)
	if v.Job.Code != equipment.Partial ||
		v.APCode != equipment.OK ||
		v.Reading == nil ||
		len(v.Reading.Clients) != 1 ||
		v.CPECode != equipment.RadioCodeRemotePermissionDenied {
		t.Fatal("CPE failure erased AP or exposed other clients", v)
	}
	cpeFail.Store(false)
	s.input, s.lastView = input, v

}
func (s *radioIndependentFixture) revokeCPEAndRequireExplicitChoice() int32 {
	t, j, ctx := s.t, s.j, context.Background()
	holdCPE, entered, cpe, cpePeer := &s.holdCPE, s.entered, s.cpe, s.cpePeer
	input, call, finish := s.input, s.call, s.finish
	v := s.lastView

	holdCPE.Store(true)
	input.Operation = uuid.NewString()
	job := call("radio.start", input, foundation.OK).Job.ID
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("CPE command not entered")
	}
	r := j.client.Call(ctx,
		Request{Op: "credential.put",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: cpe,
				Revision:  1,
				Operation: uuid.NewString(),
				Alias:     "Synthetic CPE",
				User:      "reader",
				Secret:    "replacement-synthetic-only",
				Affected:  1}})
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
	s.input = input
	return before

}
func (s *radioIndependentFixture) createExplicitCPECredential() string {
	t, j, ctx := s.t, s.j, context.Background()
	var r Response

	replacement := uuid.NewString()
	r = j.client.Call(ctx,
		Request{Op: "credential.put",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: replacement,
				Operation: uuid.NewString(),
				Alias:     "Explicit CPE choice",
				User:      "reader",
				Secret:    sshfixture.Password}})
	if r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	return replacement

}
func (s *radioIndependentFixture) replaceCPECredential(replacement string, before int32) {
	t, j, ctx, cpe, cpePeer := s.t, s.j, context.Background(), s.cpe, s.cpePeer
	var r Response

	st, c := j.store.ReadEquipment(ctx, j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	var target equipment.Device
	for _, d := range st.Devices {
		if d.ID == cpe {
			target = d
		}
	}
	r = j.client.Call(ctx,
		Request{Op: "inventory.update",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: cpe,
				Operation:    uuid.NewString(),
				Revision:     target.Revision,
				Role:         target.Role,
				Name:         target.Name,
				Address:      target.Address,
				Port:         target.Port,
				Transport:    target.Transport,
				CredentialID: replacement}})
	if r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	if cpePeer.Auth.Load() != before {
		t.Fatal("dropdown save authenticated automatically")
	}
}

// A distinct dropdown choice is intentional and must recover without a
// manual family wizard or another credential attempt.
func (s *radioIndependentFixture) recoverExplicitChoice(before int32) {
	t, cpePeer, finish := s.t, s.cpePeer, s.finish
	s.holdCPE.Store(false)
	s.releaseOnce.Do(func() { close(s.release) })
	replacement := s.createExplicitCPECredential()
	s.replaceCPECredential(replacement, before)
	input := s.input
	var v *equipment.RadioView

	input.Operation = uuid.NewString()
	v = finish(input)
	if v.Job.Code != equipment.OK || v.CPECode != equipment.OK || v.CPEReading == nil || cpePeer.Auth.Load() != before+1 {
		t.Fatalf("explicit choice did not recover: job=%+v side=%s auth=%d", v.Job, v.CPECode, cpePeer.Auth.Load()-before)
	}

}
