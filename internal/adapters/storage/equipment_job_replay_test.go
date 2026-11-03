package storage

import (
	"bytes"
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
)

// revocationCase holds one held query whose authority changes mid-flight.
type revocationCase struct {
	t             *testing.T
	change        string
	j             *journey
	s             *equipment.Service
	server        *sshfixture.Server
	hold          chan struct{}
	credential, d equipment.Command
	q             equipment.Command
	extra         []equipment.Command
	v             equipment.View
}

func TestEquipmentQueriesRevocationSaturationAndRestart(t *testing.T) {
	changes := []string{"success", "company", "logout", "reset", "renew", "credential", "inactive", "device", "cancel", "owner", "saturation"}
	for _, change := range changes {
		t.Run(change, func(t *testing.T) {
			c := newRevocationCase(t, change)
			c.startHeldQuery()
			if !c.applyAccessChange() {
				c.applyInventoryChange()
			}
			c.settle()
			c.assertPersisted()
		})
	}
}

func newRevocationCase(t *testing.T, change string) *revocationCase {
	t.Helper()
	c := &revocationCase{t: t, change: change, j: newJourney(t), hold: make(chan struct{})}
	c.j.create(companyX, "X", access.OK)
	c.j.create(companyY, "Y", access.OK)
	c.j.selectCompany(companyX, access.OK)
	c.s = newEquipment(t, c.j)
	c.server = c.startHeldPeer()
	c.credential, c.d = setupEquipment(t, c.j, c.s, c.server.Port())
	c.q = trustEquipment(t, c.s, equipmentScope(c.j), c.d)
	if change == "saturation" {
		for range 2 {
			second := c.d
			second.Port = c.startHeldPeer().Port()
			second.ID = uuid.NewString()
			second.Operation = uuid.NewString()
			equipmentCall(t, c.s, second, equipment.OK)
			c.extra = append(c.extra, trustEquipment(t, c.s, equipmentScope(c.j), second))
		}
	}
	return c
}

func (c *revocationCase) startHeldPeer() *sshfixture.Server {
	peer, e := sshfixture.Start(sshfixture.Config{Family: "v7", Hold: c.hold})
	if e != nil {
		c.t.Fatal(e)
	}
	c.t.Cleanup(func() { peer.Close() })
	return peer
}

func (c *revocationCase) startHeldQuery() {
	t := c.t
	c.v = equipmentCall(t, c.s, c.q, equipment.OK)
	same := equipmentCall(t, c.s, c.q, equipment.OK)
	if c.v.Job.ID != same.Job.ID {
		t.Fatal("retry duplicated job")
	}
	deadline := time.Now().Add(time.Second)
	for c.server.Commands.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if c.server.Commands.Load() == 0 {
		t.Fatal("query did not reach controlled peer")
	}
}

// applyAccessChange revokes the account/company scope; false for other changes.
func (c *revocationCase) applyAccessChange() bool {
	j := c.j
	switch c.change {
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
	case "owner":
		sAuth := j.svc
		sAuth.Reap(func(string) bool { return false })
	default:
		return false
	}
	return true
}

func (c *revocationCase) applyInventoryChange() {
	t, s := c.t, c.s
	switch c.change {
	case "credential":
		c.credential.Operation = uuid.NewString()
		c.credential.Revision = 1
		c.credential.Affected = 1
		equipmentCall(t, s, c.credential, equipment.OK)
	case "inactive":
		x := equipmentScope(c.j)
		x.Op, x.ID, x.Revision, x.Operation = "inventory.set-active", c.d.ID, 2, uuid.NewString()
		equipmentCall(t, s, x, equipment.OK)
	case "device":
		c.d.Op, c.d.Operation, c.d.Revision, c.d.Name = "inventory.update", uuid.NewString(), 2, "Changed"
		equipmentCall(t, s, c.d, equipment.OK)
	case "cancel":
		x := c.q
		x.Op, x.JobID = "device.query-cancel", c.v.Job.ID
		equipmentCall(t, s, x, equipment.OK)
	case "saturation":
		equipmentCall(t, s, c.extra[0], equipment.OK)
		equipmentCall(t, s, c.extra[1], access.Busy)
	}
}

func (c *revocationCase) valid() bool { return c.change == "success" || c.change == "saturation" }

func (c *revocationCase) revokesScope() bool {
	switch c.change {
	case "company", "logout", "reset", "owner", "renew":
		return true
	}
	return false
}

func (c *revocationCase) settle() {
	t := c.t
	if !c.valid() {
		time.Sleep(90 * time.Millisecond)
	}
	close(c.hold)
	if c.revokesScope() {
		time.Sleep(60 * time.Millisecond)
		x := c.q
		x.Op, x.JobID = "device.query-status", c.v.Job.ID
		if _, code := c.s.Handle(context.Background(), x); code == equipment.OK {
			t.Fatal("revoked scope can read job")
		}
		return
	}
	job := awaitJob(t, c.s, c.q, c.v.Job.ID)
	if c.valid() && (job.Code != equipment.OK || job.Result == nil) {
		t.Fatal("valid query failed", job.Code)
	}
	if !c.valid() && (job.Result != nil || job.Code == equipment.OK) {
		t.Fatal("stale query published", c.change)
	}
}

// assertPersisted inspects the persisted result only in X, after all workers stop.
func (c *revocationCase) assertPersisted() {
	t := c.t
	c.s.Close()
	if c.change == "company" {
		c.j.selectCompany(companyX, access.OK)
	}
	st, code := c.j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK {
		t.Fatal(code)
	}
	if !c.valid() && st.Devices[0].Result != nil {
		t.Fatal("stale observation persisted", c.change)
	}
	if len(st.Credentials) != 1 {
		t.Fatal("authorization change destroyed vault")
	}
	if c.change == "success" {
		c.assertRestartRunsNewRead()
	}
}

// Since f390a6a read jobs keep no durable receipt: after a restart the same
// operation is a new, separately authenticated query.
func (c *revocationCase) assertRestartRunsNewRead() {
	t := c.t
	if c.server.Auth.Load() != 1 {
		t.Fatal("first query authenticated more than once", c.server.Auth.Load())
	}
	restarted := newEquipment(t, c.j)
	again := equipmentCall(t, restarted, c.q, equipment.OK)
	if rerun := awaitJob(t, restarted, c.q, again.Job.ID); rerun.Code != equipment.OK || rerun.ID == c.v.Job.ID {
		t.Fatal("restarted read did not run as a new job", rerun.Code)
	}
	if c.server.Auth.Load() != 2 {
		t.Fatal("restarted read authenticated wrong number of times", c.server.Auth.Load())
	}
}

func TestEquipmentJobRetentionAndRecordLimit(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	var clock atomic.Value
	clock.Store(time.Now())
	key := vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil })
	s := equipment.New(j.svc, j.st, key, routerssh.Adapter{}, equipmentIDs{}, func() time.Time { return clock.Load().(time.Time) })
	defer s.Close()
	peer, e := sshfixture.Start(sshfixture.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	_, d := setupEquipment(t, j, s, peer.Port())
	q := equipmentScope(j)
	q.Op, q.ID, q.Revision = "trust.inspect", d.ID, 1
	firstOperation, firstJob := "", ""
	for i := 0; i < equipment.MaxJobs; i++ {
		q.Operation = uuid.NewString()
		v := admitWhenReleased(t, s, q, equipment.OK)
		if i == 0 {
			firstOperation, firstJob = q.Operation, v.Job.ID
		}
		if job := awaitJob(t, s, q, v.Job.ID); job.Code != equipment.OK {
			t.Fatal(job.Code)
		}
	}
	q.Operation = uuid.NewString()
	admitWhenReleased(t, s, q, equipment.Limit)
	clock.Store(clock.Load().(time.Time).Add(equipment.JobRetention + time.Second))
	expired := q
	expired.Op, expired.JobID = "device.query-status", firstJob
	equipmentCall(t, s, expired, equipment.Changed)
	equipmentCall(t, s, q, equipment.OK)
	// Since f390a6a a read keeps no durable receipt: once retention drops the
	// first job, its operation is admitted again as a new inspection.
	q.Operation = firstOperation
	if v := admitWhenReleased(t, s, q, equipment.OK); v.Job == nil || v.Job.ID == firstJob {
		t.Fatal("expired read operation was not admitted as a new job")
	}
	if peer.Auth.Load() != 0 {
		t.Fatal("inspection/retention authenticated")
	}
}

// admitWhenReleased retries the same deliberate operation while BUSY: terminal
// metadata precedes worker exit, and BUSY does not reserve or repeat network.
func admitWhenReleased(t *testing.T, s *equipment.Service, q equipment.Command, want equipment.Code) equipment.View {
	t.Helper()
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
