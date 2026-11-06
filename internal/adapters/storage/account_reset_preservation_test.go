package storage

import (
	"bytes"
	"context"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
	"time"
)

type approvedResetFixture struct{ nonce string }

func (p *approvedResetFixture) Consume(nonce string) bool { return nonce != "" && nonce == p.nonce }

func TestResetPreservesSyntheticCompanyInventoryAndVault(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "Synthetic", access.OK)
	j.selectCompany(companyX, access.OK)
	service := newEquipment(t, j)
	credential, device := setupEquipment(t, j, service, 2222)
	before, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK {
		t.Fatal(code)
	}
	proof := &approvedResetFixture{}
	j.svc.SetResetProof(proof)
	begin := func() string {
		v := j.call(access.Command{Op: "access.reset-begin", Owner: "reset-owner"}, access.OK)
		proof.nonce = v.ResetNonce
		return v.ResetNonce
	}
	reset := func(nonce string, want access.Code) access.View {
		return j.call(access.Command{
			Op: "access.reset-complete", Owner: "reset-owner", Name: "new-user",
			Password: "new isolated password", ResetNonce: nonce,
		}, want)
	}
	assertResetRollback(t, j, begin, reset)
	oldScope := j.q
	issued := reset(begin(), access.OK)
	if issued.Recovery == "" || issued.Recovery == j.recovery {
		t.Fatal("recovery was not rotated")
	}
	oldScope.Op = "access.snapshot"
	j.call(oldScope, access.Unauthorized)
	j.now = j.now.Add(31 * time.Second)
	j.call(access.Command{Op: "access.login", Owner: "new-owner", Name: "technical", Password: password}, access.Unauthorized)
	j.now = j.now.Add(31 * time.Second)
	login := j.call(access.Command{Op: "access.login", Owner: "new-owner", Name: "new-user", Password: "new isolated password"}, access.OK)
	if login.Profile != companyX || len(login.Companies) != 1 {
		t.Fatal("company or selected profile lost")
	}
	j.q = access.Command{Owner: "new-owner", Session: login.Session, Profile: login.Profile, Activation: login.Activation}
	list := equipmentScope(j)
	list.Op = "inventory.list"
	got := equipmentCall(t, service, list, equipment.OK)
	if len(got.Devices) != 1 || got.Devices[0].ID != device.ID {
		t.Fatal("inventory lost")
	}
	after, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK {
		t.Fatal(code)
	}
	if !bytes.Equal(before.WrappedKey, after.WrappedKey) || !bytes.Equal(before.Credentials[0].Envelope, after.Credentials[0].Envelope) {
		t.Fatal("vault ciphertext changed")
	}
	provider := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	secret, c := provider.Open(after.Installation, after.Company, credential.ID, 1,
		after.Credentials[0].Envelope, after.WrappedKey)
	if c != equipment.OK || secret != sshfixture.Password {
		t.Fatal("synthetic vault secret changed")
	}
	assertOldRecoveryRejected(j)
}

func assertOldRecoveryRejected(j *journey) {
	j.now = j.now.Add(31 * time.Second)
	j.call(access.Command{Op: "access.recover", Owner: "new-owner", Name: "new-user",
		Password: "another isolated password", Recovery: j.recovery}, access.Unauthorized)
}

func assertResetRollback(t *testing.T, j *journey, begin func() string, reset func(string, access.Code) access.View) {
	t.Helper()
	query := "CREATE TRIGGER reject_account_reset BEFORE UPDATE ON account " +
		"BEGIN SELECT RAISE(ABORT,'synthetic'); END"
	if _, err := j.st.db.Exec(query); err != nil {
		t.Fatal(err)
	}
	failed := begin()
	reset(failed, access.Unavailable)
	state, code := j.st.Read(context.Background())
	if code != access.OK || state.Account.Name != "technical" {
		t.Fatal("failed transaction changed account")
	}
	if _, err := j.st.db.Exec("DROP TRIGGER reject_account_reset"); err != nil {
		t.Fatal(err)
	}
}
