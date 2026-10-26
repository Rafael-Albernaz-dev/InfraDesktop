package presentation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/unix"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPPPoEBindingExportAndContext(t *testing.T) {
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
	candidate := wait(inspect.Data.Job)
	accepted := app.ConfirmTrust(profile, activation, d.ID, uuid.NewString(), candidate.ID, candidate.Fingerprint, 1)
	if accepted.Code != "OK" {
		t.Fatal(accepted.Code)
	}
	collect := func() *equipment.PPObservation {
		r := app.StartPPPoECollection(profile, activation, d.ID, uuid.NewString(), 2)
		if r.Code != "OK" {
			t.Fatal(r.Code)
		}
		end := time.Now().Add(3 * time.Second)
		for time.Now().Before(end) {
			x := app.PPPoECollectionStatus(profile, activation, r.Data.Job.ID)
			if x.Code != "OK" {
				t.Fatal(x.Code)
			}
			if x.Data.Job.State != "RUNNING" {
				if x.Data.Job.Code != equipment.OK {
					t.Fatal(x.Data.Job)
				}
				return app.GetPPPoECurrent(profile, activation, d.ID).Data.Current
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("collection deadline")
		return nil
	}
	a := collect()
	save := app.SavePPPoEReference(profile, activation, d.ID, a.ID, uuid.NewString(), uuid.NewString())
	if save.Code != "OK" {
		t.Fatal(save.Code)
	}
	b := collect()
	collectDiagnostic := func(scope string) *equipment.PDDiagnostic {
		var response PPPoEResult
		switch scope {
		case equipment.PDCatalog:
			response = app.StartPPPoECatalog(profile, activation, d.ID, uuid.NewString(), 2)
		case equipment.PDRadius:
			response = app.StartPPPoERadius(profile, activation, d.ID, uuid.NewString(), 2)
		case equipment.PDPing:
			page := app.ListPPPoESessions(profile, activation, d.ID, b.ID, equipment.PPQuery{}, "")
			if page.Code != "OK" || len(page.Data.Rows) == 0 {
				t.Fatal("no ping source")
			}
			response = app.StartPPPoEPing(profile, activation, d.ID, b.ID, page.Data.Rows[0].ID, uuid.NewString(), 2)
		}
		if response.Code != "OK" || response.Data == nil || response.Data.Job == nil {
			t.Fatal("diagnostic start", response)
		}
		until := time.Now().Add(3 * time.Second)
		for time.Now().Before(until) {
			status := app.PPPoEDiagnosticStatus(profile, activation, response.Data.Job.ID)
			if status.Code != "OK" {
				t.Fatal(status.Code)
			}
			if status.Data.Job.State != "RUNNING" {
				if status.Data.Job.Code != equipment.OK {
					t.Fatal("diagnostic failed", status.Data.Job)
				}
				return app.GetPPPoEDiagnostic(profile, activation, d.ID, scope).Data.Diagnostic
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("diagnostic did not finish")
		return nil
	}
	diagnosticIDs := []string{}
	for _, scope := range []string{equipment.PDCatalog, equipment.PDRadius, equipment.PDPing} {
		result := collectDiagnostic(scope)
		if result == nil {
			t.Fatal("diagnostic missing", scope)
		}
		diagnosticIDs = append(diagnosticIDs, result.ID)
	}
	auths, commands := peer.Auth.Load(), peer.Commands.Load()
	destination := filepath.Join(root, "export.json")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return destination, nil }
	if c := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID); c != "OK" {
		t.Fatal(c)
	}
	raw, e := os.ReadFile(destination)
	if e != nil {
		t.Fatal(e)
	}
	var file map[string]any
	if json.Unmarshal(raw, &file) != nil {
		t.Fatal("invalid JSON")
	}
	if len(file["observations"].([]any)) != 2 || len(file["deltas"].([]any)) == 0 || file["limits"] == nil {
		t.Fatal("export lacks useful comparison context")
	}
	if len(file["diagnostics"].([]any)) != 3 || len(file["diagnostic_rows"].([]any)) != 10 {
		t.Fatal("missing sanitized diagnostic scopes", file["diagnostics"], file["diagnostic_rows"])
	}
	diagnosticGroups := map[string]bool{}
	for _, rawMeta := range file["diagnostics"].([]any) {
		meta := rawMeta.(map[string]any)
		if meta["status"] != "available" {
			t.Fatal("matching scope excluded", meta)
		}
		diagnosticGroups[meta["diagnostic"].(string)] = true
	}
	for _, rawRow := range file["diagnostic_rows"].([]any) {
		if !diagnosticGroups[rawRow.(map[string]any)["diagnostic"].(string)] {
			t.Fatal("diagnostic row lost frozen group")
		}
	}
	for _, id := range diagnosticIDs {
		if bytes.Contains(raw, []byte(id)) {
			t.Fatal("raw diagnostic ID exported")
		}
	}
	groups := map[string]string{}
	for i, value := range file["observations"].([]any) {
		meta := value.(map[string]any)
		want := []string{"base", "target"}[i]
		if meta["role"] != want {
			t.Fatal("metadata lacks explicit role", meta)
		}
		alias, ok := meta["observation"].(string)
		if !ok || alias == "" {
			t.Fatal("metadata lacks observation alias")
		}
		groups[alias] = want
	}
	for _, value := range file["rows"].([]any) {
		if groups[value.(map[string]any)["observation"].(string)] == "" {
			t.Fatal("row group cannot be identified")
		}
	}
	rowAliases := map[string]bool{}
	for _, value := range file["rows"].([]any) {
		rowAliases[value.(map[string]any)["row"].(string)] = true
	}
	for _, value := range file["deltas"].([]any) {
		d := value.(map[string]any)
		for _, k := range []string{"base_row", "target_row"} {
			if id, ok := d[k].(string); ok && !rowAliases[id] {
				t.Fatal("delta cannot be related to exported row")
			}
		}
	}
	for _, value := range file["diagnostics"].([]any) {
		meta := value.(map[string]any)
		if meta["scope"] == equipment.PDPing {
			if groups[meta["source_observation"].(string)] != "target" || !rowAliases[meta["source_row"].(string)] {
				t.Fatal("ping source detached from exported observation/row", meta)
			}
		}
	}
	for _, canary := range []string{"synthetic-", "198.51.", "02:00:", peer.Fingerprint(), sshfixture.Password, d.ID, a.ID, b.ID, "export_key", "provenance"} {
		if bytes.Contains(raw, []byte(canary)) {
			t.Fatal("export leaked", canary)
		}
	}
	info, _ := os.Stat(destination)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	targetOnly := filepath.Join(root, "target-only.json")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return targetOnly, nil }
	if code := app.ExportPPPoE(profile, activation, d.ID, b.ID, ""); code != "OK" {
		t.Fatal(code)
	}
	singleRaw, _ := os.ReadFile(targetOnly)
	var single map[string]any
	if json.Unmarshal(singleRaw, &single) != nil {
		t.Fatal("target-only JSON")
	}
	metas := single["observations"].([]any)
	if len(metas) != 1 || metas[0].(map[string]any)["role"] != "target" {
		t.Fatal("target-only role")
	}
	for _, value := range single["rows"].([]any) {
		if value.(map[string]any)["observation"] != metas[0].(map[string]any)["observation"] {
			t.Fatal("target-only group")
		}
	}
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return destination, nil }
	if c := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID); c != "EXPORT_EXISTS" {
		t.Fatal("overwrite", c)
	}
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return "", nil }
	if c := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID); c != "EXPORT_CANCELLED" {
		t.Fatal(c)
	}
	if peer.Auth.Load() != auths || peer.Commands.Load() != commands {
		t.Fatal("local export connected before replacement tests")
	}
	// A deliberate refresh during the picker cannot mix a new current into pages.
	replaced := filepath.Join(root, "diagnostic-replaced-picker.json")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) {
		collectDiagnostic(equipment.PDCatalog)
		return replaced, nil
	}
	if code := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID); code != "OBSERVATION_UNAVAILABLE" {
		t.Fatal("picker replacement accepted", code)
	}
	if _, err := os.Stat(replaced); !os.IsNotExist(err) {
		t.Fatal("picker replacement published file", err)
	}
	// Repeat at the final file-sync boundary, before publication revalidation.
	replacedFinal := filepath.Join(root, "diagnostic-replaced-final.json")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return replacedFinal, nil }
	mutated := false
	app.ppExportFS = ppFileOps{sync: func(fd int) error {
		if !mutated {
			mutated = true
			collectDiagnostic(equipment.PDCatalog)
		}
		return unix.Fsync(fd)
	}}
	if code := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID); code != "OBSERVATION_UNAVAILABLE" {
		t.Fatal("final replacement accepted", code)
	}
	if _, err := os.Stat(replacedFinal); !os.IsNotExist(err) {
		t.Fatal("final replacement published file", err)
	}
	app.ppExportFS = ppFileOps{}
	auths, commands = peer.Auth.Load(), peer.Commands.Load()
	late := filepath.Join(root, "late.json")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { app.Logout(); return late, nil }
	if c := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID); c != "PROFILE_CHANGED" {
		t.Fatal(c)
	}
	if _, e = os.Stat(late); !os.IsNotExist(e) {
		t.Fatal("context created file")
	}
	if peer.Auth.Load() != auths || peer.Commands.Load() != commands {
		t.Fatal("local export connected")
	}
}
func TestPPPoEExportFilesystemFailures(t *testing.T) {
	producer := func(w io.Writer) string {
		_, e := io.WriteString(w, `{"profile":"pppoe-sanitized-v1","rows":[]}`)
		if e != nil {
			return "EXPORT_UNAVAILABLE"
		}
		return "OK"
	}
	publish := func(fn func() string) string { return fn() }
	for _, mode := range []string{"ok", "existing", "symlink", "ancestor-symlink", "file-sync", "dir-sync", "dir-sync-unlink-fails", "close", "rename", "budget", "context"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "out.json")
			ops := ppFileOps{}
			prod := producer
			pub := publish
			want := "EXPORT_UNAVAILABLE"
			switch mode {
			case "ok":
				want = "OK"
			case "existing":
				os.WriteFile(path, []byte("preserve"), 0600)
				want = "EXPORT_EXISTS"
			case "symlink":
				os.Symlink("missing", path)
				want = "EXPORT_EXISTS"
			case "ancestor-symlink":
				os.Symlink(root, filepath.Join(root, "link"))
				path = filepath.Join(root, "link", "out.json")
			case "file-sync":
				ops.sync = func(int) error { return errors.New("synthetic") }
			case "dir-sync", "dir-sync-unlink-fails":
				calls := 0
				ops.sync = func(fd int) error {
					calls++
					if calls == 1 {
						return unix.Fsync(fd)
					}
					return errors.New("synthetic")
				}
				if mode == "dir-sync-unlink-fails" {
					want = "EXPORT_DURABILITY_UNCERTAIN"
					ops.unlink = func(int, string, int) error { return errors.New("synthetic") }
				}
			case "close":
				ops.close = func(f *os.File) error { f.Close(); return errors.New("synthetic") }
			case "rename":
				ops.rename = func(int, string, int, string, uint) error { return errors.New("synthetic") }
			case "budget":
				prod = func(w io.Writer) string {
					_, e := io.WriteString(w, strings.Repeat("x", ppExportBytes+1))
					if e != nil {
						return "EXPORT_UNAVAILABLE"
					}
					return "OK"
				}
			case "context":
				want = "PROFILE_CHANGED"
				pub = func(func() string) string { return "PROFILE_CHANGED" }
			}
			if c := writePPExport(path, prod, pub, ops); c != want {
				t.Fatalf("got %s want %s", c, want)
			}
			raw, e := os.ReadFile(path)
			switch mode {
			case "ok", "dir-sync-unlink-fails":
				if e != nil || !json.Valid(raw) {
					t.Fatal("complete output missing", e)
				}
			case "existing":
				if string(raw) != "preserve" {
					t.Fatal("overwritten")
				}
			case "symlink":
				if _, e := os.Lstat(path); e != nil {
					t.Fatal("symlink removed")
				}
			default:
				if e == nil {
					t.Fatal("unexpected final output")
				}
			}
		})
	}
}

func TestPPPoEReviewExportInitialAndFinalInputs(t *testing.T) {
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
	candidate := wait(inspect.Data.Job)
	accepted := app.ConfirmTrust(profile, activation, d.ID, uuid.NewString(), candidate.ID, candidate.Fingerprint, 1)
	if accepted.Code != "OK" {
		t.Fatal(accepted.Code)
	}
	collect := func() *equipment.PPObservation {
		r := app.StartPPPoECollection(profile, activation, d.ID, uuid.NewString(), 2)
		if r.Code != "OK" {
			t.Fatal(r.Code)
		}
		end := time.Now().Add(3 * time.Second)
		for time.Now().Before(end) {
			x := app.PPPoECollectionStatus(profile, activation, r.Data.Job.ID)
			if x.Code != "OK" {
				t.Fatal(x.Code)
			}
			if x.Data.Job.State != "RUNNING" {
				if x.Data.Job.Code != equipment.OK {
					t.Fatal(x.Data.Job)
				}
				return app.GetPPPoECurrent(profile, activation, d.ID).Data.Current
			}
			time.Sleep(time.Millisecond)
		}
		t.Fatal("collection deadline")
		return nil
	}
	a := collect()
	refID := uuid.NewString()
	if r := app.SavePPPoEReference(profile, activation, d.ID, a.ID, refID, uuid.NewString()); r.Code != "OK" {
		t.Fatal(r.Code)
	}
	b := collect()
	t.Run("invalid-before-picker", func(t *testing.T) {
		called := false
		app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { called = true; return "", nil }
		code := app.ExportPPPoE(uuid.NewString(), activation, d.ID, b.ID, a.ID)
		if called || code != "PROFILE_CHANGED" {
			t.Fatal("invalid context opened picker", called, code)
		}
	})
	t.Run("base-deleted-at-file-sync", func(t *testing.T) {
		dest := filepath.Join(root, "deleted-base.json")
		app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return dest, nil }
		calls := 0
		app.ppExportFS = ppFileOps{sync: func(fd int) error {
			calls++
			if calls == 1 {
				r := app.DeletePPPoEReference(profile, activation, d.ID, refID, uuid.NewString())
				if r.Code != "OK" {
					t.Fatal(r.Code)
				}
			}
			return unix.Fsync(fd)
		}}
		code := app.ExportPPPoE(profile, activation, d.ID, b.ID, a.ID)
		if code != "OBSERVATION_UNAVAILABLE" {
			t.Fatal("deleted base published", code)
		}
		if _, e := os.Stat(dest); !os.IsNotExist(e) {
			t.Fatal("final output exists", e)
		}
	})
}
