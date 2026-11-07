package composition

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/modules/pulse"
	"infradesktop/internal/testsupport/sshfixture"
)

type liveTestStore struct {
	*pulseStore
	credentials []equipment.SealedCredential
	profile     *equipment.RadioProfile
	target      equipment.PPTarget
	reveals     atomic.Int32
	writes      atomic.Int32
}

func (s *liveTestStore) ReadEquipment(ctx context.Context, profile string) (equipment.State, equipment.Code) {
	state, code := s.pulseStore.ReadEquipment(ctx, profile)
	state.Credentials = s.credentials
	return state, code
}
func (s *liveTestStore) UpdateEquipment(context.Context, string, func(*equipment.State) equipment.Code) equipment.Code {
	s.writes.Add(1)
	return equipment.Unavailable
}
func (s *liveTestStore) RadioCurrent(context.Context, string, string) (*equipment.RadioReading, *equipment.RadioProfile, equipment.Code) {
	return nil, s.profile, equipment.OK
}
func (s *liveTestStore) ReadPPTarget(context.Context, string, string, bool) (equipment.PPTarget, equipment.Code) {
	s.reveals.Add(1)
	return s.target, equipment.OK
}

type liveTestVault struct {
	equipment.Vault
	opens atomic.Int32
}

func (*liveTestVault) Ready(string, string, []byte) bool { return true }
func (v *liveTestVault) Open(string, string, string, uint64, []byte, []byte) (string, equipment.Code) {
	v.opens.Add(1)
	return sshfixture.Password, equipment.OK
}
func liveTestInventory(devices ...equipment.Device) (liveInventory, *liveTestStore) {
	store := &liveTestStore{pulseStore: &pulseStore{
		state: access.State{Account: &access.Account{Profile: "p"}, Companies: []access.Company{{ID: "p"}}}, devices: devices},
		credentials: []equipment.SealedCredential{{Metadata: equipment.Credential{ID: "c", Revision: 7}}}}
	return liveInventory{access: access.New(store, nil, time.Now, uint32(os.Getuid()), "a"), store: store}, store
}
func TestLiveEligibilityRequiresAssignedCredentialAndConfirmedRouterOS(t *testing.T) {
	base := equipment.Device{ID: "d", Role: equipment.RoleAP, Active: true, Transport: equipment.TransportSSH,
		CredentialID: "c", Fingerprint: "trusted", Revision: 3}
	cases := []string{"confirmed", "concentrator", "subscriber", "lhg", "unconfirmed",
		"no-key", "no-credential", "inactive", "other-transport"}
	for _, state := range cases {
		t.Run(state, func(t *testing.T) {
			d := base
			inv, store := liveTestInventory()
			store.profile = &equipment.RadioProfile{Family: equipment.RadioRouterOSWirelessV6, Confirmed: true}
			d = liveEligibilityCase(state, d, store)
			store.devices = []equipment.Device{d}
			profile, targets, ok := inv.ReadTargets(context.Background())
			want := state == "confirmed" || state == "concentrator"
			if !ok || profile != "p" || (len(targets) == 1) != want {
				t.Fatal(profile, targets, ok)
			}
			if store.reveals.Load() != 0 {
				t.Fatal("inventory revealed credentials")
			}
			if want && targets[0].Stamp != pulse.ReadStamp("p", equipment.PPConnection(d), 3, 7) {
				t.Fatal("wrong stamp")
			}
		})
	}
}
func liveEligibilityCase(state string, d equipment.Device, store *liveTestStore) equipment.Device {
	switch state {
	case "concentrator":
		d.Role = equipment.RoleConcentrator
	case "subscriber":
		d.Role = equipment.RoleCPE
	case "lhg":
		store.profile.Family = equipment.RadioUnsupported
	case "unconfirmed":
		store.profile.Confirmed = false
	case "no-key":
		d.Fingerprint = ""
	case "no-credential":
		d.CredentialID = "missing"
	case "inactive":
		d.Active = false
	case "other-transport":
		d.Transport = ""
	}
	return d
}
func TestLiveDialRejectsChangedStampBeforeOpeningVault(t *testing.T) {
	d := equipment.Device{ID: "d", Role: equipment.RoleConcentrator, Active: true, Transport: equipment.TransportSSH,
		CredentialID: "c", Fingerprint: "trusted", Revision: 3}
	inv, store := liveTestInventory(d)
	store.target = equipment.PPTarget{Company: "p", Device: d, Credential: store.credentials[0]}
	vault := &liveTestVault{}
	_, targets, _ := inv.ReadTargets(context.Background())
	store.target.Credential.Metadata.Revision++
	session, fault := (liveDialer{inv, vault}).Dial(context.Background(), targets[0])
	if session != nil || fault != pulse.ReadLost || vault.opens.Load() != 0 {
		t.Fatal("stale target opened secret")
	}
}
func TestLiveBackgroundReadsWithoutSessionAndStopsBeforeStore(t *testing.T) {
	read := make(chan struct{}, 1)
	peer, err := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) {
		select {
		case read <- struct{}{}:
		default:
		}
		return "L\t1\t1h\t98\t9\n", true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	id := ""
	for n := 0; id == ""; n++ {
		if candidate := fmt.Sprint(n); pulse.ReadSlot(candidate) == 0 {
			id = candidate
		}
	}
	d := equipment.Device{ID: id, Role: equipment.RoleConcentrator, Active: true, Transport: equipment.TransportSSH,
		Address: "127.0.0.1", Port: peer.Port(), CredentialID: "c", Fingerprint: peer.Fingerprint(), Revision: 3}
	inv, store := liveTestInventory(d)
	store.credentials[0].Metadata.User = "reader"
	store.target = equipment.PPTarget{Company: "p", Device: d, Credential: store.credentials[0]}
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	var reader *pulse.ReadCollector
	closedBeforeStore := false
	server, err := ipc.ListenAccess(root, uint32(os.Getuid()), inv.access, foundation.Status{}, func() error {
		_, ok := reader.Snapshot("p", time.Now())
		closedBeforeStore = !ok
		store.closed.Store(true)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ping := pulse.NewCollector(pulseInventory{inv.access, store}, liveReply{}, 4)
	ping.Step(context.Background(), 0, time.Now())
	reader = composeLiveReader(server, inv, &liveTestVault{}, ping)
	select {
	case <-read:
	case <-time.After(2 * time.Second):
		t.Fatal("no background SSH read without session")
	}
	server.Close()
	if !closedBeforeStore || store.writes.Load() != 0 || store.readAfterClose.Load() {
		t.Fatal("lifecycle or persistence violated")
	}
	if peer.Handshakes.Load() != 1 {
		t.Fatal("unexpected handshakes")
	}
}

type liveReply struct{}

func (liveReply) Probe(context.Context, string) pulse.Outcome { return pulse.Reply }
