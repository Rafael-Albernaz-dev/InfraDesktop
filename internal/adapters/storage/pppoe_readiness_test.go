package storage

import (
	"bytes"
	"context"
	"errors"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"sync/atomic"
	"testing"
	"time"
)

func TestPPPoEProviderReadinessDuringBlockedJob(t *testing.T) {
	for _, mode := range []int32{0, 1, 2} {
		t.Run(map[int32]string{0: "stable-positive", 1: "missing", 2: "changed-key"}[mode], func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			var providerState atomic.Int32
			provider := vault.New(func() ([]byte, error) {
				switch providerState.Load() {
				case 1:
					return nil, errors.New("synthetic unavailable")
				case 2:
					return bytes.Repeat([]byte{38}, 32), nil
				default:
					return bytes.Repeat([]byte{37}, 32), nil
				}
			})
			peer, e := sshfixture.Start(sshfixture.Config{})
			if e != nil {
				t.Fatal(e)
			}
			defer peer.Close()
			remote := ppHeldReturn{returned: make(chan struct{}), release: make(chan struct{})}
			s := equipment.New(j.svc, j.st, provider, remote, equipmentIDs{}, time.Now)
			released := false
			defer func() {
				if !released {
					close(remote.release)
				}
				s.Close()
			}()
			_, d := setupEquipment(t, j, s, peer.Port())
			if c := j.st.UpdateEquipment(context.Background(), companyX, func(st *equipment.State) equipment.Code {
				st.Devices[0].Fingerprint = peer.Fingerprint()
				st.Devices[0].Revision = 2
				return equipment.OK
			}); c != equipment.OK {
				t.Fatal(c)
			}
			q := equipmentScope(j)
			q.Op = "pppoe.collect-start"
			v, c := s.HandlePP(context.Background(), q, equipment.PPInput{DeviceID: d.ID, Revision: 2, Operation: uuid.NewString()})
			if c != equipment.OK {
				t.Fatal(c)
			}
			select {
			case <-remote.returned:
			case <-time.After(3 * time.Second):
				t.Fatal("adapter return barrier")
			}
			providerState.Store(mode)
			close(remote.release)
			released = true
			status := q
			status.Op = "pppoe.collect-status"
			deadline := time.Now().Add(3 * time.Second)
			var job *equipment.Job
			for time.Now().Before(deadline) {
				v, c = s.HandlePP(context.Background(), status, equipment.PPInput{JobID: v.Job.ID})
				if c != equipment.OK {
					t.Fatal(c)
				}
				job = v.Job
				if job.State != "RUNNING" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if job == nil || job.State == "RUNNING" {
				t.Fatal("job did not terminate")
			}
			cur, c := j.st.PPCurrent(context.Background(), companyX, d.ID)
			if c != equipment.OK {
				t.Fatal(c)
			}
			if mode == 0 {
				if job.Code != equipment.OK || cur.Current == nil {
					t.Fatal("legitimate ready provider blocked", job)
				}
			} else if job.Code != equipment.VaultUnavailable || job.State != "FAILED" || cur.Current != nil || cur.Attempt == nil || cur.Attempt.Code != equipment.VaultUnavailable || cur.Attempt.State != "FAILED" {
				t.Fatal("unready provider published", mode, job)
			}
			if peer.Auth.Load() != 1 {
				t.Fatal("repeated authentication")
			}
		})
	}
}
