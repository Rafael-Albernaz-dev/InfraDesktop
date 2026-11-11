package presentation

import (
	"bytes"
	"context"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/routerssh/pppoeparse"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"

	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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
	s := &ppExportJourney{root: root}
	defer func() {
		if s.peer != nil {
			s.peer.Close()
		}
	}()
	s.setupAccountAndPeer(t, uid)
	s.prepareExport(t)
	s.assertDiagnosticGroups(t)
	s.assertObservationAliases(t)
	s.assertSanitizedAndTargetOnly(t)
	csvRaw, records, csvPath := s.readOperationalCSV(t)
	s.assertOperationalCSV(t, csvRaw, records, csvPath)
	s.assertOperationalGuards(t)
	s.assertReplacementAndLogout(t)
}

type ppExportJourney struct {
	root, profile, activation string
	app                       *App
	d                         DeviceInput
	peer                      *sshfixture.Server
	omitContext               atomic.Bool
	a, b                      *equipment.PPObservation
	diagnosticIDs             []string
	auths, commands           int32
	destination               string
	raw                       []byte
	file                      map[string]any
}

func (s *ppExportJourney) setupAccountAndPeer(t *testing.T, uid uint32) {
	var e error
	s.app = New(ipc.NewAccountClient(filepath.Join(s.root, "runtime"), filepath.Join(s.root, "client"), uid))
	const pw = "isolated modal account password"
	if s.app.Register("technical", pw).Code != "OK" || s.app.AckRecovery().Code != "OK" || s.app.Login("technical", pw, false).Code != "OK" {
		t.Fatal("account setup")
	}
	c := s.app.CreateCompany("Modal synthetic")
	if c.Code != "OK" {
		t.Fatal(c.Code)
	}
	selected := s.app.SelectCompany(c.Companies[0].ID)
	if selected.Code != "OK" {
		t.Fatal(selected.Code)
	}
	s.profile, s.activation = selected.Profile, selected.Activation

	s.peer, e = sshfixture.Start(sshfixture.Config{Mode: "interfaces", Command: func(command string) (string, bool) {
		return strings.Repeat("x", routerssh.MaxOutput+1), s.omitContext.Load() && command == pppoeparse.PPContextCommand
	}})
	if e != nil {
		t.Fatal(e)
	}
	s.d = DeviceInput{ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Saved router", Address: "127.0.0.1",
		Port: s.peer.Port(), Transport: string(equipment.TransportSSH), User: "reader", Secret: sshfixture.Password}
	saved := s.app.SaveEquipment(s.profile, s.activation, s.d)
	if saved.Code != "OK" || saved.Data.Device == nil {
		t.Fatal("enroll", saved.Code)
	}
	again := s.app.SaveEquipment(s.profile, s.activation, s.d)
	if again.Code != "OK" || again.Data.Device.ID != saved.Data.Device.ID {
		t.Fatal("enroll replay")
	}
	if s.peer.Auth.Load() != 0 || s.peer.Commands.Load() != 0 {
		t.Fatal("save unexpectedly connected")
	}
	inspect := s.app.InspectTrust(s.profile, s.activation, s.d.ID, uuid.NewString(), 1)
	candidate := s.waitTrustJob(t, inspect.Data.Job)
	accepted := s.app.ConfirmTrust(s.profile, s.activation, s.d.ID, uuid.NewString(), candidate.ID, candidate.Fingerprint, 1)
	if accepted.Code != "OK" {
		t.Fatal(accepted.Code)
	}

}

func (s *ppExportJourney) waitTrustJob(t *testing.T, job *equipment.Job) *equipment.Job {

	deadline := time.Now().Add(3 * time.Second)
	for job.State == "RUNNING" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		r := s.app.QueryStatus(s.profile, s.activation, job.ID)
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

func (s *ppExportJourney) collect(t *testing.T) *equipment.PPObservation {

	r := s.app.StartPPPoECollection(s.profile, s.activation, s.d.ID, uuid.NewString(), 2)
	if r.Code != "OK" {
		t.Fatal(r.Code)
	}
	end := time.Now().Add(3 * time.Second)
	for time.Now().Before(end) {
		x := s.app.PPPoECollectionStatus(s.profile, s.activation, r.Data.Job.ID)
		if x.Code != "OK" {
			t.Fatal(x.Code)
		}
		if x.Data.Job.State != "RUNNING" {
			if x.Data.Job.Code != equipment.OK {
				t.Fatal(x.Data.Job)
			}
			return s.app.GetPPPoECurrent(s.profile, s.activation, s.d.ID).Data.Current
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("collection deadline")
	return nil

}

func (s *ppExportJourney) collectDiagnostic(t *testing.T, scope string) *equipment.PDDiagnostic {

	var response PPPoEResult
	switch scope {
	case equipment.PDCatalog:
		response = s.app.StartPPPoECatalog(s.profile, s.activation, s.d.ID, uuid.NewString(), 2)
	case equipment.PDRadius:
		response = s.app.StartPPPoERadius(s.profile, s.activation, s.d.ID, uuid.NewString(), 2)
	case equipment.PDPing:
		page := s.app.ListPPPoESessions(s.profile, s.activation, s.d.ID, s.b.ID, equipment.PPQuery{}, "")
		if page.Code != "OK" || len(page.Data.Rows) == 0 {
			t.Fatal("no ping source")
		}
		response = s.app.StartPPPoEPing(s.profile, s.activation, s.d.ID, s.b.ID, page.Data.Rows[0].ID, uuid.NewString(), 2)
	}
	if response.Code != "OK" || response.Data == nil || response.Data.Job == nil {
		t.Fatal("diagnostic start", response)
	}
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		status := s.app.PPPoEDiagnosticStatus(s.profile, s.activation, response.Data.Job.ID)
		if status.Code != "OK" {
			t.Fatal(status.Code)
		}
		if status.Data.Job.State != "RUNNING" {
			if status.Data.Job.Code != equipment.OK {
				t.Fatal("diagnostic failed", status.Data.Job)
			}
			return s.app.GetPPPoEDiagnostic(s.profile, s.activation, s.d.ID, scope).Data.Diagnostic
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("diagnostic did not finish")
	return nil

}

func (s *ppExportJourney) prepareExport(t *testing.T) {
	var e error
	s.a = s.collect(t)
	save := s.app.SavePPPoEReference(s.profile, s.activation, s.d.ID, s.a.ID, uuid.NewString(), uuid.NewString())
	if save.Code != "OK" {
		t.Fatal(save.Code)
	}
	s.omitContext.Store(true)
	s.b = s.collect(t)
	if !s.b.ContextOmitted {
		t.Fatal("optional context budget marker lost before export")
	}
	s.diagnosticIDs = []string{}
	for _, scope := range []string{equipment.PDCatalog, equipment.PDRadius, equipment.PDPing} {
		result := s.collectDiagnostic(t, scope)
		if result == nil {
			t.Fatal("diagnostic missing", scope)
		}
		s.diagnosticIDs = append(s.diagnosticIDs, result.ID)
	}
	s.auths, s.commands = s.peer.Auth.Load(), s.peer.Commands.Load()
	s.destination = filepath.Join(s.root, "export.json")
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return s.destination, nil }
	if c := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, s.a.ID); c != "OK" {
		t.Fatal(c)
	}
	s.raw, e = os.ReadFile(s.destination)
	if e != nil {
		t.Fatal(e)
	}

	if json.Unmarshal(s.raw, &s.file) != nil {
		t.Fatal("invalid JSON")
	}

}
