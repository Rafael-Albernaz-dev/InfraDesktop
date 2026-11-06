package inventory_test

import (
	"context"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"testing"
	"time"
)

type renameJobStore struct {
	equipment.Store
	state equipment.State
}

func (s *renameJobStore) ReadEquipment(context.Context, string) (equipment.State, equipment.Code) {
	return s.state, equipment.OK
}
func (s *renameJobStore) UpdateEquipment(_ context.Context, _ string, f func(*equipment.State) equipment.Code) equipment.Code {
	return f(&s.state)
}

type renameJobVault struct{ equipment.Vault }

func (renameJobVault) Available() bool                       { return true }
func (renameJobVault) Ready(string, string, []byte) bool     { return true }
func (renameJobVault) Stamp(string) (string, equipment.Code) { return "synthetic", equipment.OK }
func (renameJobVault) Open(string, string, string, uint64, []byte, []byte) (string, equipment.Code) {
	return "synthetic", equipment.OK
}
func (renameJobVault) Seal(string, string, string, uint64, string, []byte) ([]byte, []byte, equipment.Code) {
	return []byte{1}, []byte{1}, equipment.OK
}
func TestCredentialRenameDoesNotCancelAuthorizedJob(t *testing.T) {
	credential, deviceID := fixtureUUID(), fixtureUUID()
	d := equipment.Device{ID: deviceID, CredentialID: credential, Revision: 1, Active: true}
	store := &renameJobStore{state: equipment.State{
		Company: "company", WrappedKey: []byte{1}, Devices: []equipment.Device{d},
		Credentials: []equipment.SealedCredential{{
			Metadata: equipment.Credential{ID: credential, Alias: "old", User: "reader", Revision: 1},
			Envelope: []byte{1},
		}},
	}}
	service := inventory.New(renewalAuth{}, store, renameJobVault{}, renewalRemote{}, renewalIDs{}, time.Now)
	t.Cleanup(service.Close)
	scope := access.Command{Profile: "company", ExpectedProfile: "company",
		Activation: "active", ExpectedActivation: "active"}
	cancelled := false
	job := &inventory.RunningJob{
		Command: equipment.Command{Scope: scope}, Device: d, View: equipment.Job{State: "RUNNING"},
		Cancel: func() { cancelled = true },
	}
	service.Jobs["synthetic"] = job
	q := equipment.Command{Op: "credential.put", Scope: scope, ID: credential, Operation: fixtureUUID(),
		Revision: 1, Alias: "renamed", User: "reader", Affected: 1}
	view, code := service.Handle(context.Background(), q)
	if code != equipment.OK || view.Credential == nil {
		t.Fatal("rename failed", code)
	}
	if view.Credential.Revision != 1 || cancelled || job.View.State != "RUNNING" {
		t.Fatal("metadata-only rename revoked an authorized job", code, cancelled, job.View.State)
	}
}
