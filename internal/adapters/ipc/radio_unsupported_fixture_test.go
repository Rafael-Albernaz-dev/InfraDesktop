package ipc

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"infradesktop/internal/testsupport/testwait"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type unsupportedIPCScenario struct {
	journey         *ppJourney
	apPeer, cpePeer *sshfixture.Server
	ap, cpe         string
	input           equipment.RadioInput
}

func newUnsupportedIPCScenario(t *testing.T) *unsupportedIPCScenario {
	t.Helper()
	j, observation, row := unsupportedRadioSource(t)
	apPeer, cpePeer := unsupportedRadioPeers(t)
	ap := enrollUnsupportedRadioPeer(t, j, equipment.RoleAP, apPeer)
	cpe := enrollUnsupportedRadioPeer(t, j, equipment.RoleCPE, cpePeer)
	return &unsupportedIPCScenario{journey: j, apPeer: apPeer, cpePeer: cpePeer, ap: ap, cpe: cpe,
		input: equipment.RadioInput{ObservationID: observation, RowID: row}}
}
func unsupportedRadioSource(t *testing.T) (*ppJourney, string, string) {
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
	observation := j.collect(uuid.NewString())
	rows := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: observation.ID}, foundation.OK).Data.PPPoE.Rows
	return j, observation.ID, rows[0].ID
}
func readUnsupportedRadioGolden(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "routerssh", "radioparse", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func unsupportedRadioPeers(t *testing.T) (*sshfixture.Server, *sshfixture.Server) {
	t.Helper()
	rich := readUnsupportedRadioGolden(t, "l07_ap_v6_real.txt")
	unsupported := strings.ReplaceAll(readUnsupportedRadioGolden(t, "l07_airos_xm_real.txt"),
		"XM.ar7240.v6.0.3-licensed.30600.170329.2002", "WA.v8.7.0-synthetic")
	apPeer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		if text, ok := radioAPIdentificationFixture(t, cmd); ok {
			return text, true
		}
		if strings.Contains(cmd, "L07-TICK") {
			return ("L07-TICK\ncpu=40\nbssid=02:00:00:00:10:05\nL07-CLIENTS\n 0 interface=wlan1 mac" +
					"-address=02:00:00:00:10:01 uptime=1m signal-strength=-60 bytes=100,200 packets" +
					"=10,20\nL07-TICK-DONE\n"),
				true
		}
		return rich, true
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(apPeer.Close)
	cpePeer, err := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) { return unsupported, true }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cpePeer.Close)
	return apPeer, cpePeer
}
func enrollUnsupportedRadioPeer(t *testing.T, j *ppJourney, role equipment.Role, peer *sshfixture.Server) string {
	ctx := context.Background()
	t.Helper()
	id := uuid.NewString()
	r := j.client.Call(ctx,
		Request{Op: "inventory.enroll",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: id,
				Operation: uuid.NewString(),
				Role:      role,
				Name:      "synthetic " + string(role),
				Address:   "127.0.0.1",
				Port:      peer.Port(),
				Transport: equipment.TransportSSH,
				User:      "reader",
				Secret:    sshfixture.Password}})
	if r.Code != foundation.OK {
		t.Fatal(r.Code)
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
func (f *unsupportedIPCScenario) call(t *testing.T, op string, in equipment.RadioInput) *equipment.RadioView {
	t.Helper()
	r := f.journey.client.Call(context.Background(), Request{Op: op,
		ExpectedProfile: f.journey.client.profile, ExpectedActivation: f.journey.client.activation, Radio: &in})
	if r.Code != foundation.OK {
		t.Fatalf("%s: %s", op, r.Code)
	}
	return r.Data.Radio
}
func (f *unsupportedIPCScenario) finish(t *testing.T, in equipment.RadioInput) *equipment.RadioView {
	t.Helper()
	v := f.call(t, "radio.start", in)
	id := v.Job.ID
	until := testwait.Limit(t)
	for time.Now().Before(until) {
		v = f.call(t, "radio.status", equipment.RadioInput{JobID: id})
		if v.Job.State != "RUNNING" {
			time.Sleep(20 * time.Millisecond)
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return nil
}
