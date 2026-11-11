package routerssh_test

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/radioparse"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
	"time"
)

func TestStandaloneServicoEscolheSomenteDestinoERecusa(t *testing.T) {
	for _, mode := range []string{"ok", "auth"} {
		t.Run(mode, func(t *testing.T) {
			peer := standaloneSelectedStationPeer(t, mode)
			defer peer.Close()
			other, e2 := sshfixture.Start(sshfixture.Config{})
			if e2 != nil {
				t.Fatal(e2)
			}
			defer other.Close()
			id := uuid.NewString()
			d := equipment.Device{ID: id,
				Role:         equipment.RolePTP,
				Revision:     1,
				Active:       true,
				Address:      "127.0.0.1",
				Port:         peer.Port(),
				Transport:    equipment.TransportSSH,
				Fingerprint:  peer.Fingerprint(),
				CredentialID: uuid.NewString()}
			store := recoveryStore{devices: map[string]equipment.Device{id: d}}
			auth := &recoveryAuth{}
			svc := equipment.New(auth, store, recoveryVault{}, routerssh.Adapter{}, recoveryIDs{}, time.Now)
			defer svc.Close()
			q := equipment.Command{Op: "radio.start",
				Scope: access.Command{Profile: "synthetic",
					ExpectedProfile:    "synthetic",
					Activation:         "active",
					ExpectedActivation: "active"}}
			in := equipment.RadioInput{DeviceID: id,
				Revision:  1,
				Operation: uuid.NewString(),
				Mode:      equipment.RadioOnce,
				Family:    equipment.RadioStandalone}
			v, c := svc.HandleRadio(context.Background(), q, in)
			if c != equipment.OK {
				t.Fatal(c)
			}
			result := recoveryStatus(t, svc, q, v.Job.ID, func(v equipment.RadioView) bool { return v.Job.State != "RUNNING" })
			want := equipment.OK
			if mode == "auth" {
				want = equipment.RadioCodeRemoteAuthFailed
			}
			if result.Job.Code != want || peer.Auth.Load() != 1 || other.Handshakes.Load() != 0 {
				t.Fatalf("code=%s auth=%d other=%d", result.Job.Code, peer.Auth.Load(), other.Handshakes.Load())
			}
			if mode == "ok" && (result.Reading == nil || result.Reading.Model != "RBLHG-5HPnD" || peer.Commands.Load() != 2) {
				t.Fatalf("reading=%+v commands=%d", result.Reading, peer.Commands.Load())
			}
			if mode == "auth" && peer.Commands.Load() != 0 {
				t.Fatal("comando após recusa")
			}
		})
	}
}

func standaloneSelectedStationPeer(t *testing.T, mode string) *sshfixture.Server {
	t.Helper()
	raw := recoveryGolden(t, "l07_avulso_station_v6.txt")
	peer, e1 := sshfixture.Start(sshfixture.Config{Mode: mode, ServerVersion: "SSH-2.0-ROSSSH", Command: func(cmd string) (string, bool) {
		if cmd == radioparse.RadioIdentificationCommand {
			return standaloneIdentity(raw), true
		}
		if cmd == radioparse.RadioStationCommand {
			return raw, true
		}
		t.Errorf("comando inesperado")
		return "", false
	}})
	if e1 != nil {
		t.Fatal(e1)
	}
	return peer
}
