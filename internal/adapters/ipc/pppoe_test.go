package ipc

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type ppJourney struct {
	root   string
	auth   *access.Service
	server *Server
	t      *testing.T
	store  *storage.Store
	svc    *equipment.Service
	client *AccountClient
	peer   *sshfixture.Server
	device string
	mode   atomic.Int32
}

func newPPJourney(t *testing.T, configs ...sshfixture.Config) *ppJourney {
	t.Helper()
	j := &ppJourney{t: t, device: uuid.NewString()}
	root := t.TempDir()
	j.root = root
	if os.Chmod(root, 0700) != nil {
		t.Fatal("chmod")
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
	j.store = st
	cfg := sshfixture.Config{PPPoE: func() string {
		if j.mode.Load() == 1 {
			return "H\t1\t50000\t7.26\t2d1h1s\nZ\t0\trows\n"
		}
		return "H\t1\t2\t7.26\t2d1h1s\nR\t*1\t1\tcanary-login\t198.51.100.1\tcanary-caller\t1h1s\nR\t*2\t2\tcanary-login\t198.51.100.2\tcanary-caller2\t1h1s\nZ\t2\tcomplete\t2d1h1s\n"
	}}
	if len(configs) > 0 {
		cfg = configs[0]
	}
	peer, e := sshfixture.Start(cfg)
	if e != nil {
		t.Fatal(e)
	}
	j.peer = peer
	uid := uint32(os.Getuid())
	a := access.New(st, security.Crypto{}, time.Now, uid, "pp-active", st)
	j.auth = a
	j.svc = equipment.New(a, st, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{8}, 32), nil }), routerssh.Adapter{}, testEquipmentIDs{}, time.Now)
	server, e := ListenAccess(filepath.Join(root, "runtime"), uid, a, foundation.Status{Instance: "pp"}, st.Close)
	if e != nil {
		t.Fatal(e)
	}
	j.server = server
	AttachEquipment(server, j.svc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	t.Cleanup(func() {
		peer.Close()
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	})
	j.client = NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid)
	call := func(q Request) {
		r := j.client.Call(context.Background(), q)
		if r.Code != foundation.OK {
			t.Fatal(q.Op, r.Code)
		}
	}
	call(Request{Op: "access.register", Name: "synthetic", Password: "synthetic PPPoE password"})
	call(Request{Op: "access.login", Name: "synthetic", Password: "synthetic PPPoE password"})
	call(Request{Op: "company.create", Name: "Synthetic PPPoE"})
	r := j.client.Call(context.Background(), Request{Op: "access.bootstrap"})
	call(Request{Op: "company.select", Name: r.Data.Companies[0].ID})
	r = j.client.Call(context.Background(), Request{Op: "inventory.enroll", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, Equipment: &EquipmentInput{ID: j.device, Operation: uuid.NewString(), Name: "Synthetic router", Address: "127.0.0.1", Port: peer.Port(), Transport: "ssh", User: "reader", Secret: sshfixture.Password}})
	if r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	c := st.UpdateEquipment(context.Background(), j.client.profile, func(s *equipment.State) equipment.Code {
		for i := range s.Devices {
			if s.Devices[i].ID == j.device {
				s.Devices[i].Fingerprint = peer.Fingerprint()
				s.Devices[i].Revision = 2
			}
		}
		return equipment.OK
	})
	if c != equipment.OK {
		t.Fatal(c)
	}
	return j
}
func (j *ppJourney) call(op string, in equipment.PPInput, want foundation.Code) Response {
	j.t.Helper()
	r := j.client.Call(context.Background(), Request{Op: op, ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, PPPoE: &in})
	if r.Code != want {
		j.t.Fatalf("%s got %s want %s", op, r.Code, want)
	}
	return r
}
func (j *ppJourney) collect(op string) *equipment.PPObservation {
	j.t.Helper()
	r := j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: op}, foundation.OK)
	id := r.Data.PPPoE.Job.ID
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r = j.call("pppoe.collect-status", equipment.PPInput{JobID: id}, foundation.OK)
		if r.Data.PPPoE.Job.State != "RUNNING" {
			if r.Data.PPPoE.Job.Code != equipment.OK && !(j.mode.Load() == 1 && r.Data.PPPoE.Job.Code == equipment.Limit && r.Data.PPPoE.Job.State == "FAILED") {
				j.t.Fatal(r.Data.PPPoE.Job)
			}
			return j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK).Data.PPPoE.Current
		}
		time.Sleep(time.Millisecond)
	}
	j.t.Fatal("job deadline")
	return nil
}
func TestPPPoERealClientPersistenceReplayAndQuota(t *testing.T) {
	j := newPPJourney(t)
	op := uuid.NewString()
	a := j.collect(op)
	if a == nil || a.Count != 2 {
		t.Fatal(a)
	}
	auth, commands := j.peer.Auth.Load(), j.peer.Commands.Load()
	ref := equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}
	j.call("pppoe.reference-save", ref, foundation.OK)
	j.call("pppoe.reference-save", ref, foundation.OK)
	j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, Query: &equipment.PPQuery{Duplicates: true}}, foundation.OK)
	j.call("pppoe.session-get", equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, RowID: uuid.NewString()}, foundation.Code(equipment.ObservationMissing))
	j.call("pppoe.compare", equipment.PPInput{DeviceID: j.device, BaseID: a.ID, TargetID: a.ID}, foundation.OK)
	ex := j.call("pppoe.export-page", equipment.PPInput{DeviceID: j.device, TargetID: a.ID, ExportKey: base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))}, foundation.OK)
	b, _ := json.Marshal(ex)
	for _, canary := range []string{"canary-login", "198.51.100.", "canary-caller", j.peer.Fingerprint(), "local-fixture-only"} {
		if bytes.Contains(b, []byte(canary)) {
			t.Fatal("export leaked", canary)
		}
	}
	for _, row := range ex.Data.PPPoE.ExportRows {
		if row.State != "ONLINE" || !row.Uptime.Known || row.Duplicates != 2 {
			t.Fatal("sanitized fields lost", row)
		}
	}
	if j.peer.Auth.Load() != auth || j.peer.Commands.Load() != commands {
		t.Fatal("local actions connected")
	}
	j.mode.Store(1)
	last := j.collect(uuid.NewString())
	if last.ID != a.ID {
		t.Fatal("limited replaced last good")
	}
	cur := j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK).Data.PPPoE
	if cur.Attempt.Quality.Status != "limited" || cur.Current.Provenance.Ended != a.Provenance.Ended {
		t.Fatal(cur)
	}
	// Fill the SAME ledger synthetically; no new ledger or pruning.
	c := j.store.UpdateEquipment(context.Background(), j.client.profile, func(s *equipment.State) equipment.Code {
		for len(s.Operations) < equipment.MaxOperations {
			s.Operations = append(s.Operations, equipment.Operation{ID: uuid.NewString(), Stamp: "synthetic-fill", Entity: uuid.NewString()})
		}
		return equipment.OK
	})
	if c != equipment.OK {
		t.Fatal(c)
	}
	j.mode.Store(0)
	j.collect(uuid.NewString())
	auth = j.peer.Auth.Load()
	// Reads still work at the mutation ceiling; the saved mutation replay is stable.
	j.collect(uuid.NewString())
	j.call("pppoe.reference-delete", equipment.PPInput{DeviceID: j.device, ReferenceID: ref.ReferenceID, Operation: uuid.NewString()}, foundation.Code(equipment.Limit))
	j.call("pppoe.reference-save", ref, foundation.OK)
	// After restart a deliberate submission is a fresh read, even with old ID.
	j.svc.Close()
	j.svc = equipment.New(j.auth, j.store, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{8}, 32), nil }), routerssh.Adapter{}, testEquipmentIDs{}, time.Now)
	AttachEquipment(j.server, j.svc)
	auth = j.peer.Auth.Load()
	j.collect(op)
	if j.peer.Auth.Load() != auth+1 {
		t.Fatal("fresh deliberate read after restart did not run once")
	}

}
func TestPPPoERecursiveSchemaAndCursor(t *testing.T) {
	j := newPPJourney(t)
	a := j.collect(uuid.NewString())
	q := Request{Version: 1, RequestID: "schema", ContractRevision: ProductRevision, Op: "pppoe.sessions-list", Capability: strings.Repeat("a", 64), Session: "s", Profile: j.client.profile, Activation: j.client.activation, ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, PPPoE: &equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, Query: &equipment.PPQuery{}}}
	b, _ := json.Marshal(q)
	if _, c := validate(b); c != foundation.OK {
		t.Fatal("schema positive control", c)
	}
	for _, body := range []string{strings.Replace(string(b), `"query":{`, `"query":{"search":null,`, 1), strings.Replace(string(b), `"query":{`, `"query":{"search":"x","search":"y",`, 1), strings.Replace(string(b), `"query":{`, `"query":{"shell":"forbidden",`, 1)} {
		if _, c := validate([]byte(body)); c != foundation.Invalid {
			t.Fatal("nested invalid accepted", c)
		}
	}
	server := &Server{}
	q.PPPoE.Query.Search = strings.Repeat("é", 80)
	q.PPPoE.Query.Sort = "login"
	cursor := server.ppCursor(q, 2048)
	if len(cursor) > 512 {
		t.Fatal(len(cursor))
	}
	q.PPPoE.Cursor = cursor
	if n, ok := server.ppOffset(q); !ok || n != 2048 {
		t.Fatal(n, ok)
	}
	q.PPPoE.Query.Search = "other"
	if _, ok := server.ppOffset(q); ok {
		t.Fatal("cross-query cursor accepted")
	}
	if j.peer.Commands.Load() != 1 {
		t.Fatal(fmt.Sprint(j.peer.Commands.Load()))
	}
}

func TestPPPoEJobSlotsAndInterruptedCurrent(t *testing.T) {
	j := newPPJourney(t)
	for i := 0; i < equipment.MaxJobs; i++ {
		j.collect(uuid.NewString())
	}
	r := j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}, foundation.Code(equipment.Limit))
	if r.ErrorDetail == nil || r.ErrorDetail.Reason != "job_slots" {
		t.Fatal("wrong quota cause", r.ErrorDetail)
	}
	// Persist a synthetic STARTED receipt/attempt, then reopen the service epoch.
	op := uuid.NewString()
	job := uuid.NewString()
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: op, WorkerEpoch: "previous-daemon"}
	if _, _, c := j.store.ReservePPCollection(context.Background(), j.client.profile, in, "synthetic-interrupted", job); c != equipment.OK {
		t.Fatal(c)
	}
	auths := j.peer.Auth.Load()
	cur := j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK).Data.PPPoE
	if cur.Attempt.State != "INTERRUPTED" || cur.Attempt.Code != "JOB_INTERRUPTED" || cur.Current == nil {
		t.Fatal("phantom running attempt", cur.Attempt)
	}
	if j.peer.Auth.Load() != auths {
		t.Fatal("read reconciled by network")
	}
}
func TestPPPoERealClientAuthorizationAndCursorBoundaries(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{PPRows: 32})
	a := j.collect(uuid.NewString())
	q := equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, Query: &equipment.PPQuery{Search: "synthetic"}}
	page := j.call("pppoe.sessions-list", q, foundation.OK).Data.PPPoE
	if page.Next == "" || len(page.Rows) != 16 {
		t.Fatal("positive page control")
	}
	q.Cursor = page.Next
	q.Query.Search = "other"
	j.call("pppoe.sessions-list", q, foundation.Code(equipment.CursorInvalid))
	q.Query.Search = "synthetic"
	second := j.collect(uuid.NewString())
	q.ObservationID = second.ID
	j.call("pppoe.sessions-list", q, foundation.Code(equipment.CursorInvalid))
	auths := j.peer.Auth.Load()
	request := Request{Op: "pppoe.current", ExpectedProfile: uuid.NewString(), ExpectedActivation: j.client.activation, PPPoE: &equipment.PPInput{DeviceID: j.device}}
	if r := j.client.Call(context.Background(), request); r.Code != foundation.ProfileChanged {
		t.Fatal("context bypass", r.Code)
	}
	j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK)
	for op, fields := range ppFields {
		raw := map[string]any{}
		for _, field := range fields {
			switch field {
			case "revision":
				raw[field] = 2
			case "query":
				raw[field] = map[string]any{}
			case "export_key":
				raw[field] = base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{9}, 32))
			case "export_part":
				raw[field] = "diagnostics"
			case "scope":
				raw[field] = equipment.PDCatalog
				if op == "pppoe.diagnostic-start" {
					raw[field] = equipment.PDPing
				}
			case "cursor":
				raw[field] = ""
			case "device_id":
				raw[field] = j.device
			case "observation_id", "base_id", "target_id":
				raw[field] = second.ID
			default:
				raw[field] = uuid.NewString()
			}
		}
		b, _ := json.Marshal(raw)
		var in equipment.PPInput
		if json.Unmarshal(b, &in) != nil {
			t.Fatal("test input")
		}
		r := j.client.Call(context.Background(), Request{Op: op, ExpectedProfile: uuid.NewString(), ExpectedActivation: j.client.activation, PPPoE: &in})
		if r.Code != foundation.ProfileChanged {
			t.Fatal("operation escaped common auth guard", op, r.Code)
		}
	}
	if j.peer.Auth.Load() != auths {
		t.Fatal("negative/local operation connected")
	}
}

func TestL06CReadReplayOnlyWhileActive(t *testing.T) {
	hold := make(chan struct{})
	released := false
	j := newPPJourney(t, sshfixture.Config{Hold: hold, PPRows: 1})
	defer func() {
		if !released {
			close(hold)
		}
	}()
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}
	first := j.call("pppoe.collect-start", in, foundation.OK).Data.PPPoE.Job
	again := j.call("pppoe.collect-start", in, foundation.OK).Data.PPPoE.Job
	if first.ID != again.ID {
		t.Fatal("active read started twice")
	}
	other := in
	other.Scope = equipment.PDCatalog
	j.call("pppoe.diagnostic-start", other, foundation.Code(equipment.Revision))
	close(hold)
	released = true
	deadline := time.Now().Add(3 * time.Second)
	for {
		job := j.call("pppoe.collect-status", equipment.PPInput{JobID: first.ID}, foundation.OK).Data.PPPoE.Job
		if job.State != "RUNNING" {
			if job.Code != equipment.OK {
				t.Fatal(job)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("held read did not complete")
		}
		time.Sleep(time.Millisecond)
	}
	if j.peer.Commands.Load() != 1 {
		t.Fatal("active read repeated remote command")
	}
	next := j.collect(in.Operation)
	if next == nil || j.peer.Commands.Load() != 2 {
		t.Fatal("completed read was durably replayed")
	}
}

type l06cPublicationAuthorization struct {
	equipment.Authorizer
	armed     atomic.Bool
	attempted chan equipment.Code
}

func (a *l06cPublicationAuthorization) WithAuthorization(ctx context.Context, uid uint32, q access.Command, fn func(uint64) equipment.Code) equipment.Code {
	c := a.Authorizer.WithAuthorization(ctx, uid, q, fn)
	// Worker checks/watchers use the20s network context. The separate publish
	// authorization has its own3s deadline. This observes denial without cancelling
	// a worker or granting authority to a revoked session.
	if until, ok := ctx.Deadline(); ok && a.armed.Load() && time.Until(until) <= equipment.PPPersistenceBudget {
		select {
		case a.attempted <- c:
		default:
		}
	}
	return c
}
func TestL06CRevokedBeforePublication(t *testing.T) {
	hold := make(chan struct{})
	released := false
	j := newPPJourney(t, sshfixture.Config{Hold: hold, PPRows: 1})
	defer func() {
		if !released {
			close(hold)
		}
	}()
	observed := &l06cPublicationAuthorization{Authorizer: j.auth, attempted: make(chan equipment.Code, 1)}
	j.svc.Close() // No jobs exist yet. Never close/cancel the job under test.
	j.svc = equipment.New(observed, j.store, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{8}, 32), nil }), routerssh.Adapter{}, testEquipmentIDs{}, time.Now)
	AttachEquipment(j.server, j.svc)
	profile := j.client.profile
	j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}, foundation.OK)
	deadline := time.Now().Add(3 * time.Second)
	for j.peer.Commands.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if j.peer.Commands.Load() != 1 {
		t.Fatal("remote read barrier")
	}
	observed.armed.Store(true)
	if r := j.client.Call(context.Background(), Request{Op: "access.logout"}); r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	close(hold)
	released = true
	select {
	case c := <-observed.attempted:
		if c == equipment.OK {
			t.Fatal("revoked publication authorized")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not reach publication authorization")
	}
	if r := j.client.Call(context.Background(), Request{Op: "access.login", Name: "synthetic", Password: "synthetic PPPoE password"}); r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	if r := j.client.Call(context.Background(), Request{Op: "company.select", Name: profile}); r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	cur := j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK)
	if cur.Data.PPPoE.Current != nil {
		t.Fatal("published after effective logout")
	}
}
