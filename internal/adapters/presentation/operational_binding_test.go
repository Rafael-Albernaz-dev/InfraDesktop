package presentation

import (
	"bytes"
	"context"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

// operationalRig is the real binding -> IPC -> service -> SQLite -> SSH fixture path.
type operationalRig struct {
	t                   *testing.T
	ctx                 context.Context
	st                  *storage.Store
	app                 *App
	peer                *sshfixture.Server
	profile, activation string
	device              DeviceInput
}

func TestOperationalBindingInventoryTrustAndRead(t *testing.T) {
	r := newOperationalRig(t)
	r.enrollDevice()
	r.acceptTrustWithoutPassword()
	job := r.queryAndRerun()
	r.assertListGetProjection(job)
	r.assertChangedPinFailsBeforePassword()
}

func newOperationalRig(t *testing.T) *operationalRig {
	t.Helper()
	root := t.TempDir()
	t.Logf("synthetic root: %s; cleanup: t.TempDir after server cancellation", root)
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"runtime", "state", "client"} {
		if e := os.Mkdir(filepath.Join(root, n), 0700); e != nil {
			t.Fatal(e)
		}
	}
	st, e := storage.Open(filepath.Join(root, "state"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.Close() })
	uid := uint32(os.Getuid())
	auth := access.New(st, security.Crypto{}, time.Now, uid, "modal", st)
	key := filepath.Join(root, "key")
	if e = os.WriteFile(key, bytes.Repeat([]byte{29}, 32), 0600); e != nil {
		t.Fatal(e)
	}
	provider := vault.New(func() ([]byte, error) { return vault.ReadKey(key) })
	svc := equipment.New(auth, st, provider, routerssh.Adapter{}, modalIDs{}, time.Now)
	t.Cleanup(svc.Close)
	server, e := ipc.ListenAccess(filepath.Join(root, "runtime"), uid, auth, foundation.Status{Instance: "modal"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	ipc.AttachEquipment(server, svc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	})
	app := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	return &operationalRig{t: t, ctx: ctx, st: st, app: app}
}

func (r *operationalRig) enrollDevice() {
	t, app := r.t, r.app
	const pw = "isolated modal account password"
	if app.Register("technical", pw).Code != "OK" || app.AckRecovery().Code != "OK" || app.Login("technical", pw, false).Code != "OK" {
		t.Fatal("account setup")
	}
	c := app.CreateCompany("Modal synthetic")
	if c.Code != "OK" {
		t.Fatal(c.Code)
	}
	selected := app.SelectCompany(c.Companies[0].ID)
	if selected.Code != "OK" {
		t.Fatal(selected.Code)
	}
	r.profile, r.activation = selected.Profile, selected.Activation
	peer, e := sshfixture.Start(sshfixture.Config{Mode: "interfaces"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { peer.Close() })
	r.peer = peer
	r.device = DeviceInput{
		ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Saved router", Address: "127.0.0.1",
		Port: peer.Port(), Transport: string(equipment.TransportSSH), User: "reader", Secret: sshfixture.Password,
	}
	saved := app.SaveEquipment(r.profile, r.activation, r.device)
	if saved.Code != "OK" || saved.Data.Device == nil {
		t.Fatal("enroll", saved.Code)
	}
	again := app.SaveEquipment(r.profile, r.activation, r.device)
	if again.Code != "OK" || again.Data.Device.ID != saved.Data.Device.ID {
		t.Fatal("enroll replay")
	}
	if peer.Auth.Load() != 0 || peer.Commands.Load() != 0 {
		t.Fatal("save unexpectedly connected")
	}
}

// wait polls the binding until job leaves RUNNING and requires SUCCEEDED.
func (r *operationalRig) wait(job *equipment.Job) *equipment.Job {
	deadline := time.Now().Add(3 * time.Second)
	for job.State == "RUNNING" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		s := r.app.QueryStatus(r.profile, r.activation, job.ID)
		if s.Code != "OK" {
			r.t.Fatal(s.Code)
		}
		job = s.Data.Job
	}
	if job.State != "SUCCEEDED" {
		r.t.Fatalf("job %#v", job)
	}
	return job
}

func (r *operationalRig) acceptTrustWithoutPassword() {
	t, app, d := r.t, r.app, r.device
	inspect := app.InspectTrust(r.profile, r.activation, d.ID, uuid.NewString(), 1)
	if inspect.Code != "OK" {
		t.Fatal(inspect.Code)
	}
	candidate := r.wait(inspect.Data.Job)
	if r.peer.Auth.Load() != 0 {
		t.Fatal("inspection authenticated")
	}
	acceptID := uuid.NewString()
	accepted := app.ConfirmTrust(r.profile, r.activation, d.ID, acceptID, candidate.ID, candidate.Fingerprint, 1)
	if accepted.Code != "OK" || accepted.Data.Device.TrustAcceptance == nil {
		t.Fatal("acceptance", accepted.Code)
	}
	if app.ConfirmTrust(r.profile, r.activation, d.ID, acceptID, candidate.ID, candidate.Fingerprint, 1).Code != "OK" {
		t.Fatal("acceptance replay")
	}
	if r.peer.Auth.Load() != 0 {
		t.Fatal("acceptance sent password")
	}
}

// queryAndRerun reads the device, then repeats the same operation. Replay is
// scoped to active jobs since f390a6a: after completion the same operation is a
// new, separately authenticated query (see TestL06CReadReplayOnlyWhileActive).
func (r *operationalRig) queryAndRerun() *equipment.Job {
	t, app, d := r.t, r.app, r.device
	operation := uuid.NewString()
	started := app.StartQuery(r.profile, r.activation, d.ID, operation, 2)
	if started.Code != "OK" {
		t.Fatal(started.Code)
	}
	job := r.wait(started.Data.Job)
	if job.Result == nil || len(job.Result.Interfaces) != 128 || job.Result.InterfacePart.Status != "limited" {
		t.Fatal("volume result")
	}
	if r.peer.Auth.Load() != 1 {
		t.Fatal("first query authenticated more than once", r.peer.Auth.Load())
	}
	restarted := app.StartQuery(r.profile, r.activation, d.ID, operation, 2)
	if restarted.Code != "OK" {
		t.Fatal(restarted.Code)
	}
	if rerun := r.wait(restarted.Data.Job); rerun.ID == job.ID || r.peer.Auth.Load() != 2 {
		t.Fatal("completed query replayed without a new authenticated run", rerun.ID == job.ID, r.peer.Auth.Load())
	}
	return job
}

func (r *operationalRig) assertListGetProjection(job *equipment.Job) {
	t := r.t
	listed := r.app.ListEquipment(r.profile, r.activation, "", 0, false)
	full := r.app.GetEquipment(r.profile, r.activation, r.device.ID)
	if listed.Code != "OK" || len(listed.Data.Devices) != 1 || listed.Data.Devices[0].ID != r.device.ID {
		t.Fatal("list projection", listed.Code, len(listed.Data.Devices))
	}
	if len(listed.Data.Devices[0].Result.Interfaces) != 0 {
		t.Fatal("list carried full interface volume", len(listed.Data.Devices[0].Result.Interfaces))
	}
	if full.Code != "OK" || len(full.Data.Device.Result.Interfaces) != 128 {
		t.Fatal("get projection", full.Code, len(full.Data.Device.Result.Interfaces))
	}
	if full.Data.Device.Result.At != listed.Data.Devices[0].Result.At {
		t.Fatal("list/get disagree on result time", full.Data.Device.Result.At, listed.Data.Devices[0].Result.At)
	}
	if full.Data.Device.LastAttempt.State != "SUCCEEDED" {
		t.Fatal("persisted attempt", full.Data.Device.LastAttempt.State, job.ID)
	}
}

// A changed pin must fail before password even through the binding/real IPC client.
func (r *operationalRig) assertChangedPinFailsBeforePassword() {
	t := r.t
	auths := r.peer.Auth.Load()
	if code := r.st.UpdateEquipment(r.ctx, r.profile, func(state *equipment.State) equipment.Code {
		state.Devices[0].Fingerprint = "SHA256:changed"
		return equipment.OK
	}); code != equipment.OK {
		t.Fatal(code)
	}
	changed := r.app.StartQuery(r.profile, r.activation, r.device.ID, uuid.NewString(), 2)
	if changed.Code != "OK" {
		t.Fatal(changed.Code)
	}
	deadline := time.Now().Add(3 * time.Second)
	last := changed.Data.Job
	for last.State == "RUNNING" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		last = r.app.QueryStatus(r.profile, r.activation, last.ID).Data.Job
	}
	if last.Code != equipment.TrustChanged || r.peer.Auth.Load() != auths {
		t.Fatal("changed key sent password", last.Code)
	}
}
