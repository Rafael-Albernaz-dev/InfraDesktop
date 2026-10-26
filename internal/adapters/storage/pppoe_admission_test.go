package storage

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
	"time"
)

type ppHeldReturn struct {
	routerssh.Adapter
	returned chan struct{}
	release  chan struct{}
}

func (r ppHeldReturn) CollectPPPoE(ctx context.Context, d equipment.Device, u, p string, check func() equipment.Code) (*equipment.PPObservation, equipment.Code) {
	o, c := r.Adapter.CollectPPPoE(ctx, d, u, p, check)
	close(r.returned)
	<-r.release
	return o, c
}
func TestPPPoEAdmissionAliasesAndCancel(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	hold := make(chan struct{})
	peer, e := sshfixture.Start(sshfixture.Config{Hold: hold})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	remote := ppHeldReturn{returned: make(chan struct{}), release: make(chan struct{})}
	released := false
	defer func() {
		if !released {
			close(remote.release)
		}
	}()
	s := equipment.New(j.svc, j.st, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{37}, 32), nil }), remote, equipmentIDs{}, time.Now)
	// Cleanup is registered before the release defer, so a failure still releases the worker.
	defer func() {
		if !released {
			close(remote.release)
			released = true
		}
		s.Close()
	}()
	cred, d := setupEquipment(t, j, s, peer.Port())
	_ = cred
	var alias equipment.Device
	if c := j.st.UpdateEquipment(context.Background(), companyX, func(st *equipment.State) equipment.Code {
		st.Devices[0].Fingerprint = peer.Fingerprint()
		st.Devices[0].Revision = 2
		alias = st.Devices[0]
		alias.ID = uuid.NewString()
		alias.Address = "::ffff:127.0.0.1"
		st.Devices = append(st.Devices, alias)
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	q := equipmentScope(j)
	q.Op = "pppoe.collect-start"
	in := equipment.PPInput{DeviceID: d.ID, Revision: 2, Operation: uuid.NewString()}
	v, c := s.HandlePP(context.Background(), q, in)
	if c != equipment.OK {
		t.Fatal(c)
	}
	deadline := time.Now().Add(3 * time.Second)
	for peer.Commands.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if peer.Commands.Load() != 1 {
		t.Fatal("command barrier")
	}
	try := in
	try.DeviceID = alias.ID
	try.Operation = uuid.NewString()
	if _, c = s.HandlePP(context.Background(), q, try); c != access.Busy {
		t.Fatal("alias overlapped", c)
	}
	summary := equipmentScope(j)
	summary.Op = "device.query-start"
	summary.ID = alias.ID
	summary.Revision = 2
	summary.Operation = uuid.NewString()
	if _, c = s.Handle(context.Background(), summary); c != access.Busy {
		t.Fatal("summary overlapped", c)
	}
	cancel := q
	cancel.Op = "pppoe.collect-cancel"
	if _, c = s.HandlePP(context.Background(), cancel, equipment.PPInput{JobID: v.Job.ID}); c != equipment.OK {
		t.Fatal(c)
	}
	select {
	case <-remote.returned:
	case <-time.After(3 * time.Second):
		t.Fatal("SSH did not cancel")
	}
	if _, c = s.HandlePP(context.Background(), q, try); c != access.Busy {
		t.Fatal("cancel released slot before worker exit", c)
	}
	close(remote.release)
	released = true
	close(hold)
	s.Close()
	cur, c := j.st.PPCurrent(context.Background(), companyX, d.ID)
	if c != equipment.OK || cur.Current != nil {
		t.Fatal("late canceled publication", c)
	}
	if peer.Auth.Load() != 1 {
		t.Fatal("alias authenticated", peer.Auth.Load())
	}
}
