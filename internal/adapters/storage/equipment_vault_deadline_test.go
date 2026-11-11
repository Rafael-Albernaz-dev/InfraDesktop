package storage

import (
	"bytes"
	"context"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"infradesktop/internal/testsupport/testwait"
	"sync"
	"testing"
	"time"
)

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
	d.Transport = equipment.TransportSSH
	d.CredentialID = q.ID
	equipmentCall(t, s, d, equipment.OK)
	return q, d
}

func awaitJob(t *testing.T, s *equipment.Service, q equipment.Command, id string) equipment.Job {
	t.Helper()
	deadline := testwait.Limit(t)
	q.Op = "device.query-status"
	q.JobID = id
	for time.Now().Before(deadline) {
		v := equipmentCall(t, s, q, equipment.OK)
		if v.Job.State != "RUNNING" && fixtureJobReleased(s, id) {
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
	absent := vault.New(func() ([]byte, error) { return nil, fmt.Errorf("missing") })
	missing := equipment.New(j.svc, j.st, absent, routerssh.Adapter{}, equipmentIDs{}, time.Now)
	defer missing.Close()
	q := equipmentScope(j)
	q.Op = "inventory.list"
	v := equipmentCall(t, missing, q, equipment.OK)
	if v.VaultAvailable || len(v.Devices) != 1 {
		t.Fatal("missing vault blocked inventory")
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
			if r.Job.Code != equipment.Code(foundation.Deadline) {
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

func TestMissingCompanyKeyPreservesInventory(t *testing.T) {
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
			sameEnvelope := bytes.Equal(before.Credentials[0].Envelope, after.Credentials[0].Envelope)
			if len(after.WrappedKey) != 0 || len(after.Credentials) != 1 || !sameEnvelope || len(before.Operations) != len(after.Operations) {
				t.Fatal("missing key caused destructive write")
			}
		})
	}
}
