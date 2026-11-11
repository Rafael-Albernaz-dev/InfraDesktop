package ipc

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"strings"
	"testing"
	"time"
)

func standaloneStationJourney(t *testing.T) *ppJourney {
	t.Helper()
	raw, err := os.ReadFile("../routerssh/radioparse/testdata/l07_avulso_station_v6.txt")
	if err != nil {
		t.Fatal(err)
	}
	station := string(raw)
	head, _, _ := strings.Cut(station, "L07-AVULSO-HEALTH")
	head = strings.ReplaceAll(head, "L07-AVULSO-RESOURCE", "L07-ID-RESOURCE")
	head = strings.ReplaceAll(head, "L07-AVULSO-PACKAGES", "L07-ID-PACKAGES")
	head = strings.ReplaceAll(head, "L07-AVULSO-BOARD", "L07-ID-BOARD")
	identity := head + "L07-ID-DONE\n"
	j := newPPJourney(t, sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		switch cmd {
		case radioparse.RadioIdentificationCommand:
			return identity, true
		case radioparse.RadioStationCommand:
			return station, true
		}
		t.Error("comando inesperado")
		return "", false
	}})
	return j
}
func createStandalonePTPTarget(t *testing.T, j *ppJourney) (string, string) {
	t.Helper()
	st, c := j.store.ReadEquipment(context.Background(), j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	cred := st.Devices[0].CredentialID
	id := uuid.NewString()
	created := j.client.Call(context.Background(),
		Request{Op: "inventory.create",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Equipment: &EquipmentInput{ID: id,
				Operation:    uuid.NewString(),
				Role:         equipment.RolePTP,
				Name:         "PTP sintético",
				Address:      "127.0.0.1",
				Port:         j.peer.Port(),
				Transport:    equipment.TransportSSH,
				CredentialID: cred}})
	if created.Code != foundation.OK {
		t.Fatal(created.Code)
	}
	return id, cred
}
func startStandaloneRadioOnce(t *testing.T, j *ppJourney, id string, revision uint64) string {
	t.Helper()
	start := j.client.Call(context.Background(),
		Request{Op: "radio.start",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Radio: &equipment.RadioInput{DeviceID: id,
				Revision:  revision,
				Operation: uuid.NewString(),
				Mode:      equipment.RadioOnce,
				Family:    equipment.RadioStandalone}})
	if start.Code != foundation.OK || start.Data.Radio.Job == nil {
		t.Fatal(start.Code)
	}
	return start.Data.Radio.Job.ID
}
func awaitStandaloneRadioJob(t *testing.T, j *ppJourney, job string) *equipment.RadioView {
	t.Helper()
	var v *equipment.RadioView
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r := j.client.Call(context.Background(),
			Request{Op: "radio.status",
				ExpectedProfile:    j.client.profile,
				ExpectedActivation: j.client.activation,
				Radio:              &equipment.RadioInput{JobID: job}})
		if r.Code != foundation.OK {
			t.Fatal(r.Code)
		}
		v = r.Data.Radio
		if v.Job.State != "RUNNING" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	return v
}
func assertStandaloneStoredReading(t *testing.T, j *ppJourney, id string) {
	t.Helper()
	current := j.client.Call(context.Background(),
		Request{Op: "radio.current",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Radio:              &equipment.RadioInput{DeviceID: id}})
	if current.Code != foundation.OK ||
		current.Data.Radio.Reading == nil ||
		current.Data.Radio.Reading.Model != "RBLHG-5HPnD" ||
		current.Data.Radio.Reading.Mode != "station-bridge" {
		t.Fatalf("leitura persistida: %+v", current.Data.Radio)
	}
}
func assertStandaloneTOFUReceipt(t *testing.T, j *ppJourney, id, cred string) {
	t.Helper()
	state, c := j.store.ReadEquipment(context.Background(), j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	found := false
	for _, d := range state.Devices {
		if d.ID == id {
			found = d.Fingerprint == j.peer.Fingerprint() && d.TrustOrigin == "automatic-tofu" && d.CredentialID == cred
		}
	}
	if !found || j.peer.Auth.Load() != 1 || j.peer.Commands.Load() != 2 {
		t.Fatalf("pin=%t auth=%d commands=%d", found, j.peer.Auth.Load(), j.peer.Commands.Load())
	}
}
