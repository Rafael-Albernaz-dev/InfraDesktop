package ipc

import (
	"bytes"
	"context"
	"encoding/json"
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
	"reflect"
	"strings"
	"testing"
	"time"
)

type testEquipmentIDs struct{}

func (testEquipmentIDs) ID() (string, equipment.Code) { return uuid.NewString(), equipment.OK }
func TestEquipmentStrictSchema(t *testing.T) {
	for op, allowed := range equipmentFields {
		q := Request{Version: 1, ContractRevision: ProductRevision, RequestID: "new", Op: op, Capability: strings.Repeat("a", 64), Session: "s", Profile: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", Activation: "a", ExpectedProfile: "aaaaaaaa-aaaa-4aaa-aaaa-aaaaaaaaaaaa", ExpectedActivation: "a", Equipment: &EquipmentInput{}}
		b, _ := json.Marshal(q)
		if _, c := validate(b); c != foundation.OK {
			t.Fatal(op, c)
		}
		for _, field := range []string{"secret", "path", "shell", "sql", "active", "origin", "job_id", "revision", "operation_id", "search", "owner"} {
			ok := false
			for _, f := range allowed {
				if f == field {
					ok = true
				}
			}
			if ok {
				continue
			}
			var m map[string]any
			_ = json.Unmarshal(b, &m)
			m["equipment"] = map[string]any{field: "forbidden"}
			bad, _ := json.Marshal(m)
			if _, c := validate(bad); c != foundation.Invalid {
				t.Fatal("cross-operation field accepted", op, field, c)
			}
		}
		for _, body := range []string{strings.Replace(string(b), `"equipment":{}`, `"equipment":{"id":"one","id":"two"}`, 1), strings.Replace(string(b), `"equipment":{}`, `"equipment":{"id":null}`, 1), strings.Replace(string(b), `"contract_revision":8`, `"contract_revision":5`, 1)} {
			if _, c := validate([]byte(body)); c == foundation.OK {
				t.Fatal("malformed/old contract accepted", op)
			}
		}
	}
}
func TestIntegrationEquipmentSQLiteVaultSSH(t *testing.T) {
	root := t.TempDir()
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	dirs := map[string]string{}
	for _, name := range []string{"runtime", "state", "client"} {
		dirs[name] = filepath.Join(root, name)
		if e := os.Mkdir(dirs[name], 0700); e != nil {
			t.Fatal(e)
		}
	}
	store, e := storage.Open(dirs["state"])
	if e != nil {
		t.Fatal(e)
	}
	uid := uint32(os.Getuid())
	account := access.New(store, security.Crypto{}, time.Now, uid, "activation", store)
	service := equipment.New(account, store, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{7}, 32), nil }), routerssh.Adapter{}, testEquipmentIDs{}, time.Now)
	server, e := ListenAccess(dirs["runtime"], uid, account, foundation.Status{Instance: "part1"}, store.Close)
	if e != nil {
		t.Fatal(e)
	}
	AttachEquipment(server, service)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	defer func() {
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	peer, e := sshfixture.Start(sshfixture.Config{Family: "v6"})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	client := NewAccountClient(dirs["runtime"], dirs["client"], uid)
	call := func(q Request, want foundation.Code) Response {
		t.Helper()
		r := client.Call(context.Background(), q)
		if r.Code != want {
			t.Fatalf("%s got %s want %s", q.Op, r.Code, want)
		}
		return r
	}
	call(Request{Op: "access.register", Name: "technical", Password: "isolated IPC password"}, foundation.OK)
	call(Request{Op: "access.login", Name: "technical", Password: "isolated IPC password"}, foundation.OK)
	r := call(Request{Op: "company.create", Name: "X"}, foundation.OK)
	company := r.Data.Companies[0].ID
	call(Request{Op: "company.select", Name: company}, foundation.OK)
	request := func(op string, input *EquipmentInput) Request {
		return Request{Op: op, ExpectedProfile: client.profile, ExpectedActivation: client.activation, Equipment: input}
	}
	device := &EquipmentInput{ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Test peer", Address: "127.0.0.1", Port: peer.Port(), Transport: "ssh", User: "reader", Secret: sshfixture.Password}
	call(request("inventory.enroll", device), foundation.OK)
	call(request("inventory.enroll", device), foundation.OK)
	reconciled := call(request("inventory.enroll-status", &EquipmentInput{ID: device.ID, Operation: device.Operation}), foundation.OK)
	if reconciled.Data.Equipment.OperationState != "COMMITTED" || reconciled.Data.Equipment.Device.CredentialID == "" {
		t.Fatal("direct IPC enrollment not reconciled")
	}
	call(request("inventory.enroll-status", &EquipmentInput{ID: uuid.NewString(), Operation: device.Operation}), foundation.Code(equipment.Revision))
	call(request("inventory.enroll-status", &EquipmentInput{ID: device.ID, Operation: uuid.NewString()}), foundation.OK)
	for _, op := range []string{"inventory.list", "credential.list"} {
		r = call(request(op, &EquipmentInput{ID: device.ID}), foundation.Invalid)
		_ = r
	}
	r = call(request("credential.list", &EquipmentInput{}), foundation.OK)
	b, _ := json.Marshal(r)
	if bytes.Contains(b, []byte(sshfixture.Password)) || bytes.Contains(b, []byte("envelope")) || bytes.Contains(b, []byte("ciphertext")) {
		t.Fatal("credential response leaked vault")
	}
	inspect := &EquipmentInput{ID: device.ID, Revision: 1, Operation: uuid.NewString()}
	r = call(request("trust.inspect", inspect), foundation.OK)
	poll := func(id string) equipment.Job {
		t.Helper()
		until := time.Now().Add(3 * time.Second)
		for time.Now().Before(until) {
			r := call(request("device.query-status", &EquipmentInput{JobID: id}), foundation.OK)
			if r.Data.Equipment.Job.State != "RUNNING" {
				return *r.Data.Equipment.Job
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("job deadline")
		return equipment.Job{}
	}
	call(request("inventory.enroll-status", &EquipmentInput{ID: device.ID, Operation: inspect.Operation}), foundation.Code(equipment.Revision))
	job := poll(r.Data.Equipment.Job.ID)
	if job.Code != equipment.OK || peer.Auth.Load() != 0 {
		t.Fatal("inspect sent authentication")
	}
	call(request("trust.confirm", &EquipmentInput{ID: device.ID, Operation: uuid.NewString(), Revision: 1, JobID: job.ID, Fingerprint: job.Fingerprint}), foundation.OK)
	r = call(request("device.query-start", &EquipmentInput{ID: device.ID, Operation: uuid.NewString(), Revision: 2}), foundation.OK)
	job = poll(r.Data.Equipment.Job.ID)
	if job.Code != equipment.OK || job.Result.Family != "v6" {
		t.Fatal("IPC query did not return observed fixture data", job.Code)
	}
	r = call(request("inventory.get", &EquipmentInput{ID: device.ID}), foundation.OK)
	if len(r.Data.Equipment.Device.Result.Interfaces) != 1 {
		t.Fatal("observation not persisted")
	}
	// Saving metadata through the real client preserves the completed reading,
	// including for inactive devices, without another SSH authentication.
	current := r.Data.Equipment.Device
	auths, commands := peer.Auth.Load(), peer.Commands.Load()
	for _, active := range []bool{true, false} {
		if !active {
			current = call(request("inventory.set-active", &EquipmentInput{ID: current.ID, Revision: current.Revision, Operation: uuid.NewString(), Active: false}), foundation.OK).Data.Equipment.Device
		}
		before := *current
		edit := &EquipmentInput{ID: before.ID, Revision: before.Revision, Operation: uuid.NewString(), Name: "Edited metadata", Address: before.Address, Port: before.Port, Transport: before.Transport, CredentialID: before.CredentialID}
		current = call(request("inventory.update", edit), foundation.OK).Data.Equipment.Device
		if current.Revision != before.Revision+1 || current.Active != active || current.Name != edit.Name || !reflect.DeepEqual(current.Result, before.Result) || !reflect.DeepEqual(current.LastAttempt, before.LastAttempt) || !reflect.DeepEqual(current.TrustAcceptance, before.TrustAcceptance) {
			t.Fatal("metadata edit lost reading, attempt, trust or active state through IPC")
		}
		replay := call(request("inventory.update", edit), foundation.OK).Data.Equipment.Device
		stored := call(request("inventory.get", &EquipmentInput{ID: current.ID}), foundation.OK).Data.Equipment.Device
		if !reflect.DeepEqual(replay, current) || !reflect.DeepEqual(stored, current) || peer.Auth.Load() != auths || peer.Commands.Load() != commands {
			t.Fatal("metadata edit/replay changed persisted data or performed SSH")
		}
	}
	old := request("inventory.list", &EquipmentInput{})
	call(Request{Op: "company.create", Name: "Y"}, foundation.OK)
	r = call(Request{Op: "access.bootstrap"}, foundation.OK)
	call(Request{Op: "company.select", Name: r.Data.Companies[1].ID}, foundation.OK)
	call(old, foundation.ProfileChanged)
	r = call(request("inventory.list", &EquipmentInput{}), foundation.OK)
	isolated := call(request("inventory.enroll-status", &EquipmentInput{ID: device.ID, Operation: device.Operation}), foundation.OK)
	if isolated.Data.Equipment.OperationState != "NOT_FOUND" || isolated.Data.Equipment.Device != nil {
		t.Fatal("X receipt leaked into Y")
	}
	if len(r.Data.Equipment.Devices) != 0 {
		t.Fatal("X leaked into Y via IPC")
	}
}

func TestEquipmentSchemaSnapshotAndBounds(t *testing.T) {
	b, e := os.ReadFile("schema-v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Revision   int                 `json:"product_revision"`
		Operations map[string][]string `json:"equipment_operations"`
	}
	if json.Unmarshal(b, &schema) != nil || schema.Revision != ProductRevision || len(schema.Operations) != len(equipmentFields) {
		t.Fatal("equipment schema drift")
	}
	for op, fields := range equipmentFields {
		actual := schema.Operations[op]
		if len(actual) != len(fields) {
			t.Fatal("schema fields drift", op)
		}
		for i, f := range fields {
			if actual[i] != f {
				t.Fatal("schema allowlist drift", op)
			}
		}
	}
}
