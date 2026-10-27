package ipc

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
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

func (j *ppJourney) diagnostic(in equipment.PPInput, want equipment.Code) *equipment.PDDiagnostic {
	j.t.Helper()
	r := j.call("pppoe.diagnostic-start", in, foundation.OK)
	id := r.Data.PPPoE.Job.ID
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r = j.call("pppoe.diagnostic-status", equipment.PPInput{JobID: id}, foundation.OK)
		job := r.Data.PPPoE.Job
		if job.State != "RUNNING" {
			if job.Code != want {
				j.t.Fatalf("diagnostic job %+v want %s", job, want)
			}
			return j.call("pppoe.diagnostic-current", equipment.PPInput{DeviceID: j.device, Scope: in.Scope}, foundation.OK).Data.PPPoE.Diagnostic
		}
		time.Sleep(time.Millisecond)
	}
	j.t.Fatal("diagnostic job did not finish")
	return nil
}

func TestPPDiagnosticRealClientScopesPagesSanitizedExportAndReplay(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{RealTime: true, PPRows: 2})
	observation := j.collect(uuid.NewString())
	inputs := []equipment.PPInput{}
	for _, scope := range []string{equipment.PDCatalog, equipment.PDRadius} {
		in := equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: scope, Operation: uuid.NewString()}
		inputs = append(inputs, in)
		diagnostic := j.diagnostic(in, equipment.OK)
		if diagnostic == nil || diagnostic.Count == 0 {
			t.Fatal("empty integration diagnostic", scope)
		}
		before := j.peer.Commands.Load()
		page := j.call("pppoe.diagnostic-list", equipment.PPInput{DeviceID: j.device, Scope: scope, DiagnosticID: diagnostic.ID}, foundation.OK)
		if len(page.Data.PPPoE.DiagnosticRows) == 0 {
			t.Fatal("no persisted rows")
		}
		exported := j.call("pppoe.export-page", equipment.PPInput{DeviceID: j.device, TargetID: observation.ID, Scope: scope, DiagnosticID: diagnostic.ID, ExportPart: "diagnostics", ExportKey: base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}, foundation.OK)
		encoded, _ := json.Marshal(exported)
		for _, canary := range []string{"fixture-ether", "fixture-service", "198.51.100.20", "local-fixture-only", diagnostic.ID} {
			if bytes.Contains(encoded, []byte(canary)) {
				t.Fatal("diagnostic export leaked", canary)
			}
		}
		if len(exported.Data.PPPoE.ExportDiagnostics) == 0 || j.peer.Commands.Load() != before {
			t.Fatal("stored page/export caused network")
		}
		// A second deliberate read after completion gets a new job, not a permanent replay.
		again := j.diagnostic(in, equipment.OK)
		if again.ID == diagnostic.ID || j.peer.Commands.Load() != before+1 {
			t.Fatal("completed read retained permanent replay")
		}
	}
	session := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: observation.ID}, foundation.OK).Data.PPPoE.Rows[0]
	ping := equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDPing, Operation: uuid.NewString(), ObservationID: observation.ID, RowID: session.ID}
	result := j.diagnostic(ping, equipment.OK)
	if result == nil || result.SourceObservation != observation.ID || result.SourceRow != session.ID || j.peer.PingCalls.Load() != 1 {
		t.Fatal("ping source/run identity", result, j.peer.PingCalls.Load())
	}
	// No automatic ping retry; a deliberate second run is a fresh finite action.
	j.diagnostic(ping, equipment.OK)
	if j.peer.PingCalls.Load() != 2 {
		t.Fatal("explicit second ping did not execute once")
	}
	before := j.peer.Auth.Load()
	j.svc.Close()
	j.svc = equipment.New(j.auth, j.store, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{8}, 32), nil }), routerssh.Adapter{}, testEquipmentIDs{}, time.Now)
	AttachEquipment(j.server, j.svc)
	j.call("pppoe.diagnostic-start", ping, "PING_READING_STALE")
	if j.peer.Auth.Load() != before {
		t.Fatal("stale ping after restart reached network")
	}
	if c := j.store.UpdateEquipment(context.Background(), j.client.profile, func(s *equipment.State) equipment.Code {
		for len(s.Operations) < equipment.MaxOperations {
			s.Operations = append(s.Operations, equipment.Operation{ID: uuid.NewString(), Stamp: "synthetic-budget", Entity: uuid.NewString()})
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	j.diagnostic(inputs[0], equipment.OK)
	j.diagnostic(equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDCatalog, Operation: uuid.NewString()}, equipment.OK)
	if j.peer.Auth.Load() != before+2 {
		t.Fatal("mutation ceiling blocked reads")
	}

}

func TestPPDiagnosticStrictInputAndContextBeforeNetwork(t *testing.T) {
	j := newPPJourney(t)
	observation := j.collect(uuid.NewString())
	before := j.peer.Auth.Load()
	for _, in := range []equipment.PPInput{
		{DeviceID: j.device, Revision: 2, Scope: "other", Operation: uuid.NewString()},
		{DeviceID: j.device, Revision: 2, Scope: equipment.PDCatalog, Operation: uuid.NewString(), ObservationID: observation.ID},
		{DeviceID: j.device, Revision: 2, Scope: equipment.PDPing, Operation: uuid.NewString()},
	} {
		j.call("pppoe.diagnostic-start", in, foundation.Invalid)
	}
	j.call("pppoe.diagnostic-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDPing, Operation: uuid.NewString(), ObservationID: observation.ID, RowID: uuid.NewString()}, foundation.Code(equipment.ObservationMissing))
	if j.peer.Auth.Load() != before {
		t.Fatal("invalid input reached SSH")
	}
	q := Request{Version: 1, ContractRevision: ProductRevision, RequestID: "pd-recursive", Op: "pppoe.diagnostic-start", Capability: strings.Repeat("a", 64), Session: "s", Profile: j.client.profile, Activation: j.client.activation, ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, PPPoE: &equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDCatalog, Operation: uuid.NewString()}}
	raw, _ := json.Marshal(q)
	for _, malformed := range []string{strings.Replace(string(raw), `"scope":"catalog"`, `"scope":"catalog","scope":"radius"`, 1), strings.Replace(string(raw), `"scope":"catalog"`, `"scope":"catalog","address":"198.51.100.1"`, 1), strings.Replace(string(raw), `"scope":"catalog"`, `"scope":null`, 1)} {
		if _, c := validate([]byte(malformed)); c != foundation.Invalid {
			t.Fatal("invalid recursive/target field accepted", c)
		}
	}
}

func TestPPDiagnosticRadiusMonitorFailurePreservesGoodDataAndDate(t *testing.T) {
	var denied atomic.Bool
	j := newPPJourney(t, sshfixture.Config{Command: func(command string) (string, bool) {
		if command == routerssh.PDRadiusCommand {
			if denied.Load() {
				return "failure: not enough permissions\n", true
			}
			return sshfixture.RadiusTranscript("7.26"), true
		}
		return "", false
	}})
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDRadius, Operation: uuid.NewString()}
	first := j.diagnostic(in, equipment.OK)
	denied.Store(true)
	in.Operation = uuid.NewString()
	last := j.diagnostic(in, "REMOTE_PERMISSION_DENIED")
	if first == nil || last == nil || last.ID != first.ID || last.Provenance.Ended != first.Provenance.Ended {
		t.Fatal("monitor failure replaced good data/date", first, last)
	}
	page := j.call("pppoe.diagnostic-list", equipment.PPInput{DeviceID: j.device, Scope: equipment.PDRadius, DiagnosticID: first.ID}, foundation.OK)
	if len(page.Data.PPPoE.DiagnosticRows) != 2 || page.Data.PPPoE.DiagnosticRows[1].Counters.Requests != "100" {
		t.Fatal("failed monitor zeroed counters", page.Data.PPPoE)
	}
}

func TestPPDiagnosticSharedAdmissionCancellationAndRevocation(t *testing.T) {
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "logout"}[revoke], func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			released := false
			defer func() {
				if !released {
					close(release)
				}
			}()
			j := newPPJourney(t, sshfixture.Config{Command: func(command string) (string, bool) {
				if command == routerssh.PDCatalogCommand {
					close(entered)
					<-release
					return sshfixture.CatalogTranscript("7.26"), true
				}
				return "", false
			}})
			in := equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDCatalog, Operation: uuid.NewString()}
			start := j.call("pppoe.diagnostic-start", in, foundation.OK)
			job := start.Data.PPPoE.Job.ID
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("worker did not reach controlled peer barrier")
			}
			before := j.peer.Auth.Load()
			j.call("pppoe.diagnostic-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDRadius, Operation: uuid.NewString()}, foundation.Busy)
			changed := in
			changed.Scope = equipment.PDRadius
			j.call("pppoe.diagnostic-start", changed, foundation.Code(equipment.Revision))
			j.call("pppoe.collect-cancel", equipment.PPInput{JobID: job}, foundation.Invalid)
			if j.call("pppoe.diagnostic-status", equipment.PPInput{JobID: job}, foundation.OK).Data.PPPoE.Job.State != "RUNNING" {
				t.Fatal("wrong-kind cancel affected diagnostic")
			}
			if revoke {
				if result := j.client.Call(context.Background(), Request{Op: "access.logout"}); result.Code != foundation.OK {
					t.Fatal(result.Code)
				}
			} else {
				j.call("pppoe.diagnostic-cancel", equipment.PPInput{JobID: job}, foundation.OK)
			}
			close(release)
			released = true
			j.svc.Close()
			view, code := j.store.PPDiagnosticCurrent(context.Background(), j.client.profile, in)
			if revoke {
				view, code = j.store.PPDiagnosticCurrent(context.Background(), start.Data.Profile, in)
			}
			if code != equipment.OK || view.Diagnostic != nil || j.peer.Auth.Load() != before {
				t.Fatal("cancel/revocation published or caused another connection", view, code, j.peer.Auth.Load())
			}
		})
	}
}

func TestPPDiagnosticExportEnforcesTargetContextAtIPC(t *testing.T) {
	j := newPPJourney(t)
	observation := j.collect(uuid.NewString())
	diagnostic := j.diagnostic(equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDCatalog, Operation: uuid.NewString()}, equipment.OK)
	input := equipment.PPInput{DeviceID: j.device, TargetID: observation.ID, Scope: equipment.PDCatalog, DiagnosticID: diagnostic.ID, ExportPart: "diagnostics", ExportKey: base64.RawStdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32))}
	good := j.call("pppoe.export-page", input, foundation.OK)
	if len(good.Data.PPPoE.ExportDiagnostics) == 0 {
		t.Fatal("matching context lost")
	}
	before := j.peer.Commands.Load()
	input.TargetID = uuid.NewString()
	denied := j.call("pppoe.export-page", input, foundation.Code(equipment.ObservationMissing))
	if denied.Data != nil {
		t.Fatal("missing target exposed diagnostic rows")
	}
	input.TargetID = observation.ID
	db, err := sql.Open("sqlite", filepath.Join(j.root, "state", "companies", j.client.profile+".db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	diagnostic.Provenance.Version = "6.49"
	body, _ := json.Marshal(diagnostic)
	if _, err = db.Exec("UPDATE pppoe_diagnostic_observations SET meta_json=? WHERE id=?", string(body), diagnostic.ID); err != nil {
		t.Fatal(err)
	}
	denied = j.call("pppoe.export-page", input, foundation.Code(equipment.ObservationIncompatible))
	if denied.Data != nil || j.peer.Commands.Load() != before {
		t.Fatal("incompatible export exposed data or reached network")
	}
	session := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: observation.ID}, foundation.OK).Data.PPPoE.Rows[0]
	ping := j.diagnostic(equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDPing, Operation: uuid.NewString(), ObservationID: observation.ID, RowID: session.ID}, equipment.OK)
	input.Scope = equipment.PDPing
	input.DiagnosticID = ping.ID
	good = j.call("pppoe.export-page", input, foundation.OK)
	if len(good.Data.PPPoE.ExportDiagnostics) != 1 {
		t.Fatal("matching ping source lost")
	}
	ping.SourceObservation = uuid.NewString()
	body, _ = json.Marshal(ping)
	if _, err = db.Exec("UPDATE pppoe_diagnostic_observations SET meta_json=? WHERE id=?", string(body), ping.ID); err != nil {
		t.Fatal(err)
	}
	before = j.peer.Commands.Load()
	denied = j.call("pppoe.export-page", input, foundation.Code(equipment.ObservationIncompatible))
	if denied.Data != nil || j.peer.Commands.Load() != before {
		t.Fatal("ping source mismatch exported or reached network")
	}
}

// I26: bounded synthetic workload through the actual client/IPC/service/SQLite/
// pinned SSH path. Counts and measurements do not establish firmware or4GB support.
func TestPPDiagnosticCatalogBoundaryWorkloadRealIPC(t *testing.T) {
	var count atomic.Int32
	count.Store(2047)
	j := newPPJourney(t, sshfixture.Config{Command: func(command string) (string, bool) {
		if command != routerssh.PDCatalogCommand {
			return "", false
		}
		n := int(count.Load())
		var body strings.Builder
		fmt.Fprintf(&body, "D\t1\tcatalog\t7.26\t2d1h1s\t%d\n", n)
		if n > equipment.PPMaxRows {
			body.WriteString("Z\t0\trows\t2d1h1s\n")
			return body.String(), true
		}
		for i := 0; i < n; i++ {
			fmt.Fprintf(&body, "I\t*%X\tdynamic-%04d\tpppoe-in\ttrue\tfalse\ttrue\n", i+1, i)
		}
		fmt.Fprintf(&body, "Z\t%d\tcomplete\t2d1h1s\n", n)
		return body.String(), true
	}})
	var previous *equipment.PDDiagnostic
	for _, n := range []int{2047, 2048, 2049} {
		count.Store(int32(n))
		started := time.Now()
		want := equipment.OK
		if n > 2048 {
			want = equipment.Limit
		}
		current := j.diagnostic(equipment.PPInput{DeviceID: j.device, Revision: 2, Scope: equipment.PDCatalog, Operation: uuid.NewString()}, want)
		if current == nil {
			t.Fatal("no good catalog")
		}
		if n <= 2048 && current.Count != n {
			t.Fatal("silently truncated catalog", current.Count, n)
		}
		if n > 2048 && (current.ID != previous.ID || current.Provenance.Ended != previous.Provenance.Ended) {
			t.Fatal("limit replaced good catalog/date")
		}
		if current.Bytes > equipment.PPObservationBytes {
			t.Fatal("observation budget bypass")
		}
		remote := j.peer.Commands.Load()
		cursor := ""
		seen := map[string]bool{}
		maxFrame := 0
		pages := 0
		for {
			in := equipment.PPInput{DeviceID: j.device, Scope: equipment.PDCatalog, DiagnosticID: current.ID, Cursor: cursor}
			page := j.call("pppoe.diagnostic-list", in, foundation.OK)
			encoded, _ := json.Marshal(page)
			if len(encoded) > 48*1024 {
				t.Fatal("projection exceeds48KiB", len(encoded))
			}
			if len(encoded) > maxFrame {
				maxFrame = len(encoded)
			}
			if len(page.Data.PPPoE.DiagnosticRows) > equipment.PageSize {
				t.Fatal("page limit bypass")
			}
			for _, row := range page.Data.PPPoE.DiagnosticRows {
				if seen[row.ID] {
					t.Fatal("duplicate row across pages")
				}
				seen[row.ID] = true
			}
			pages++
			cursor = page.Data.PPPoE.Next
			if cursor == "" {
				break
			}
			if pages > 128 {
				t.Fatal("unbounded page loop")
			}
		}
		if len(seen) != current.Count || j.peer.Commands.Load() != remote {
			t.Fatal("incomplete local paging or extra network", len(seen), current.Count)
		}
		rss := []string{}
		if raw, e := os.ReadFile("/proc/self/status"); e == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(line, "VmRSS:") || strings.HasPrefix(line, "VmHWM:") {
					rss = append(rss, line)
				}
			}
		}
		var dbBytes int64
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if info, e := os.Stat(filepath.Join(j.root, "state", "companies", j.client.profile+".db"+suffix)); e == nil {
				dbBytes += info.Size()
			}
		}
		t.Logf("synthetic catalog requested=%d retained=%d pages=%d observationBytes=%d maxResponseFrame=%d elapsed=%s sqlitePhysicalBytes=%d selfRSS=%v", n, current.Count, pages, current.Bytes, maxFrame, time.Since(started), dbBytes, rss)
		previous = current
	}
}
