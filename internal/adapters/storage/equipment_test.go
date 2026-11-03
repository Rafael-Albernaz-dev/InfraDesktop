package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

type equipmentIDs struct{}

func (equipmentIDs) ID() (string, equipment.Code) { return uuid.NewString(), equipment.OK }
func newEquipment(t *testing.T, j *journey) *equipment.Service {
	t.Helper()
	s := equipment.New(j.svc, j.st, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil }), routerssh.Adapter{}, equipmentIDs{}, time.Now)
	t.Cleanup(s.Close)
	return s
}
func equipmentScope(j *journey) equipment.Command {
	q := j.q
	q.ExpectedProfile = q.Profile
	q.ExpectedActivation = q.Activation
	return equipment.Command{Scope: q, UID: 1000}
}
func equipmentCall(t *testing.T, s *equipment.Service, q equipment.Command, want equipment.Code) equipment.View {
	t.Helper()
	v, c := s.Handle(context.Background(), q)
	if c != want {
		t.Fatalf("%s got %s want %s", q.Op, c, want)
	}
	return v
}

// setupEquipment moved to equipment_vault_deadline_test.go.

func TestEquipmentPersistenceIsolationConflictAndRollback(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	credential, d := setupEquipment(t, j, s, 2222)
	equipmentCall(t, s, d, equipment.OK)
	changed := d
	changed.Name = "conflicting retry"
	equipmentCall(t, s, changed, equipment.Revision)
	stale := d
	stale.Op = "inventory.update"
	stale.Operation = uuid.NewString()
	stale.Revision = 0
	equipmentCall(t, s, stale, equipment.Revision)
	d2 := d
	d2.ID = uuid.NewString()
	d2.Operation = uuid.NewString()
	equipmentCall(t, s, d2, equipment.OK)
	list := equipmentScope(j)
	list.Op = "credential.list"
	if v := equipmentCall(t, s, list, equipment.OK); len(v.Credentials) != 1 || v.Credentials[0].Links != 2 {
		t.Fatal("shared references wrong")
	}
	replace := credential
	replace.Operation = uuid.NewString()
	replace.Revision = 1
	replace.Secret = "replacement test only"
	replace.Affected = 1
	equipmentCall(t, s, replace, equipment.Revision)
	replace.Affected = 2
	// Failing SQL rolls back ciphertext, revision, ledger and the wrapped key.
	if _, e := j.st.unit.Exec("CREATE TRIGGER reject_credential BEFORE UPDATE ON credentials BEGIN SELECT RAISE(ABORT,'test'); END"); e != nil {
		t.Fatal(e)
	}
	equipmentCall(t, s, replace, equipment.Unavailable)
	st, _ := j.st.ReadEquipment(context.Background(), companyX)
	if st.Credentials[0].Metadata.Revision != 1 {
		t.Fatal("failed commit changed revision")
	}
	j.st.unit.Exec("DROP TRIGGER reject_credential")
	equipmentCall(t, s, replace, equipment.OK)
	equipmentCall(t, s, replace, equipment.OK)
	off := equipmentScope(j)
	off.Op = "inventory.set-active"
	off.ID = d.ID
	off.Operation = uuid.NewString()
	off.Revision = 1
	off.Active = false
	equipmentCall(t, s, off, equipment.OK)
	list.Op = "inventory.list"
	if v := equipmentCall(t, s, list, equipment.OK); len(v.Devices) != 1 || v.Devices[0].ID != d2.ID {
		t.Fatal("deactivation affected unrelated device")
	}
	list.Inactive = true
	if len(equipmentCall(t, s, list, equipment.OK).Devices) != 1 {
		t.Fatal("inactive missing")
	}
	off.Operation = uuid.NewString()
	off.Revision = 2
	off.Active = true
	equipmentCall(t, s, off, equipment.OK)
	oldScope := list
	j.selectCompany(companyY, access.OK)
	equipmentCall(t, s, oldScope, equipment.Changed)
	y := equipmentScope(j)
	y.Op = "inventory.list"
	if len(equipmentCall(t, s, y, equipment.OK).Devices) != 0 {
		t.Fatal("X leaked into Y")
	}
	yc, yd := setupEquipment(t, j, s, 2222)
	if yc.ID == credential.ID || yd.ID == d.ID {
		t.Fatal("identity reused")
	}
	y.CredentialID = credential.ID
	y.ID = uuid.NewString()
	y.Operation = uuid.NewString()
	y.Op = "inventory.create"
	y.Name = "foreign"
	y.Address = "127.0.0.1"
	y.Port = 2222
	y.Transport = "ssh"
	equipmentCall(t, s, y, equipment.MissingCredential)
	j.selectCompany(companyX, access.OK)
	// Reopen a coherent closed database set and use the same isolated provider.
	s.Close()
	dir := j.st.dir
	j.st.Close()
	store, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	j.st = store
	j.svc = access.New(store, security.Crypto{}, time.Now, 1000, "restart")
	v, c := j.svc.Handle(context.Background(), 1000, access.Command{Op: "access.login", Owner: "owner", Name: "technical", Password: password})
	if c != access.OK {
		t.Fatal(c)
	}
	j.q = access.Command{Session: v.Session, Owner: "owner", Profile: v.Profile, Activation: v.Activation}
	s = newEquipment(t, j)
	q := equipmentScope(j)
	q.Op = "inventory.list"
	if len(equipmentCall(t, s, q, equipment.OK).Devices) != 2 {
		t.Fatal("restart lost devices")
	}
	st, c = store.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	plain, vc := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil }).Open(st.Installation, st.Company, credential.ID, 2, st.Credentials[0].Envelope, st.WrappedKey)
	if vc != equipment.OK || plain != "replacement test only" {
		t.Fatal("coherent restore cannot decrypt")
	}
	if _, vc = vault.New(func() ([]byte, error) { return nil, fmt.Errorf("missing") }).Open(st.Installation, st.Company, credential.ID, 2, st.Credentials[0].Envelope, st.WrappedKey); vc != equipment.VaultUnavailable {
		t.Fatal("restore without key accepted")
	}
	// Scan all company/global files and live WAL/SHM, not only logical rows.
	if e = filepath.WalkDir(dir, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		if bytes.Contains(b, []byte(sshfixture.Password)) || bytes.Contains(b, []byte("replacement test only")) {
			t.Fatal("remote plaintext reached storage")
		}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
}

// awaitJob moved to equipment_vault_deadline_test.go.

// trustEquipment moved to equipment_vault_deadline_test.go.

func TestEquipmentActivationPreservesObservation(t *testing.T) {
	for _, scenario := range []string{"completed", "query-in-flight"} {
		t.Run(scenario, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			// Permit one complete reading, then block the next query's first command.
			hold := make(chan struct{}, 3)
			for range 3 {
				hold <- struct{}{}
			}
			peer, e := sshfixture.Start(sshfixture.Config{Family: "v7", Hold: hold})
			if e != nil {
				t.Fatal(e)
			}
			defer peer.Close()
			defer close(hold)
			_, d := setupEquipment(t, j, s, peer.Port())
			q := trustEquipment(t, s, equipmentScope(j), d)
			first := equipmentCall(t, s, q, equipment.OK)
			job := awaitJob(t, s, q, first.Job.ID)
			if job.State != "SUCCEEDED" || job.Code != equipment.OK || job.Result == nil || len(job.Result.Interfaces) == 0 || job.Result.At == "" {
				t.Fatal("initial complete reading missing")
			}
			get := equipmentScope(j)
			get.Op, get.ID = "inventory.get", d.ID
			expected := *equipmentCall(t, s, get, equipment.OK).Device
			if !reflect.DeepEqual(expected.Result, job.Result) {
				t.Fatal("initial reading was not persisted")
			}
			var pending string
			if scenario == "query-in-flight" {
				q.Operation = uuid.NewString()
				pending = equipmentCall(t, s, q, equipment.OK).Job.ID
				deadline := time.Now().Add(time.Second)
				for peer.Commands.Load() < 4 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if peer.Commands.Load() != 4 {
					t.Fatal("second query did not reach controlled blocking point")
				}
			}
			expected.LastAttempt = equipmentCall(t, s, get, equipment.OK).Device.LastAttempt
			auths := peer.Auth.Load()
			check := func(v *equipment.Device) {
				t.Helper()
				if !reflect.DeepEqual(v, &expected) {
					t.Fatalf("activation changed completed observation or linked device data: got %#v expected %#v", v, &expected)
				}
				st, c := j.st.ReadEquipment(context.Background(), companyX)
				if c != equipment.OK || len(st.Devices) != 1 || !reflect.DeepEqual(st.Devices[0], expected) {
					t.Fatal("stored observation or linked device data changed")
				}
			}
			toggle := equipmentScope(j)
			toggle.Op, toggle.ID = "inventory.set-active", d.ID
			toggle.Operation, toggle.Revision = uuid.NewString(), expected.Revision
			expected.Active, expected.Revision = false, expected.Revision+1
			check(equipmentCall(t, s, toggle, equipment.OK).Device)
			if pending != "" {
				status := q
				status.Op, status.JobID = "device.query-status", pending
				got := equipmentCall(t, s, status, equipment.OK).Job
				if got.State != "CANCELLED" || got.Code != equipment.Cancelled || got.Result != nil {
					t.Fatal("deactivation did not cancel in-flight query")
				}
			}
			// Retrying the same write must preserve both reading and revision.
			check(equipmentCall(t, s, toggle, equipment.OK).Device)
			stale := toggle
			stale.Operation = uuid.NewString()
			equipmentCall(t, s, stale, equipment.Revision)
			check(equipmentCall(t, s, get, equipment.OK).Device)
			q.Operation, q.Revision = uuid.NewString(), expected.Revision
			equipmentCall(t, s, q, equipment.Inactive)
			list := equipmentScope(j)
			list.Op = "inventory.list"
			if len(equipmentCall(t, s, list, equipment.OK).Devices) != 0 {
				t.Fatal("inactive device remained in active list")
			}
			list.Inactive = true
			items := equipmentCall(t, s, list, equipment.OK).Devices
			summary := *expected.Result
			summary.Interfaces = nil
			if len(items) != 1 || !reflect.DeepEqual(items[0].Result, &summary) {
				t.Fatal("inactive list lost last reading summary")
			}
			toggle.Operation, toggle.Revision, toggle.Active = uuid.NewString(), expected.Revision, true
			expected.Active, expected.Revision = true, expected.Revision+1
			check(equipmentCall(t, s, toggle, equipment.OK).Device)
			if peer.Auth.Load() != auths {
				t.Fatal("activation or refused query performed automatic authentication")
			}
			// Join cancelled workers before checking that no late result was published.
			s.Close()
			check(equipmentCall(t, s, get, equipment.OK).Device)
			dir := j.st.dir
			j.st.Close()
			j.st, e = Open(dir)
			if e != nil {
				t.Fatal(e)
			}
			j.svc = access.New(j.st, security.Crypto{}, time.Now, 1000, "restart")
			v := j.call(access.Command{Op: "access.login", Owner: "owner", Name: "technical", Password: password}, access.OK)
			j.q = access.Command{Session: v.Session, Owner: "owner", Profile: v.Profile, Activation: v.Activation}
			s = newEquipment(t, j)
			get = equipmentScope(j)
			get.Op, get.ID = "inventory.get", d.ID
			check(equipmentCall(t, s, get, equipment.OK).Device)
			// A genuine endpoint edit still invalidates its old trust and observation.
			edit := equipmentScope(j)
			edit.Op, edit.ID, edit.Operation = "inventory.update", d.ID, uuid.NewString()
			edit.Revision, edit.Name = expected.Revision, expected.Name
			edit.Address, edit.Port, edit.Transport = "127.0.0.2", expected.Port, expected.Transport
			edit.CredentialID = expected.CredentialID
			expected.Address, expected.Revision = edit.Address, expected.Revision+1
			expected.Fingerprint, expected.TrustOrigin, expected.Result = "", "", nil
			expected.TrustAcceptance, expected.LastAttempt = nil, nil
			check(equipmentCall(t, s, edit, equipment.OK).Device)
		})
	}
}

// TestEquipmentQueriesRevocationSaturationAndRestart moved to equipment_job_replay_test.go.

// TestEquipmentConcurrentOptimisticUpdateAndMissingVault moved to equipment_vault_deadline_test.go.

func TestEquipmentCoherentRestoreAndSchemaThree(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	setupEquipment(t, j, s, 22)
	st, _ := j.st.ReadEquipment(context.Background(), companyX)
	installation := st.Installation
	assertSchemaThreeRollbackAndUpgrade(t, j)
	credential, _ := setupEquipment(t, j, s, 22)
	s.Close()
	source := j.st.dir
	j.st.Close()
	// Copy only after clean close: no omitted active WAL is presented as backup.
	restored := privateDir(t)
	if e := copyUnitWithoutLock(source, restored); e != nil {
		t.Fatal(e)
	}
	restoredStore, e := Open(restored)
	if e != nil {
		t.Fatal(e)
	}
	defer restoredStore.Close()
	st, c := restoredStore.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK || st.Installation != installation || st.Company != companyX {
		t.Fatal("restored identities differ")
	}
	v := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	plain, c := v.Open(st.Installation, st.Company, credential.ID, 1, st.Credentials[0].Envelope, st.WrappedKey)
	if c != equipment.OK || plain != sshfixture.Password {
		t.Fatal("coherent set failed to restore")
	}
	missing := vault.New(func() ([]byte, error) { return nil, fmt.Errorf("missing") })
	if missing.Ready(st.Installation, st.Company, st.WrappedKey) {
		t.Fatal("missing key reported ready")
	}
	wrong := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{38}, 32), nil })
	if wrong.Ready(st.Installation, st.Company, st.WrappedKey) {
		t.Fatal("wrong master reported ready")
	}
}

// assertSchemaThreeRollbackAndUpgrade simulates an exact V2 unit, then exercises
// the additive V3 rollback and upgrade.
func assertSchemaThreeRollbackAndUpgrade(t *testing.T, j *journey) {
	t.Helper()
	dropPPPoEFixtureSchema(t, j)
	stmt := "DROP TABLE devices;DROP TABLE credentials;DROP TABLE vault_key;DROP TABLE equipment_operations;" +
		"DELETE FROM schema_migrations WHERE version>=3;CREATE TABLE devices(conflict TEXT)"
	if _, e := j.st.unit.Exec(stmt); e != nil {
		t.Fatal(e)
	}
	if j.st.validateUnit(j.st.unit, companyX) == nil {
		t.Fatal("conflicting schema accepted")
	}
	var count int
	j.st.unit.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&count)
	if count != 2 {
		t.Fatal("failed migration advanced ledger")
	}
	j.st.unit.Exec("DROP TABLE devices")
	if e := j.st.validateUnit(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
}

// copyUnitWithoutLock copies a closed unit directory, skipping the process lock.
func copyUnitWithoutLock(source, restored string) error {
	return filepath.WalkDir(source, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		relative, _ := filepath.Rel(source, path)
		if relative == "." {
			return nil
		}
		target := filepath.Join(restored, relative)
		if d.IsDir() {
			return os.Mkdir(target, 0700)
		}
		if filepath.Base(path) == "database.lock" {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		return os.WriteFile(target, b, 0600)
	})
}

// TestEquipmentInspectNonblockingAndRealTotalDeadline moved to equipment_vault_deadline_test.go.

// TestEquipmentJobRetentionAndRecordLimit moved to equipment_job_replay_test.go.

// TestFocalMissingCompanyKeyPreservesInventory moved to equipment_vault_deadline_test.go.

func TestFocalWriteReplayPreservesJobs(t *testing.T) {
	for _, kind := range []string{"device", "credential"} {
		t.Run(kind, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			hold := make(chan struct{})
			peer, e := sshfixture.Start(sshfixture.Config{Family: "v7", Hold: hold})
			if e != nil {
				t.Fatal(e)
			}
			defer peer.Close()
			defer func() {
				select {
				case <-hold:
				default:
					close(hold)
				}
			}()
			cr, d := setupEquipment(t, j, s, peer.Port())
			q := trustEquipment(t, s, equipmentScope(j), d)
			replay := d
			if kind == "credential" {
				replay = cr
			}
			var second equipment.Command
			if kind == "credential" {
				secondPeer, e := sshfixture.Start(sshfixture.Config{Family: "v7", Hold: hold})
				if e != nil {
					t.Fatal(e)
				}
				defer secondPeer.Close()
				d2 := d
				d2.Port = secondPeer.Port()
				d2.ID = uuid.NewString()
				d2.Operation = uuid.NewString()
				equipmentCall(t, s, d2, equipment.OK)
				second = trustEquipment(t, s, equipmentScope(j), d2)
			}
			v := equipmentCall(t, s, q, equipment.OK)
			var secondID string
			if kind == "credential" {
				secondID = equipmentCall(t, s, second, equipment.OK).Job.ID
			}
			equipmentCall(t, s, replay, equipment.OK)
			if secondID != "" {
				x := second
				x.Op = "device.query-status"
				x.JobID = secondID
				if equipmentCall(t, s, x, equipment.OK).Job.State != "RUNNING" {
					t.Fatal("shared replay invalidated other device")
				}
			}
			status := q
			status.Op = "device.query-status"
			status.JobID = v.Job.ID
			if job := equipmentCall(t, s, status, equipment.OK).Job; job.State != "RUNNING" {
				t.Fatal("replay invalidated running query", job)
			}
			close(hold)
			job := awaitJob(t, s, q, v.Job.ID)
			if job.Code != equipment.OK {
				t.Fatal(job.Code)
			}
			if secondID != "" {
				if awaitJob(t, s, second, secondID).Code != equipment.OK {
					t.Fatal("shared query failed")
				}
			}
			equipmentCall(t, s, replay, equipment.OK)
			if secondID != "" {
				x := second
				x.Op = "device.query-status"
				x.JobID = secondID
				if got := equipmentCall(t, s, x, equipment.OK).Job; got.State != "SUCCEEDED" || got.Result == nil {
					t.Fatal("shared replay erased completed result")
				}
			}
			if got := equipmentCall(t, s, status, equipment.OK).Job; got.State != "SUCCEEDED" || got.Result == nil {
				t.Fatal("replay invalidated completed result", got)
			}
		})
	}
}
