package ipc

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
)

type resetJourneyIDs struct{}

func (resetJourneyIDs) ID() (string, equipment.Code) { return uuid.NewString(), equipment.OK }

func resetInventoryHarness(t *testing.T) (*AccountClient, *ipcResetFixture, *time.Time) {
	t.Helper()
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"runtime", "state", "client"} {
		if err := os.Mkdir(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	store, err := storage.Open(filepath.Join(root, "state"))
	if err != nil {
		t.Fatal(err)
	}
	now, uid := time.Now(), uint32(os.Getuid())
	account := access.New(store, security.Crypto{}, func() time.Time { return now }, uid, "reset-journey", store)
	proof := &ipcResetFixture{}
	v := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	inventory := equipment.New(account, store, v, nil, resetJourneyIDs{}, func() time.Time { return now })
	t.Cleanup(inventory.Close)
	server, err := ListenAccess(filepath.Join(root, "runtime"), uid, account, foundation.Status{Instance: "fixture"}, store.Close)
	if err != nil {
		t.Fatal(err)
	}
	account.SetResetProof(proof)
	AttachEquipment(server, inventory)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	return NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid), proof, &now
}

func TestLostAccountResetReachesPreservedInventory(t *testing.T) {
	client, proof, clock := resetInventoryHarness(t)
	call := func(q Request, want foundation.Code) Response {
		t.Helper()
		r := client.Call(context.Background(), q)
		if r.Code != want {
			t.Fatalf("%s: got %s want %s", q.Op, r.Code, want)
		}
		return r
	}
	original := call(Request{Op: "access.register", Name: "old-user", Password: "synthetic old password"}, foundation.OK)
	call(Request{Op: "access.ack-issued", Recovery: original.Data.Recovery}, foundation.OK)
	*clock = clock.Add(31 * time.Second)
	call(Request{Op: "access.login", Name: "old-user", Password: "synthetic old password"}, foundation.OK)
	company := call(Request{Op: "company.create", Name: "Synthetic company"}, foundation.OK).Data.Companies[0].ID
	selected := call(Request{Op: "company.select", Name: company}, foundation.OK)
	profile, activation := selected.Data.Profile, selected.Data.Activation
	credential, device := uuid.NewString(), uuid.NewString()
	call(Request{Op: "credential.put", ExpectedProfile: profile, ExpectedActivation: activation,
		Equipment: &EquipmentInput{ID: credential, Operation: uuid.NewString(), Alias: "Fixture",
			User: "reader", Secret: "synthetic secret"}}, foundation.OK)
	call(Request{Op: "inventory.create", ExpectedProfile: profile, ExpectedActivation: activation,
		Equipment: &EquipmentInput{ID: device, Operation: uuid.NewString(), Name: "Fixture router", Address: "192.0.2.1",
			Port: 22, Transport: equipment.TransportSSH, CredentialID: credential}}, foundation.OK)
	call(Request{Op: "access.logout"}, foundation.OK)
	begin := call(Request{Op: "access.reset-begin"}, foundation.OK)
	proof.nonce = begin.Data.ResetNonce
	reset := call(Request{Op: "access.reset-complete", Name: "new-user",
		Password: "synthetic new password", ResetNonce: proof.nonce}, foundation.OK)
	if reset.Data.Recovery == "" || reset.Data.Recovery == original.Data.Recovery {
		t.Fatal("new recovery code missing")
	}
	call(Request{Op: "access.ack-issued", Recovery: reset.Data.Recovery}, foundation.OK)
	*clock = clock.Add(31 * time.Second)
	call(Request{Op: "access.login", Name: "new-user", Password: "synthetic old password"}, foundation.Unauthenticated)
	*clock = clock.Add(31 * time.Second)
	call(Request{Op: "access.login", Name: "new-user", Password: "synthetic new password"}, foundation.OK)
	selected = call(Request{Op: "company.select", Name: company}, foundation.OK)
	result := call(Request{Op: "inventory.list", ExpectedProfile: selected.Data.Profile, ExpectedActivation: selected.Data.Activation,
		Equipment: &EquipmentInput{}}, foundation.OK)
	if result.Data.Equipment == nil || len(result.Data.Equipment.Devices) != 1 ||
		result.Data.Equipment.Devices[0].ID != device || result.Data.Equipment.Devices[0].CredentialID != credential {
		t.Fatal("saved inventory or credential association was not preserved")
	}
}
