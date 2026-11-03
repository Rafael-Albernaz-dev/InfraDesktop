package routerssh_test

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type standalonePeerStore struct {
	recoveryStore
	mu    sync.Mutex
	state equipment.State
}

func (s *standalonePeerStore) ReadEquipment(_ context.Context, _ string) (equipment.State, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := s.state
	v.Devices = append([]equipment.Device(nil), s.state.Devices...)
	return v, equipment.OK
}
func (s *standalonePeerStore) UpdateEquipment(_ context.Context, _ string, f func(*equipment.State) equipment.Code) equipment.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	return f(&s.state)
}
func (s *standalonePeerStore) ReadPPTarget(_ context.Context, _ string, id string, _ bool) (equipment.PPTarget, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.state.Devices {
		if d.ID == id {
			return equipment.PPTarget{Device: d,
					Credential: equipment.SealedCredential{Metadata: equipment.Credential{ID: d.CredentialID,
						Revision: 1,
						User:     "reader"}}},
				equipment.OK
		}
	}
	return equipment.PPTarget{}, equipment.Invalid
}
func TestStandaloneTOFURealAntesDaSenha(t *testing.T) {
	raw := recoveryGolden(t, "l07_avulso_station_v6.txt")
	var store *standalonePeerStore
	var peer *sshfixture.Server
	var persisted atomic.Bool
	peer, e := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", OnPassword: func() {
		st, _ := store.ReadEquipment(context.Background(), "synthetic")
		persisted.Store(st.Devices[0].Fingerprint == peer.Fingerprint())
	}, Command: func(cmd string) (string, bool) {
		if cmd == routerssh.RadioIdentificationCommand {
			return standaloneIdentity(raw), true
		}
		if cmd == routerssh.RadioStationCommand {
			return raw, true
		}
		return "", false
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	id := uuid.NewString()
	d := equipment.Device{ID: id,
		Role:         equipment.RolePTP,
		Revision:     1,
		Active:       true,
		Address:      "127.0.0.1",
		Port:         peer.Port(),
		Transport:    equipment.TransportSSH,
		CredentialID: uuid.NewString()}
	store = &standalonePeerStore{state: equipment.State{Devices: []equipment.Device{d}}}
	svc := equipment.New(&recoveryAuth{}, store, recoveryVault{}, routerssh.Adapter{}, recoveryIDs{}, time.Now)
	defer svc.Close()
	q := equipment.Command{Op: "radio.start",
		Scope: access.Command{Profile: "synthetic",
			ExpectedProfile:    "synthetic",
			Activation:         "active",
			ExpectedActivation: "active"}}
	v,
		c := svc.HandleRadio(context.Background(),
		q,
		equipment.RadioInput{DeviceID: id,
			Revision:  1,
			Operation: uuid.NewString(),
			Mode:      equipment.RadioOnce,
			Family:    equipment.RadioStandalone})
	if c != equipment.OK {
		t.Fatal(c)
	}
	result := recoveryStatus(t, svc, q, v.Job.ID, func(v equipment.RadioView) bool { return v.Job.State != "RUNNING" })
	st, _ := store.ReadEquipment(context.Background(), "synthetic")
	if !persisted.Load() ||
		result.Job.Code != equipment.OK ||
		st.Devices[0].Fingerprint != peer.Fingerprint() ||
		peer.Auth.Load() != 1 ||
		peer.Handshakes.Load() != 2 {
		t.Fatalf("code=%s pin=%s auth=%d handshakes=%d", result.Job.Code, st.Devices[0].Fingerprint, peer.Auth.Load(), peer.Handshakes.Load())
	}
}

type revokeNotifyAuth struct {
	revoked atomic.Bool
	once    sync.Once
	denied  chan struct{}
}

func (a *revokeNotifyAuth) WithAuthorization(_ context.Context, _ uint32, _ access.Command, f func(uint64) equipment.Code) equipment.Code {
	if a.revoked.Load() {
		a.once.Do(func() { close(a.denied) })
		return access.Unauthorized
	}
	return f(1)
}
func TestStandaloneRevogadoDuranteInspecaoNaoSalvaPinNemEnviaSenha(t *testing.T) {
	auth := &revokeNotifyAuth{denied: make(chan struct{})}
	peer, err := sshfixture.Start(sshfixture.Config{ServerVersion: "SSH-2.0-ROSSSH", OnHandshake: func() { auth.revoked.Store(true) }})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	id := uuid.NewString()
	d := equipment.Device{ID: id,
		Role:         equipment.RolePTP,
		Revision:     1,
		Active:       true,
		Address:      "127.0.0.1",
		Port:         peer.Port(),
		Transport:    equipment.TransportSSH,
		CredentialID: uuid.NewString()}
	store := &standalonePeerStore{state: equipment.State{Devices: []equipment.Device{d}}}
	svc := equipment.New(auth, store, recoveryVault{}, routerssh.Adapter{}, recoveryIDs{}, time.Now)
	defer svc.Close()
	q := equipment.Command{Op: "radio.start",
		Scope: access.Command{Profile: "synthetic",
			ExpectedProfile:    "synthetic",
			Activation:         "active",
			ExpectedActivation: "active"}}
	v,
		c := svc.HandleRadio(context.Background(),
		q,
		equipment.RadioInput{DeviceID: id,
			Revision:  1,
			Operation: uuid.NewString(),
			Mode:      equipment.RadioOnce,
			Family:    equipment.RadioStandalone})
	if c != equipment.OK {
		t.Fatal(c)
	}
	deadline := time.Now().Add(time.Second)
	for peer.Handshakes.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for peer.ActiveConnections() != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if peer.Handshakes.Load() != 1 || peer.ActiveConnections() != 0 {
		t.Fatal("inspeção não terminou")
	}
	select {
	case <-auth.denied:
	case <-time.After(time.Second):
		t.Fatal("revogação não alcançou o guard")
	}
	svc.Close() // Wait for the worker after the authorization guard has rejected it.
	st, _ := store.ReadEquipment(context.Background(), "synthetic")
	if v.Job == nil || st.Devices[0].Fingerprint != "" || peer.Auth.Load() != 0 || peer.Commands.Load() != 0 {
		t.Fatalf("pin=%s auth=%d commands=%d", st.Devices[0].Fingerprint, peer.Auth.Load(), peer.Commands.Load())
	}
}
