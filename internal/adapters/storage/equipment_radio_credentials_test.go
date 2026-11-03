package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"reflect"
	"testing"
)

func TestRadioDefaultsAndTOFU(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	peer, err := sshfixture.Start(sshfixture.Config{Family: "v6"})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	d, ap := prepareDefaultAPForTOFU(t, j, s, peer.Port())
	inspect := equipmentScope(j)
	inspect.Op = "trust.inspect"
	inspect.ID = d.ID
	inspect.Revision = 1
	inspect.Operation = uuid.NewString()
	job := equipmentCall(t, s, inspect, equipment.OK).Job
	done := awaitJob(t, s, inspect, job.ID)
	if done.Code != equipment.OK {
		t.Fatal(done)
	}
	st, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	for _, v := range st.Devices {
		if v.ID == d.ID {
			ap = v
		}
	}
	if ap.Fingerprint != peer.Fingerprint() ||
		ap.TrustAcceptance == nil ||
		ap.TrustAcceptance.Mode != "automatic-tofu" ||
		ap.TrustAcceptance.At == "" {
		t.Fatalf("TOFU not recorded: %+v", ap)
	}
	if peer.Auth.Load() != 0 {
		t.Fatal("password sent during TOFU")
	}
	if err = peer.RotateHostKey(); err != nil {
		t.Fatal(err)
	}
	inspect.Operation = uuid.NewString()
	inspect.Revision = ap.Revision
	done = awaitJob(t, s, inspect, equipmentCall(t, s, inspect, equipment.OK).Job.ID)
	if done.Code != equipment.TrustChanged || peer.Auth.Load() != 0 {
		t.Fatal("changed pin not blocked", done.Code, peer.Auth.Load())
	}
}

func TestRadioRefusedCredentialNeedsExplicitReplacement(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	peer, err := sshfixture.Start(sshfixture.Config{Family: "v6"})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	chosen, d := setupEquipment(t, j, s, peer.Port())
	other := refuseAssociatedRadioCredential(t, j, s, peer, chosen, d)
	query := equipmentScope(j)
	query.Op = "device.query-start"
	query.ID = d.ID
	query.Revision = 1
	query.Operation = uuid.NewString()
	refused := awaitJob(t, s, query, equipmentCall(t, s, query, equipment.OK).Job.ID)
	if refused.Code != equipment.RadioCodeRemoteAuthFailed || peer.Auth.Load() != 1 {
		t.Fatal("refused credential retried/fell back", refused.Code, peer.Auth.Load())
	}
	// Only this explicit user choice changes the persisted association.
	d.Op = "inventory.update"
	d.Revision = 1
	d.CredentialID = other.ID
	d.Operation = uuid.NewString()
	equipmentCall(t, s, d, equipment.OK)
	if peer.Auth.Load() != 1 {
		t.Fatal("saving dropdown choice started a connection")
	}
	get := equipmentScope(j)
	get.Op = "inventory.get"
	get.ID = d.ID
	saved := equipmentCall(t, s, get, equipment.OK).Device
	if saved.CredentialID != other.ID {
		t.Fatal("choice not remembered")
	}
	query.Revision = saved.Revision
	query.Operation = uuid.NewString()
	allowed := awaitJob(t, s, query, equipmentCall(t, s, query, equipment.OK).Job.ID)
	if allowed.Code != equipment.OK || peer.Auth.Load() != 2 {
		t.Fatal("explicitly associated credential failed", allowed.Code, peer.Auth.Load())
	}
}

func TestRadioMigration13PreservesFixtureAndRollsBack(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	setupEquipment(t, j, s, 2222)
	before, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	if _, err := j.st.unit.Exec("DROP TABLE radio_credential_defaults; DELETE FROM schema_migrations WHERE version=13"); err != nil {
		t.Fatal(err)
	}
	original := companyRadioJourneySchema
	defer func() { companyRadioJourneySchema = original }()
	companyRadioJourneySchema = original + ";invalid SQL;"
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err == nil {
		t.Fatal("invalid migration accepted")
	}
	var n int
	if err := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='radio_credential_defaults'").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial migration escaped", err, n)
	}
	companyRadioJourneySchema = original
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err != nil {
		t.Fatal(err)
	}
	after, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed credentials/vault/inventory/operations")
	}
	if err := j.st.unit.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&n); err != nil || n != 13 {
		t.Fatal("schema13 missing", n, err)
	}
}

func TestRadioClassDefaultsPersistAPCPEPTP(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	cr, base := setupEquipment(t, j, s, 2222)
	for _, role := range []equipment.Role{equipment.RoleAP, equipment.RoleCPE, equipment.RolePTP} {
		q := equipmentScope(j)
		q.Op = "credential.default-set"
		q.ID = cr.ID
		q.CredentialID = cr.ID
		q.Role = role
		q.Operation = uuid.NewString()
		equipmentCall(t, s, q, equipment.OK)
		d := base
		d.ID = uuid.NewString()
		d.Operation = uuid.NewString()
		d.Role = role
		d.CredentialID = ""
		v := equipmentCall(t, s, d, equipment.OK)
		if v.Device == nil || v.Device.CredentialID != cr.ID || v.Device.Role != role {
			t.Fatal("class default missing", role, v.Device)
		}
	}
	j.selectCompany(companyX, access.OK)
	list := equipmentScope(j)
	list.Op = "credential.list"
	v := equipmentCall(t, s, list, equipment.OK)
	for _, role := range []equipment.Role{equipment.RoleAP, equipment.RoleCPE, equipment.RolePTP} {
		if v.CredentialDefaults[string(role)] != cr.ID {
			t.Fatal("default not persisted", role)
		}
	}
}
