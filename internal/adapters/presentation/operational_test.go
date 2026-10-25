package presentation

import (
	"bytes"
	"context"
	"github.com/google/uuid"
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
)

func TestOperationalBindingInventoryTrustAndRead(t *testing.T) {
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
	defer st.Close()
	uid := uint32(os.Getuid())
	auth := access.New(st, security.Crypto{}, time.Now, uid, "modal", st)
	key := filepath.Join(root, "key")
	if e = os.WriteFile(key, bytes.Repeat([]byte{29}, 32), 0600); e != nil {
		t.Fatal(e)
	}
	provider := vault.New(func() ([]byte, error) { return vault.ReadKey(key) })
	svc := equipment.New(auth, st, provider, routerssh.Adapter{}, modalIDs{}, time.Now)
	defer svc.Close()
	server, e := ipc.ListenAccess(filepath.Join(root, "runtime"), uid, auth, foundation.Status{Instance: "modal"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	ipc.AttachEquipment(server, svc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	defer func() {
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	app := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
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
	profile, activation := selected.Profile, selected.Activation
	peer, e := sshfixture.Start(sshfixture.Config{Mode: "interfaces"})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := DeviceInput{ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Saved router", Address: "127.0.0.1", Port: peer.Port(), Transport: "ssh", User: "reader", Secret: sshfixture.Password}
	saved := app.SaveEquipment(profile, activation, d)
	if saved.Code != "OK" || saved.Data.Device == nil {
		t.Fatal("enroll", saved.Code)
	}
	again := app.SaveEquipment(profile, activation, d)
	if again.Code != "OK" || again.Data.Device.ID != saved.Data.Device.ID {
		t.Fatal("enroll replay")
	}
	if peer.Auth.Load() != 0 || peer.Commands.Load() != 0 {
		t.Fatal("save unexpectedly connected")
	}
	wait := func(job *equipment.Job) *equipment.Job {
		deadline := time.Now().Add(3 * time.Second)
		for job.State == "RUNNING" && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
			r := app.QueryStatus(profile, activation, job.ID)
			if r.Code != "OK" {
				t.Fatal(r.Code)
			}
			job = r.Data.Job
		}
		if job.State != "SUCCEEDED" {
			t.Fatalf("job %#v", job)
		}
		return job
	}
	inspect := app.InspectTrust(profile, activation, d.ID, uuid.NewString(), 1)
	if inspect.Code != "OK" {
		t.Fatal(inspect.Code)
	}
	candidate := wait(inspect.Data.Job)
	if peer.Auth.Load() != 0 {
		t.Fatal("inspection authenticated")
	}
	acceptID := uuid.NewString()
	accepted := app.ConfirmTrust(profile, activation, d.ID, acceptID, candidate.ID, candidate.Fingerprint, 1)
	if accepted.Code != "OK" || accepted.Data.Device.TrustAcceptance == nil {
		t.Fatal("acceptance", accepted.Code)
	}
	if app.ConfirmTrust(profile, activation, d.ID, acceptID, candidate.ID, candidate.Fingerprint, 1).Code != "OK" {
		t.Fatal("acceptance replay")
	}
	if peer.Auth.Load() != 0 {
		t.Fatal("acceptance sent password")
	}
	operation := uuid.NewString()
	started := app.StartQuery(profile, activation, d.ID, operation, 2)
	if started.Code != "OK" {
		t.Fatal(started.Code)
	}
	job := wait(started.Data.Job)
	if job.Result == nil || len(job.Result.Interfaces) != 128 || job.Result.InterfacePart.Status != "limited" {
		t.Fatal("volume result")
	}
	if app.StartQuery(profile, activation, d.ID, operation, 2).Code != "OK" || peer.Auth.Load() != 1 {
		t.Fatal("start replay authenticated again")
	}
	listed := app.ListEquipment(profile, activation, "", 0, false)
	full := app.GetEquipment(profile, activation, d.ID)
	if listed.Code != "OK" || len(listed.Data.Devices) != 1 || listed.Data.Devices[0].ID != d.ID || len(listed.Data.Devices[0].Result.Interfaces) != 0 || full.Code != "OK" || len(full.Data.Device.Result.Interfaces) != 128 || full.Data.Device.Result.At != listed.Data.Devices[0].Result.At || full.Data.Device.LastAttempt.State != "SUCCEEDED" {
		t.Fatal("list/get projection or persisted attempt")
	}
	// A changed pin must fail before password even through the binding/real IPC client.
	auths := peer.Auth.Load()
	if code := st.UpdateEquipment(ctx, profile, func(state *equipment.State) equipment.Code {
		state.Devices[0].Fingerprint = "SHA256:changed"
		return equipment.OK
	}); code != equipment.OK {
		t.Fatal(code)
	}
	changed := app.StartQuery(profile, activation, d.ID, uuid.NewString(), 2)
	if changed.Code != "OK" {
		t.Fatal(changed.Code)
	}
	deadline := time.Now().Add(3 * time.Second)
	last := changed.Data.Job
	for last.State == "RUNNING" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		last = app.QueryStatus(profile, activation, last.ID).Data.Job
	}
	if last.Code != equipment.TrustChanged || peer.Auth.Load() != auths {
		t.Fatal("changed key sent password", last.Code)
	}
}
