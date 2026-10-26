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
	"sync"
	"sync/atomic"
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
func setupEquipment(t *testing.T, j *journey, s *equipment.Service, port int) (equipment.Command, equipment.Command) {
	t.Helper()
	q := equipmentScope(j)
	q.Op = "credential.put"
	q.ID = uuid.NewString()
	q.Operation = uuid.NewString()
	q.Alias = "Shared read"
	q.User = "reader"
	q.Secret = sshfixture.Password
	equipmentCall(t, s, q, equipment.OK)
	d := equipmentScope(j)
	d.Op = "inventory.create"
	d.ID = uuid.NewString()
	d.Operation = uuid.NewString()
	d.Name = "Router fixture"
	d.Address = "127.0.0.1"
	d.Port = port
	d.Transport = "ssh"
	d.CredentialID = q.ID
	equipmentCall(t, s, d, equipment.OK)
	return q, d
}
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
func awaitJob(t *testing.T, s *equipment.Service, q equipment.Command, id string) equipment.Job {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	q.Op = "device.query-status"
	q.JobID = id
	for time.Now().Before(deadline) {
		v := equipmentCall(t, s, q, equipment.OK)
		if v.Job.State != "RUNNING" {
			return *v.Job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return equipment.Job{}
}
func trustEquipment(t *testing.T, s *equipment.Service, q equipment.Command, d equipment.Command) equipment.Command {
	t.Helper()
	q.Op = "trust.inspect"
	q.ID = d.ID
	q.Revision = 1
	q.Operation = uuid.NewString()
	v := equipmentCall(t, s, q, equipment.OK)
	job := awaitJob(t, s, q, v.Job.ID)
	if job.Code != equipment.OK {
		t.Fatal(job.Code)
	}
	q.Op = "trust.confirm"
	q.Operation = uuid.NewString()
	q.JobID = job.ID
	q.Fingerprint = job.Fingerprint
	equipmentCall(t, s, q, equipment.OK)
	equipmentCall(t, s, q, equipment.OK)
	q.Op = "device.query-start"
	q.Revision = 2
	q.Operation = uuid.NewString()
	return q
}
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
func TestEquipmentQueriesRevocationSaturationAndRestart(t *testing.T) {
	for _, change := range []string{"success", "company", "logout", "reset", "renew", "credential", "inactive", "device", "cancel", "owner", "saturation"} {
		t.Run(change, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.create(companyY, "Y", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			hold := make(chan struct{})
			server, e := sshfixture.Start(sshfixture.Config{Family: "v7", Hold: hold})
			if e != nil {
				t.Fatal(e)
			}
			defer server.Close()
			credential, d := setupEquipment(t, j, s, server.Port())
			q := trustEquipment(t, s, equipmentScope(j), d)
			extra := []equipment.Command{}
			if change == "saturation" {
				for range 2 {
					peer, e := sshfixture.Start(sshfixture.Config{Family: "v7", Hold: hold})
					if e != nil {
						t.Fatal(e)
					}
					defer peer.Close()
					second := d
					second.Port = peer.Port()
					second.ID = uuid.NewString()
					second.Operation = uuid.NewString()
					equipmentCall(t, s, second, equipment.OK)
					extra = append(extra, trustEquipment(t, s, equipmentScope(j), second))
				}
			}
			v := equipmentCall(t, s, q, equipment.OK)
			same := equipmentCall(t, s, q, equipment.OK)
			if v.Job.ID != same.Job.ID {
				t.Fatal("retry duplicated job")
			}
			deadline := time.Now().Add(time.Second)
			for server.Commands.Load() == 0 && time.Now().Before(deadline) {
				time.Sleep(time.Millisecond)
			}
			if server.Commands.Load() == 0 {
				t.Fatal("query did not reach controlled peer")
			}
			switch change {
			case "company":
				j.selectCompany(companyY, access.OK)
			case "logout":
				x := j.q
				x.Op = "access.logout"
				j.call(x, access.OK)
			case "reset":
				j.call(access.Command{Op: "access.recover", Owner: "owner", Name: "technical", Password: password, Recovery: j.recovery}, access.OK)
			case "renew":
				x := j.q
				x.Op = "access.renew-recovery"
				x.Password = password
				j.recovery = j.call(x, access.OK).Recovery
				x.Op = "access.ack-recovery"
				j.call(x, access.OK)
			case "credential":
				credential.Operation = uuid.NewString()
				credential.Revision = 1
				credential.Affected = 1
				equipmentCall(t, s, credential, equipment.OK)
			case "inactive":
				x := equipmentScope(j)
				x.Op = "inventory.set-active"
				x.ID = d.ID
				x.Revision = 2
				x.Operation = uuid.NewString()
				equipmentCall(t, s, x, equipment.OK)
			case "device":
				d.Op = "inventory.update"
				d.Operation = uuid.NewString()
				d.Revision = 2
				d.Name = "Changed"
				equipmentCall(t, s, d, equipment.OK)
			case "cancel":
				x := q
				x.Op = "device.query-cancel"
				x.JobID = v.Job.ID
				equipmentCall(t, s, x, equipment.OK)
			case "owner":
				sAuth := j.svc
				sAuth.Reap(func(string) bool { return false })
			case "saturation":
				q2 := extra[0]
				equipmentCall(t, s, q2, equipment.OK)
				q2 = extra[1]
				equipmentCall(t, s, q2, access.Busy)
			}
			if change != "success" && change != "saturation" {
				time.Sleep(90 * time.Millisecond)
			}
			close(hold)
			if change == "company" || change == "logout" || change == "reset" || change == "owner" || change == "renew" {
				time.Sleep(60 * time.Millisecond)
				x := q
				x.Op = "device.query-status"
				x.JobID = v.Job.ID
				_, c := s.Handle(context.Background(), x)
				if c == equipment.OK {
					t.Fatal("revoked scope can read job")
				}
			} else {
				job := awaitJob(t, s, q, v.Job.ID)
				if change == "success" || change == "saturation" {
					if job.Code != equipment.OK || job.Result == nil {
						t.Fatal("valid query failed", job.Code)
					}
				} else if job.Result != nil || job.Code == equipment.OK {
					t.Fatal("stale query published", change)
				}
			}
			// Inspect persisted result only in X, after all workers stop.
			s.Close()
			if change == "company" {
				j.selectCompany(companyX, access.OK)
			}
			st, c := j.st.ReadEquipment(context.Background(), companyX)
			if c != equipment.OK {
				t.Fatal(c)
			}
			if change != "success" && change != "saturation" && st.Devices[0].Result != nil {
				t.Fatal("stale observation persisted", change)
			}
			if len(st.Credentials) != 1 {
				t.Fatal("authorization change destroyed vault")
			}
			if change == "success" {
				restarted := newEquipment(t, j)
				equipmentCall(t, restarted, q, "JOB_INTERRUPTED")
				if server.Auth.Load() != 1 {
					t.Fatal("restart retried authentication")
				}
			}
		})
	}
}
func TestEquipmentConcurrentOptimisticUpdateAndMissingVault(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	_, d := setupEquipment(t, j, s, 22)
	d.Op = "inventory.update"
	d.Revision = 1
	var wg sync.WaitGroup
	codes := make(chan equipment.Code, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			q := d
			q.Operation = uuid.NewString()
			_, c := s.Handle(context.Background(), q)
			codes <- c
		}()
	}
	wg.Wait()
	close(codes)
	success, conflict := 0, 0
	for c := range codes {
		if c == equipment.OK {
			success++
		}
		if c == equipment.Revision {
			conflict++
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("optimistic revision did not serialize")
	}
	missing := equipment.New(j.svc, j.st, vault.New(func() ([]byte, error) { return nil, fmt.Errorf("missing") }), routerssh.Adapter{}, equipmentIDs{}, time.Now)
	defer missing.Close()
	q := equipmentScope(j)
	q.Op = "inventory.list"
	v := equipmentCall(t, missing, q, equipment.OK)
	if v.VaultAvailable || len(v.Devices) != 1 {
		t.Fatal("missing vault blocked inventory")
	}
}

func TestEquipmentCoherentRestoreAndSchemaThree(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	credential, _ := setupEquipment(t, j, s, 22)
	st, _ := j.st.ReadEquipment(context.Background(), companyX)
	installation := st.Installation
	// Simulate an exact V2 unit, then exercise additive V3 rollback and upgrade.
	dropPPPoEFixtureSchema(t, j)
	if _, e := j.st.unit.Exec("DROP TABLE devices;DROP TABLE credentials;DROP TABLE vault_key;DROP TABLE equipment_operations;DELETE FROM schema_migrations WHERE version>=3;CREATE TABLE devices(conflict TEXT)"); e != nil {
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
	credential, _ = setupEquipment(t, j, s, 22)
	s.Close()
	source := j.st.dir
	j.st.Close()
	// Copy only after clean close: no omitted active WAL is presented as backup.
	restored := privateDir(t)
	e := filepath.WalkDir(source, func(path string, d os.DirEntry, e error) error {
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
	if e != nil {
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
func TestEquipmentInspectNonblockingAndRealTotalDeadline(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	peer, e := sshfixture.Start(sshfixture.Config{Mode: "handshake"})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	_, d := setupEquipment(t, j, s, peer.Port())
	q := equipmentScope(j)
	q.Op = "trust.inspect"
	q.ID = d.ID
	q.Revision = 1
	q.Operation = uuid.NewString()
	began := time.Now()
	v := equipmentCall(t, s, q, equipment.OK)
	if time.Since(began) > time.Second {
		t.Fatal("inspect blocked the caller")
	}
	snapshot := j.q
	snapshot.Op = "access.snapshot"
	j.call(snapshot, access.OK)
	// The actual 20s budget is exercised once per run without changing production limits.
	until := time.Now().Add(equipment.QueryBudget + 2*time.Second)
	q.Op = "device.query-status"
	q.JobID = v.Job.ID
	for time.Now().Before(until) {
		r := equipmentCall(t, s, q, equipment.OK)
		if r.Job.State != "RUNNING" {
			if r.Job.Code != "DEADLINE_EXCEEDED" {
				t.Fatal("wrong terminal deadline", r.Job.Code)
			}
			if elapsed := time.Since(began); elapsed < equipment.QueryBudget || elapsed > equipment.QueryBudget+time.Second {
				t.Fatal("unbounded total deadline", elapsed)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("job exceeded total deadline")
}

func TestEquipmentJobRetentionAndRecordLimit(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	var clock atomic.Value
	clock.Store(time.Now())
	s := equipment.New(j.svc, j.st, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil }), routerssh.Adapter{}, equipmentIDs{}, func() time.Time { return clock.Load().(time.Time) })
	defer s.Close()
	peer, e := sshfixture.Start(sshfixture.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	_, d := setupEquipment(t, j, s, peer.Port())
	q := equipmentScope(j)
	q.Op = "trust.inspect"
	q.ID = d.ID
	q.Revision = 1
	firstOperation, firstJob := "", ""
	// Terminal metadata precedes worker exit. Wait on the same deliberate
	// operation for admission release; BUSY does not reserve/repeat network.
	admit := func(want equipment.Code) equipment.View {
		deadline := time.Now().Add(3 * time.Second) // Same existing awaitJob budget.
		for {
			v, c := s.Handle(context.Background(), q)
			if c != access.Busy {
				if c != want {
					t.Fatalf("retention admission got %s want %s", c, want)
				}
				return v
			}
			if !time.Now().Before(deadline) {
				t.Fatal("worker admission did not release")
			}
			time.Sleep(time.Millisecond)
		}
	}
	for i := 0; i < equipment.MaxJobs; i++ {
		q.Operation = uuid.NewString()
		v := admit(equipment.OK)
		if i == 0 {
			firstOperation = q.Operation
			firstJob = v.Job.ID
		}
		job := awaitJob(t, s, q, v.Job.ID)
		if job.Code != equipment.OK {
			t.Fatal(job.Code)
		}
	}
	q.Operation = uuid.NewString()
	admit(equipment.Limit)
	clock.Store(clock.Load().(time.Time).Add(equipment.JobRetention + time.Second))
	expired := q
	expired.Op = "device.query-status"
	expired.JobID = firstJob
	equipmentCall(t, s, expired, equipment.Changed)
	equipmentCall(t, s, q, equipment.OK)
	q.Operation = firstOperation
	equipmentCall(t, s, q, "JOB_INTERRUPTED")
	if peer.Auth.Load() != 0 {
		t.Fatal("inspection/retention authenticated")
	}
}

func TestFocalMissingCompanyKeyPreservesInventory(t *testing.T) {
	for _, sql := range []string{"DELETE FROM vault_key", "UPDATE vault_key SET envelope=x''"} {
		t.Run(sql, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			cr, _ := setupEquipment(t, j, s, 2222)
			before, _ := j.st.ReadEquipment(context.Background(), companyX)
			if _, e := j.st.unit.Exec(sql); e != nil {
				t.Fatal(e)
			}
			q := equipmentScope(j)
			q.Op = "inventory.list"
			v := equipmentCall(t, s, q, equipment.OK)
			if v.VaultAvailable || len(v.Devices) != 1 {
				t.Fatal("missing company key advertised ready or inventory lost")
			}
			cr.ID = uuid.NewString()
			cr.Operation = uuid.NewString()
			equipmentCall(t, s, cr, equipment.VaultUnavailable)
			after, _ := j.st.ReadEquipment(context.Background(), companyX)
			if len(after.WrappedKey) != 0 || len(after.Credentials) != 1 || !bytes.Equal(before.Credentials[0].Envelope, after.Credentials[0].Envelope) || len(before.Operations) != len(after.Operations) {
				t.Fatal("missing key caused destructive write")
			}
		})
	}
}
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
