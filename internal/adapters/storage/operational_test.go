package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestOperationalEditsPreserveOnlyMatchingReading(t *testing.T) {
	for _, change := range []string{"name-active", "name-inactive", "name-pending", "address", "port", "credential"} {
		t.Run(change, func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			hold := make(chan struct{}, 3)
			for range 3 {
				hold <- struct{}{}
			}
			peer, err := sshfixture.Start(sshfixture.Config{Hold: hold})
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			defer func() {
				if hold != nil {
					close(hold)
				}
			}()
			cred, d := setupEquipment(t, j, s, peer.Port())
			query := trustEquipment(t, s, equipmentScope(j), d)
			started := equipmentCall(t, s, query, equipment.OK)
			done := awaitJob(t, s, query, started.Job.ID)
			if done.Result == nil || done.Code != equipment.OK {
				t.Fatal("initial reading missing")
			}
			get := equipmentScope(j)
			get.Op, get.ID = "inventory.get", d.ID
			current := equipmentCall(t, s, get, equipment.OK).Device
			var pending string
			if change == "name-pending" {
				query.Operation = uuid.NewString()
				pending = equipmentCall(t, s, query, equipment.OK).Job.ID
				deadline := time.Now().Add(time.Second)
				for peer.Commands.Load() < 4 && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				if peer.Commands.Load() != 4 {
					t.Fatal("pending query did not reach command barrier")
				}
				current = equipmentCall(t, s, get, equipment.OK).Device
				if current.LastAttempt == nil || current.LastAttempt.State != "STARTED" {
					t.Fatal("missing pending attempt")
				}
			}
			if change == "name-inactive" {
				toggle := equipmentScope(j)
				toggle.Op, toggle.ID, toggle.Operation, toggle.Revision, toggle.Active = "inventory.set-active", d.ID, uuid.NewString(), current.Revision, false
				current = equipmentCall(t, s, toggle, equipment.OK).Device
			}
			before := *current
			edit := equipmentScope(j)
			edit.Op, edit.ID, edit.Operation = "inventory.update", d.ID, uuid.NewString()
			edit.Revision, edit.Name, edit.Address, edit.Port, edit.Transport, edit.CredentialID = before.Revision, "Renamed router", before.Address, before.Port, before.Transport, before.CredentialID
			switch change {
			case "address":
				edit.Address = "127.0.0.2"
			case "port":
				edit.Port = 1
				if edit.Port == before.Port {
					edit.Port = 2
				}
			case "credential":
				cred.ID, cred.Operation, cred.Alias = uuid.NewString(), uuid.NewString(), "Other reader"
				equipmentCall(t, s, cred, equipment.OK)
				edit.CredentialID = cred.ID
			}
			auths, commands := peer.Auth.Load(), peer.Commands.Load()
			saved := equipmentCall(t, s, edit, equipment.OK).Device
			if saved.Revision != before.Revision+1 || saved.Active != before.Active {
				t.Fatal("metadata changed activity/revision")
			}
			if change == "name-active" || change == "name-inactive" || change == "name-pending" {
				if !reflect.DeepEqual(saved.Result, before.Result) || !reflect.DeepEqual(saved.LastAttempt, before.LastAttempt) || !reflect.DeepEqual(saved.TrustAcceptance, before.TrustAcceptance) {
					t.Fatal("metadata erased completed reading/attempt/trust")
				}
			} else if saved.Result != nil || saved.LastAttempt != nil {
				t.Fatal("changed connection retained incompatible reading")
			}
			if (change == "address" || change == "port") && (saved.Fingerprint != "" || saved.TrustAcceptance != nil) {
				t.Fatal("changed endpoint retained pin")
			}
			if change == "credential" && (saved.Fingerprint != before.Fingerprint || !reflect.DeepEqual(saved.TrustAcceptance, before.TrustAcceptance)) {
				t.Fatal("credential change erased unrelated endpoint trust")
			}
			replay := equipmentCall(t, s, edit, equipment.OK).Device
			stored := equipmentCall(t, s, get, equipment.OK).Device
			if !reflect.DeepEqual(saved, replay) || !reflect.DeepEqual(saved, stored) || peer.Auth.Load() != auths || peer.Commands.Load() != commands {
				t.Fatal("edit/replay did not persist coherently or connected")
			}
			if pending != "" {
				status := equipmentScope(j)
				status.Op, status.JobID = "device.query-status", pending
				job := equipmentCall(t, s, status, equipment.OK).Job
				if job.State != "CANCELLED" || job.Code != equipment.Cancelled || job.Result != nil {
					t.Fatal("edit did not cancel old revision query")
				}
				close(hold)
				hold = nil
				s.Close()
				after := equipmentCall(t, s, get, equipment.OK).Device
				if !reflect.DeepEqual(after, saved) || after.LastAttempt.State != "STARTED" || peer.Auth.Load() != auths || peer.Commands.Load() != commands {
					t.Fatal("late query overwrote preserved reading or invented completion")
				}
			}
		})
	}
}

func TestOperationalExplicitFirstAcceptanceWithoutOrigin(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
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
	q.Operation = uuid.NewString()
	v := equipmentCall(t, s, q, equipment.OK)
	job := awaitJob(t, s, q, v.Job.ID)
	if peer.Auth.Load() != 0 {
		t.Fatal("inspection sent password")
	}
	q.Op = "trust.confirm"
	q.Operation = uuid.NewString()
	q.JobID = job.ID
	q.Fingerprint = job.Fingerprint
	before, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK {
		t.Fatal(code)
	}
	if _, e := j.st.unit.Exec("CREATE TRIGGER deny_acceptance BEFORE UPDATE OF trust_acceptance ON devices WHEN NEW.trust_acceptance IS NOT OLD.trust_acceptance BEGIN SELECT RAISE(ABORT,'isolated failure'); END"); e != nil {
		t.Fatal(e)
	}
	equipmentCall(t, s, q, equipment.Unavailable)
	rolled, code := j.st.ReadEquipment(context.Background(), companyX)
	if code != equipment.OK || rolled.Devices[0].Fingerprint != "" || rolled.Devices[0].TrustAcceptance != nil || len(rolled.Operations) != len(before.Operations) {
		t.Fatal("failed acceptance left partial pin/context/receipt")
	}
	if _, e := j.st.unit.Exec("DROP TRIGGER deny_acceptance"); e != nil {
		t.Fatal(e)
	}
	v = equipmentCall(t, s, q, equipment.OK)
	if v.Device.TrustAcceptance == nil || v.Device.TrustAcceptance.Mode != "explicit-first-acceptance" || v.Device.TrustAcceptance.UID != 1000 || v.Device.TrustAcceptance.Company != companyX || v.Device.TrustAcceptance.Revision != 1 || v.Device.TrustAcceptance.Address != "127.0.0.1" || v.Device.TrustAcceptance.At == "" {
		t.Fatal("missing daemon acceptance context")
	}
	equipmentCall(t, s, q, equipment.OK) // exact replay, no new event
	if peer.Auth.Load() != 0 {
		t.Fatal("acceptance itself authenticated")
	}
	if v.Device == nil || v.Device.Fingerprint != peer.Fingerprint() || v.Device.TrustOrigin != "" {
		t.Fatal("missing pin or invented external verification")
	}
}

func TestOperationalPartsFailureAttemptAndExplicitRecovery(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	var mode atomic.Value
	mode.Store("")
	peer, e := sshfixture.Start(sshfixture.Config{InterfaceMode: func() string { return mode.Load().(string) }})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	_, d := setupEquipment(t, j, s, peer.Port())
	q := trustEquipment(t, s, equipmentScope(j), d)
	run := func() equipment.Job {
		q.Operation = uuid.NewString()
		v := equipmentCall(t, s, q, equipment.OK)
		return awaitJob(t, s, q, v.Job.ID)
	}
	first := run()
	if first.Code != equipment.OK {
		t.Fatal(first.Code)
	}
	mode.Store("interfaces-permission")
	second := run()
	if second.Code != equipment.OK || second.Result == nil || second.Result.InterfacePart.Status != "stale" || second.Result.InterfacePart.Code != "REMOTE_PERMISSION_DENIED" || second.Result.InterfacePart.At != first.Result.InterfacePart.At || !reflect.DeepEqual(second.Result.Interfaces, first.Result.Interfaces) || second.Result.At == first.Result.At {
		t.Fatal("did not preserve separately dated interface snapshot")
	}
	get := equipmentScope(j)
	get.Op = "inventory.get"
	get.ID = d.ID
	saved := equipmentCall(t, s, get, equipment.OK).Device
	if saved.LastAttempt == nil || saved.LastAttempt.State != "SUCCEEDED" || saved.Result.Quality != "partial" {
		t.Fatal("missing partial attempt")
	}
	peer.Close()
	failed := run()
	if failed.Code != "CONNECT_FAILED" {
		t.Fatal(failed.Code)
	}
	saved = equipmentCall(t, s, get, equipment.OK).Device
	if saved.LastAttempt.State != "FAILED" || saved.LastAttempt.Code != "CONNECT_FAILED" || !reflect.DeepEqual(saved.Result, second.Result) {
		t.Fatal("failed retry lost last valid result or recent attempt")
	}
	reset := equipmentScope(j)
	reset.Op = "trust.reset"
	reset.ID = d.ID
	reset.Revision = saved.Revision
	reset.Operation = uuid.NewString()
	resetView := equipmentCall(t, s, reset, equipment.OK)
	equipmentCall(t, s, reset, equipment.OK)
	if resetView.Device.Fingerprint != "" || resetView.Device.TrustAcceptance != nil || !reflect.DeepEqual(resetView.Device.Result, saved.Result) {
		t.Fatal("explicit recovery erased reading or kept pin")
	}
	q.Revision = resetView.Device.Revision
	q.Operation = uuid.NewString()
	equipmentCall(t, s, q, equipment.TrustRequired)
	state, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK || len(state.Devices) != 1 {
		t.Fatal("recovery storage")
	}
}

func TestOperationalAcceptanceRejectsWrongCandidateAndScope(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	peer, e := sshfixture.Start(sshfixture.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	_, d := setupEquipment(t, j, s, peer.Port())
	_, other := setupEquipment(t, j, s, peer.Port())
	inspect := equipmentScope(j)
	inspect.Op = "trust.inspect"
	inspect.ID = d.ID
	inspect.Revision = 1
	inspect.Operation = uuid.NewString()
	v := equipmentCall(t, s, inspect, equipment.OK)
	job := awaitJob(t, s, inspect, v.Job.ID)
	accept := inspect
	accept.Op = "trust.confirm"
	accept.JobID = job.ID
	accept.Fingerprint = job.Fingerprint
	accept.Operation = uuid.NewString()
	for _, change := range []string{"device", "revision", "key", "job"} {
		bad := accept
		bad.Operation = uuid.NewString()
		switch change {
		case "device":
			bad.ID = other.ID
		case "revision":
			bad.Revision++
		case "key":
			bad.Fingerprint = "SHA256:other"
		case "job":
			bad.JobID = uuid.NewString()
		}
		equipmentCall(t, s, bad, equipment.TrustRequired)
	}
	// Completed inspection cancellation is a UI dismissal; it cannot accept another device.
	j.selectCompany(companyY, access.OK)
	equipmentCall(t, s, accept, equipment.Changed)
	if peer.Auth.Load() != 0 {
		t.Fatal("negative acceptance sent password")
	}
}

func TestOperationalAdditiveV3MigrationPreservesLegacy(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "rollback"}[conflict], func(t *testing.T) {
			j := newJourney(t)
			j.create(companyX, "X", access.OK)
			j.selectCompany(companyX, access.OK)
			s := newEquipment(t, j)
			_, d := setupEquipment(t, j, s, 22)
			legacy := `{"identity":"Previous router","version":"6.49","interfaces":[],"observed_at":"2026-10-23T10:00:00Z"}`
			if _, e := j.st.unit.Exec("UPDATE devices SET fingerprint=?,trust_origin=?,observation=? WHERE id=?", "SHA256:legacy", "Historical independent source", legacy, d.ID); e != nil {
				t.Fatal(e)
			}
			dropPPPoEFixtureSchema(t, j)
			if _, e := j.st.unit.Exec("ALTER TABLE devices DROP COLUMN trust_acceptance;DELETE FROM schema_migrations WHERE version=4"); e != nil {
				t.Fatal(e)
			}
			if !conflict {
				if _, e := j.st.unit.Exec("ALTER TABLE devices DROP COLUMN last_attempt"); e != nil {
					t.Fatal(e)
				}
			}
			e := j.st.validateUnit(j.st.unit, companyX)
			if conflict {
				if e == nil {
					t.Fatal("conflict accepted")
				}
				var columns int
				j.st.unit.QueryRow("SELECT count(*) FROM pragma_table_info('devices') WHERE name='trust_acceptance'").Scan(&columns)
				if columns != 0 {
					t.Fatal("failed V4 migration left first column")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			state, code := j.st.ReadEquipment(context.Background(), companyX)
			if code != equipment.OK || state.Devices[0].TrustAcceptance != nil || state.Devices[0].TrustOrigin != "Historical independent source" || state.Devices[0].Fingerprint != "SHA256:legacy" || state.Devices[0].Result.Identity != "Previous router" || len(state.Credentials) != 1 || len(state.WrappedKey) == 0 {
				t.Fatal("upgrade reclassified or lost legacy data")
			}
			var h string
			j.st.unit.QueryRow("SELECT checksum FROM schema_migrations WHERE version=3").Scan(&h)
			if h != checksum([]byte(companyEquipmentSchema)) {
				t.Fatal("changed historical checksum")
			}
		})
	}
}

func TestOperationalPersistenceFailureKeepsPreviousReading(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	peer, e := sshfixture.Start(sshfixture.Config{})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	_, d := setupEquipment(t, j, s, peer.Port())
	q := trustEquipment(t, s, equipmentScope(j), d)
	first := equipmentCall(t, s, q, equipment.OK)
	job := awaitJob(t, s, q, first.Job.ID)
	if job.Result == nil {
		t.Fatal("missing initial reading")
	}
	if _, e := j.st.unit.Exec("CREATE TRIGGER deny_observation BEFORE UPDATE OF observation ON devices WHEN NEW.observation IS NOT OLD.observation BEGIN SELECT RAISE(ABORT,'isolated failure'); END"); e != nil {
		t.Fatal(e)
	}
	q.Operation = uuid.NewString()
	second := equipmentCall(t, s, q, equipment.OK)
	failed := awaitJob(t, s, q, second.Job.ID)
	if failed.Code != "PERSISTENCE_FAILED" || failed.Result != nil {
		t.Fatal("failed storage reported remote success", failed.Code)
	}
	get := equipmentScope(j)
	get.Op = "inventory.get"
	get.ID = d.ID
	saved := equipmentCall(t, s, get, equipment.OK).Device
	if !reflect.DeepEqual(saved.Result, job.Result) || saved.LastAttempt.State != "STARTED" || peer.Auth.Load() != 2 {
		t.Fatal("failed storage lost prior data or automatically retried")
	}
}
