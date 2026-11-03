package ipc

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"testing"
)

func unknownRadioSourceJourney(t *testing.T) (*ppJourney, string, equipment.PPRow) {
	t.Helper()
	ctx := context.Background()
	j := newPPJourney(t, sshfixture.Config{Family: "v6", PPPoE: func() string {
		return "H\t1\t1\t6.49.19 (long-term)\t2d1h1s\nR\t*1\t1\tfixture\t127.0.0.1\t02:00:00:00:20:01\t1h1s\nZ\t1\tcomplete\t2d1h1s\n"
	}})
	if c := j.store.UpdateEquipment(ctx, j.client.profile, func(s *equipment.State) equipment.Code {
		for i := range s.Devices {
			if s.Devices[i].ID == j.device {
				s.Devices[i].Role = equipment.RoleConcentrator
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	obs := j.collect(uuid.NewString())
	rows := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: obs.ID}, foundation.OK).Data.PPPoE.Rows
	if len(rows) != 1 {
		t.Fatal("linha sintética ausente")
	}
	return j, obs.ID, rows[0]
}
func unknownRadioIdentityPeer(t *testing.T) *sshfixture.Server {
	t.Helper()
	peer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-unrecognized", Command: func(cmd string) (string, bool) {
		if (!strings.Contains(cmd, "mca-status") && cmd != routerssh.RadioWOMIdentificationCommand) || strings.Contains(cmd, "iw dev") {
			t.Error("outro driver usado")
		}
		return "L07-DRIVER-ABSENT\n", true
	}})
	if err != nil {
		t.Fatal(err)
	}
	return peer
}
func enrollPreservedRadioTargets(t *testing.T, j *ppJourney, peer *sshfixture.Server) (string, string) {
	t.Helper()
	ctx := context.Background()
	state, c := j.store.ReadEquipment(ctx, j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	cred := state.Devices[0].CredentialID
	ap, id := uuid.NewString(), uuid.NewString()
	for _, item := range []struct {
		id   string
		role equipment.Role
		port int
	}{{ap, equipment.RoleAP, j.peer.Port()}, {id, equipment.RoleCPE, peer.Port()}} {
		r := j.client.Call(ctx,
			Request{Op: "inventory.create",
				ExpectedProfile:    j.client.profile,
				ExpectedActivation: j.client.activation,
				Equipment: &EquipmentInput{ID: item.id,
					Operation:    uuid.NewString(),
					Role:         item.role,
					Name:         "rádio sintético",
					Address:      "127.0.0.1",
					Port:         item.port,
					Transport:    equipment.TransportSSH,
					CredentialID: cred}})
		if r.Code != foundation.OK {
			t.Fatal(r.Code)
		}
	}
	if c := j.store.UpdateEquipment(ctx, j.client.profile, func(s *equipment.State) equipment.Code {
		for i := range s.Devices {
			if s.Devices[i].ID == id {
				s.Devices[i].Fingerprint = peer.Fingerprint()
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	return ap, id
}
