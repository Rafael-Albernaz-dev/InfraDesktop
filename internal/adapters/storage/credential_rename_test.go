package storage

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

type renameObservedVault struct {
	equipment.Vault
	opens, seals int
}

func (v *renameObservedVault) Open(a, b, c string, r uint64, e, k []byte) (string, equipment.Code) {
	v.opens++
	return v.Vault.Open(a, b, c, r, e, k)
}
func (v *renameObservedVault) Seal(a, b, c string, r uint64, s string, k []byte) ([]byte, []byte, equipment.Code) {
	v.seals++
	return v.Vault.Seal(a, b, c, r, s, k)
}
func saveConfirmedRadioForEdit(t *testing.T, j *journey, target equipment.PPTarget) {
	t.Helper()
	ctx := context.Background()
	r := &equipment.RadioReading{ID: uuid.NewString(), Family: equipment.RadioRouterOSWirelessV6, Perspective: "AP"}
	equipment.StampRadio(r, target.Device, target.Credential.Metadata.Revision, time.Now())
	if code := j.st.SaveRadio(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r); code != equipment.OK {
		t.Fatal(code)
	}
	code := j.st.ConfirmRadioFamily(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r.Family, 10, 20)
	if code != equipment.OK {
		t.Fatal(code)
	}
}
func TestCredentialRenamePreservesSealedSecretAndSavedReadings(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	saveConfirmedRadioForEdit(t, j, target)
	observation := ppSynthetic(t, j, target, 1, "synthetic-login")
	before, code := j.st.ReadEquipment(ctx, companyX)
	if code != equipment.OK {
		t.Fatal(code)
	}
	observed := &renameObservedVault{Vault: vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })}
	service := equipment.New(j.svc, j.st, observed, routerssh.Adapter{}, equipmentIDs{}, time.Now)
	t.Cleanup(service.Close)
	q := equipmentScope(j)
	q.Op, q.ID, q.Operation = "credential.put", target.Credential.Metadata.ID, uuid.NewString()
	q.Alias, q.User = "Renamed synthetic", target.Credential.Metadata.User
	q.Revision, q.Affected = target.Credential.Metadata.Revision, 1
	view := equipmentCall(t, service, q, equipment.OK)
	reading, profile, code := j.st.RadioCurrent(ctx, companyX, target.Device.ID)
	pp, _ := j.st.PPCurrent(ctx, companyX, target.Device.ID)
	t.Logf("rename: revision=%d radio_compatible=%t profile_confirmed=%t opens=%d seals=%d",
		view.Credential.Revision, reading != nil && reading.Compatible, profile != nil && profile.Confirmed,
		observed.opens, observed.seals)
	validRadio := reading != nil && reading.Compatible && profile != nil && profile.Confirmed
	if code != equipment.OK || view.Credential.Revision != q.Revision || !validRadio {
		t.Fatal("rename invalidated saved radio context")
	}
	if pp.Current == nil || pp.Current.ID != observation.ID {
		t.Fatal("rename hid saved PPPoE observation")
	}
	equipmentCall(t, service, q, equipment.OK)
	stale := q
	stale.Operation, stale.Revision, stale.Alias = uuid.NewString(), 0, "stale synthetic"
	equipmentCall(t, service, stale, equipment.Revision)
	assertRenamedCredentialStorage(t, j, before, observed, q)
}

func assertRenamedCredentialStorage(t *testing.T, j *journey, before equipment.State, observed *renameObservedVault, q equipment.Command) {
	t.Helper()
	ctx := context.Background()
	after, code := j.st.ReadEquipment(ctx, companyX)
	sameEnvelope := bytes.Equal(before.Credentials[0].Envelope, after.Credentials[0].Envelope)
	sameKey := bytes.Equal(before.WrappedKey, after.WrappedKey)
	if code != equipment.OK || observed.opens != 0 || observed.seals != 0 || !sameEnvelope || !sameKey {
		t.Fatal("metadata edit touched sealed secret")
	}
	if after.Credentials[0].Metadata.Alias != q.Alias || len(after.Operations) != len(before.Operations)+1 {
		t.Fatal("rename replay or stale revision changed state")
	}
}

func TestCredentialUserEditStillAdvancesConnectionRevision(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	observed := &renameObservedVault{Vault: vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })}
	service := equipment.New(j.svc, j.st, observed, routerssh.Adapter{}, equipmentIDs{}, time.Now)
	t.Cleanup(service.Close)
	q := equipmentScope(j)
	q.Op, q.ID, q.Operation = "credential.put", target.Credential.Metadata.ID, uuid.NewString()
	q.Alias, q.User = "Synthetic access", "different-reader"
	q.Revision, q.Affected = 1, 1
	view := equipmentCall(t, service, q, equipment.OK)
	if view.Credential == nil || view.Credential.Revision != 2 || observed.opens != 1 || observed.seals != 1 {
		t.Fatal("user change skipped connection revision")
	}
	after, code := j.st.ReadEquipment(ctx, companyX)
	if code != equipment.OK {
		t.Fatal(code)
	}
	oldSecret, oldCode := observed.Vault.Open(target.Installation, companyX, q.ID, 1, target.Credential.Envelope, target.Wrapped)
	secret, newCode := observed.Vault.Open(after.Installation, companyX, q.ID, 2, after.Credentials[0].Envelope, after.WrappedKey)
	if oldCode != equipment.OK || newCode != equipment.OK || secret != oldSecret {
		t.Fatal("user-only edit did not preserve the saved secret")
	}
}
