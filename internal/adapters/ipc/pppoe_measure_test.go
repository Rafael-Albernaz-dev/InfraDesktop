//go:build l06measure

package ipc

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestPPPoEMeasure(t *testing.T) {
	var n atomic.Int32
	var tick atomic.Int32
	j := newPPJourney(t, sshfixture.Config{PPPoE: func() string { return sshfixture.PPPoETranscript(int(n.Load()), "7.26", int(tick.Add(1))) }})
	path := filepath.Join(j.root, "state", "companies", j.client.profile+".db")
	db, e := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	size := func(path string) int64 {
		st, e := os.Stat(path)
		if e != nil {
			return 0
		}
		return st.Size()
	}
	records := []map[string]any{}
	maxFrame := 0
	frame := func(r Response) {
		b, _ := json.Marshal(r)
		if len(b) > maxFrame {
			maxFrame = len(b)
		}
		if len(b) > 48*1024 {
			t.Fatal("frame budget", len(b))
		}
	}
	collect := func(count int) map[string]any {
		n.Store(int32(count))
		start := time.Now()
		r := j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}, foundation.OK)
		frame(r)
		job := r.Data.PPPoE.Job.ID
		pollMax := time.Duration(0)
		polls := 0
		deadline := time.Now().Add(equipment.QueryBudget + time.Second)
		for time.Now().Before(deadline) {
			at := time.Now()
			r = j.call("pppoe.collect-status", equipment.PPInput{JobID: job}, foundation.OK)
			latency := time.Since(at)
			if latency > pollMax {
				pollMax = latency
			}
			polls++
			frame(r)
			if r.Data.PPPoE.Job.State != "RUNNING" {
				break
			}
			time.Sleep(time.Millisecond)
		}
		if r.Data.PPPoE.Job.State == "RUNNING" {
			t.Fatal("bounded job did not terminate")
		}
		elapsed := time.Since(start)
		cur := j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK)
		frame(cur)
		if count > equipment.PPMaxRows && (r.Data.PPPoE.Job.Code != equipment.Limit || cur.Data.PPPoE.Attempt.Quality.Status != "limited") {
			t.Fatal("large collection not limited", count, r)
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		rec := map[string]any{"requested_rows": count, "job_code": r.Data.PPPoE.Job.Code, "quality": cur.Data.PPPoE.Attempt.Quality, "elapsed_ms": float64(elapsed.Microseconds()) / 1000, "poll_max_ms": float64(pollMax.Microseconds()) / 1000, "polls": polls, "db_bytes": size(path), "wal_bytes": size(path + "-wal"), "heap_alloc": m.HeapAlloc, "heap_sys": m.HeapSys}
		if cur.Data.PPPoE.Current != nil {
			rec["retained_count"] = cur.Data.PPPoE.Current.Count
			rec["payload_bytes"] = cur.Data.PPPoE.Current.Bytes
			in := equipment.PPInput{DeviceID: j.device, ObservationID: cur.Data.PPPoE.Current.ID}
			at := time.Now()
			page := j.call("pppoe.sessions-list", in, foundation.OK)
			frame(page)
			rec["first_page_ms"] = float64(time.Since(at).Microseconds()) / 1000
		}
		records = append(records, rec)
		return rec
	}
	var reference string
	for _, count := range []int{0, 16, 128, 1000, 2047, 2048, 2049, 10000, 50000} {
		collect(count)
		if count == 128 {
			cur := j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK)
			reference = cur.Data.PPPoE.Current.ID
			j.call("pppoe.reference-save", equipment.PPInput{DeviceID: j.device, ObservationID: reference, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}, foundation.OK)
		}
	}
	// Ten replacements retain precisely one current plus the explicit reference.
	for i := 0; i < 10; i++ {
		collect(16)
	}
	var stored int
	if db.QueryRow("SELECT count(*) FROM pppoe_observations").Scan(&stored) != nil || stored != 2 {
		t.Fatal("hidden history", stored)
	}
	cur := j.call("pppoe.current", equipment.PPInput{DeviceID: j.device}, foundation.OK)
	at := time.Now()
	comp := j.call("pppoe.compare", equipment.PPInput{DeviceID: j.device, BaseID: reference, TargetID: cur.Data.PPPoE.Current.ID}, foundation.OK)
	frame(comp)
	compareMs := float64(time.Since(at).Microseconds()) / 1000
	// Read actual own-process RSS high water and report scope, never data values.
	status, _ := os.ReadFile("/proc/self/status")
	rss := ""
	for _, line := range strings.Split(string(status), "\n") {
		if strings.HasPrefix(line, "VmHWM:") {
			rss = line
		}
	}
	out := map[string]any{"scope": "synthetic real Unix IPC/SQLite/SSH; not firmware or 4GB acceptance", "go": runtime.Version(), "cpus": runtime.NumCPU(), "records": records, "max_frame_bytes": maxFrame, "comparison_ms": compareMs, "retained_observations": stored, "rss_high_water": rss, "root_cleanup": "t.TempDir after owned peer/server shutdown", "status": "PASS"}
	b, _ := json.MarshalIndent(out, "", "  ")
	if dir := os.Getenv("L06_MEASURE_DIR"); dir != "" {
		if e := os.WriteFile(filepath.Join(dir, "measure-data.json"), append(b, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log(string(b))
	_ = context.Background()
}

// A barrier at the real store port proves overlap, without replacing SQLite or
// the implementation being measured. Its controlled wait is included in timings.
type ppMeasuredStore struct {
	equipment.Store
	equipment.PPStore
	entered, release chan struct{}
	gate             atomic.Bool
}

func (s *ppMeasuredStore) PPPage(ctx context.Context, company string, in equipment.PPInput) (equipment.PPView, equipment.Code) {
	if s.gate.CompareAndSwap(true, false) {
		close(s.entered)
		select {
		case <-s.release:
		case <-ctx.Done():
			return equipment.PPView{}, equipment.Unavailable
		}
	}
	return s.PPStore.PPPage(ctx, company, in)
}

type ppMeasuredRemote struct {
	routerssh.Adapter
	returned chan error
}

func (r *ppMeasuredRemote) CollectPPPoE(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code) (*equipment.PPObservation, equipment.Code) {
	o, c := r.Adapter.CollectPPPoE(ctx, d, user, secret, check)
	r.returned <- ctx.Err()
	return o, c
}

func TestPPPoEMeasureTwoJobs(t *testing.T) {
	hold := make(chan struct{}, 2)
	hold <- struct{}{}
	hold <- struct{}{}
	var tick atomic.Int32
	cfg := sshfixture.Config{Hold: hold, PPPoE: func() string { return sshfixture.PPPoETranscript(1000, "7.26", int(tick.Add(1))) }}
	j := newPPJourney(t, cfg)
	a := j.collect(uuid.NewString())
	j.call("pppoe.reference-save", equipment.PPInput{DeviceID: j.device, ObservationID: a.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}, foundation.OK)
	b := j.collect(uuid.NewString())
	peer2, e := sshfixture.Start(cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer peer2.Close()
	defer close(hold)
	second := uuid.NewString()
	third := uuid.NewString()
	if c := j.store.UpdateEquipment(context.Background(), j.client.profile, func(s *equipment.State) equipment.Code {
		d := s.Devices[0]
		d.ID = second
		d.Port = peer2.Port()
		d.Fingerprint = peer2.Fingerprint()
		s.Devices = append(s.Devices, d)
		d.ID = third
		d.Port = 1
		s.Devices = append(s.Devices, d)
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	j.svc.Close() // Setup's idle service; no running work is cancelled here.
	measured := &ppMeasuredStore{Store: j.store, PPStore: j.store, entered: make(chan struct{}), release: make(chan struct{})}
	remote := &ppMeasuredRemote{returned: make(chan error, 2)}
	j.svc = equipment.New(j.auth, measured, vault.New(func() ([]byte, error) { return bytes.Repeat([]byte{8}, 32), nil }), remote, testEquipmentIDs{}, time.Now)
	AttachEquipment(j.server, j.svc)
	one := j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}, foundation.OK).Data.PPPoE.Job.ID
	two := j.call("pppoe.collect-start", equipment.PPInput{DeviceID: second, Revision: 2, Operation: uuid.NewString()}, foundation.OK).Data.PPPoE.Job.ID
	j.call("pppoe.collect-start", equipment.PPInput{DeviceID: third, Revision: 2, Operation: uuid.NewString()}, foundation.Busy)
	deadline := time.Now().Add(3 * time.Second)
	for (j.peer.Commands.Load() < 3 || peer2.Commands.Load() < 1) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if j.peer.Commands.Load() != 3 || peer2.Commands.Load() != 1 {
		t.Fatal("two-job command barriers")
	}
	pollStart := time.Now()
	j.call("pppoe.collect-status", equipment.PPInput{JobID: one}, foundation.OK)
	j.call("pppoe.collect-status", equipment.PPInput{JobID: two}, foundation.OK)
	pollMs := float64(time.Since(pollStart).Microseconds()) / 1000
	observerDir := filepath.Join(j.root, "observer")
	if e := os.Mkdir(observerDir, 0700); e != nil {
		t.Fatal(e)
	}
	observer := NewAccountClient(filepath.Join(j.root, "runtime"), observerDir, uint32(os.Getuid()))
	if r := observer.Call(context.Background(), Request{Op: "access.login", Name: "synthetic", Password: "synthetic PPPoE password"}); r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	measured.gate.Store(true)
	done := make(chan Response, 1)
	at := time.Now()
	go func() {
		done <- j.client.Call(context.Background(), Request{Op: "pppoe.compare", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, PPPoE: &equipment.PPInput{DeviceID: j.device, BaseID: a.ID, TargetID: b.ID}})
	}()
	select {
	case <-measured.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("compare did not enter real store")
	}
	snapshotStart := time.Now()
	snapshotDone := make(chan Response, 1)
	go func() { snapshotDone <- observer.Call(context.Background(), Request{Op: "access.snapshot"}) }()
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		j.server.mu.Lock()
		n := len(j.server.connections)
		j.server.mu.Unlock()
		if n >= 2 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	j.server.mu.Lock()
	overlap := len(j.server.connections) >= 2
	j.server.mu.Unlock()
	close(measured.release)
	if !overlap {
		t.Fatal("snapshot never overlapped comparison lease")
	}
	snapshot := <-snapshotDone
	snapshotMs := float64(time.Since(snapshotStart).Microseconds()) / 1000
	if snapshot.Code != foundation.OK {
		t.Fatal("status blocked", snapshot.Code)
	}
	comparison := <-done
	if comparison.Code != foundation.OK {
		t.Fatal("comparison under load", comparison.Code)
	}
	comparisonMs := float64(time.Since(at).Microseconds()) / 1000
	revokeStart := time.Now()
	if r := j.client.Call(context.Background(), Request{Op: "access.logout"}); r.Code != foundation.OK {
		t.Fatal(r.Code)
	}
	// Hold remains blocked and Close has not run: cancellation must be caused by
	// the real logout/authorization watcher, not cleanup or a successful fixture.
	for i := 0; i < 2; i++ {
		select {
		case err := <-remote.returned:
			if err != context.Canceled {
				t.Fatal("logout did not cancel remote", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("logout left worker alive")
		}
	}
	revokeMs := float64(time.Since(revokeStart).Microseconds()) / 1000
	cur, c := j.store.PPCurrent(context.Background(), b.Provenance.Company, j.device)
	if c != equipment.OK || cur.Current.ID != b.ID {
		t.Fatal("revocation replaced last good", c)
	}
	other, c := j.store.PPCurrent(context.Background(), b.Provenance.Company, second)
	if c != equipment.OK || other.Current != nil {
		t.Fatal("late result published", c)
	}
	out := map[string]any{"scope": "two owned SSH peers blocked; real IPC/SQLite, third BUSY; observer snapshot admitted during store barrier in1000x1000 comparison (controlled barrier included in timings); owner logout cancels both SSH adapters before cleanup; no late publish", "two_job_status_ms": pollMs, "observer_snapshot_ms": snapshotMs, "comparison_ms": comparisonMs, "logout_and_worker_cleanup_ms": revokeMs, "status": "PASS"}
	raw, _ := json.MarshalIndent(out, "", "  ")
	if dir := os.Getenv("L06_MEASURE_DIR"); dir != "" {
		if e := os.WriteFile(filepath.Join(dir, "two-jobs-data.json"), append(raw, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	t.Log(string(raw))
}
