package ipc

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"testing"
	"time"
)

func TestL06DContextOmissionAcrossIPCAndExport(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{PPRows: 2, Command: func(command string) (string, bool) {
		return strings.Repeat("x", routerssh.MaxOutput+1), command == routerssh.PPContextCommand
	}})
	o := j.collect(uuid.NewString())
	if o == nil || !o.ContextOmitted || o.Count != 2 || o.Quality.Status != "complete" {
		t.Fatal("optional limit crossed IPC as failed/unknown base", o)
	}
	page := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE
	if page.Current == nil || !page.Current.ContextOmitted || len(page.Rows) != 2 || page.Rows[0].State != "ONLINE" || page.Rows[0].Context != nil {
		t.Fatal("base state/marker lost on page")
	}
	groups := j.call("pppoe.context-counts", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE
	if groups.Current == nil || !groups.Current.ContextOmitted || len(groups.Groups) != 0 {
		t.Fatal("context omission became known grouping")
	}
	commands := j.peer.Commands.Load()
	j.call("pppoe.reference-save", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID, ReferenceID: uuid.NewString(), Operation: uuid.NewString()}, foundation.OK)
	ex := j.call("pppoe.export-page", equipment.PPInput{DeviceID: j.device, TargetID: o.ID, ExportKey: "CQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQkJCQk"}, foundation.OK).Data.PPPoE
	if len(ex.ExportRows) != 2 || ex.ExportRows[0].State != "ONLINE" {
		t.Fatal("sanitized base rows lost after omission")
	}
	b, _ := json.Marshal(ex.ExportRows)
	if strings.Contains(string(b), "synthetic-") || strings.Contains(string(b), "context") || j.peer.Commands.Load() != commands {
		t.Fatal("sanitized allowlist/local reads changed")
	}
}

func TestL06DBandwidthThroughRealIPCAndSSH(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{PPRows: 1, RealTime: true})
	o := j.collect(uuid.NewString())
	page := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE
	before, c := j.store.ReadEquipment(context.Background(), j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "bandwidth", ObservationID: o.ID, RowIDs: []string{page.Rows[0].ID}}
	job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
	replay := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
	if job.ID != replay.ID {
		t.Fatal("active read replay launched twice")
	}
	j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}, foundation.Busy)
	deadline := time.Now().Add(5 * time.Second)
	var result *equipment.PPView
	for time.Now().Before(deadline) {
		result = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
		if result.Job.State != "RUNNING" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if result.Job.State != "SUCCEEDED" || result.Read == nil || len(result.Read.Rows) != 1 {
		t.Fatal("finite read did not complete", result.Job)
	}
	row := result.Read.Rows[0]
	if row.Download == nil || row.Upload == nil || row.Download.Min <= row.Upload.Min || result.Read.Duration < int64(time.Second) {
		t.Fatal("rate direction/windows lost in IPC", row)
	}
	after, c := j.store.ReadEquipment(context.Background(), j.client.profile)
	if c != equipment.OK || len(after.Operations) != len(before.Operations) {
		t.Fatal("sample consumed permanent mutation receipt")
	}
	commands := j.peer.Commands.Load()
	j.call("pppoe.read-page", equipment.PPInput{JobID: job.ID}, foundation.OK)
	if j.peer.Commands.Load() != commands {
		t.Fatal("reading result triggered SSH")
	}
	invalid := in
	invalid.Operation = uuid.NewString()
	invalid.RowIDs = []string{page.Rows[0].ID, page.Rows[0].ID}
	j.call("pppoe.read-start", invalid, foundation.Invalid)
	if j.peer.Commands.Load() != commands {
		t.Fatal("invalid rows reached SSH")
	}
}

func TestL06DReadCancelBlocksLatePublication(t *testing.T) {
	hold := make(chan struct{}, 2)
	hold <- struct{}{}
	hold <- struct{}{}
	released := false
	defer func() {
		if !released {
			close(hold)
		}
	}()
	j := newPPJourney(t, sshfixture.Config{PPRows: 1, RealTime: true, Hold: hold})
	o := j.collect(uuid.NewString())
	row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "bandwidth", ObservationID: o.ID, RowIDs: []string{row.ID}}
	job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
	deadline := time.Now().Add(3 * time.Second)
	for j.peer.Commands.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if j.peer.Commands.Load() != 3 {
		t.Fatal("read did not reach owned SSH barrier")
	}
	result := j.call("pppoe.read-cancel", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
	if result.Job.State != "CANCELLED" || result.Read != nil {
		t.Fatal("cancel did not clear result", result.Job)
	}
	close(hold)
	released = true
	// Admission of another job proves worker AND watcher released the same slot.
	var next *equipment.Job
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		r := j.client.Call(context.Background(), Request{Op: "pppoe.collect-start", ExpectedProfile: j.client.profile, ExpectedActivation: j.client.activation, PPPoE: &equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}})
		if r.Code == foundation.OK {
			next = r.Data.PPPoE.Job
			break
		}
		if r.Code != foundation.Busy {
			t.Fatal(r.Code)
		}
		time.Sleep(time.Millisecond)
	}
	if next == nil {
		t.Fatal("cancel leaked endpoint slot")
	}
	result = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
	if result.Read != nil || result.Job.Code != equipment.Cancelled {
		t.Fatal("cancelled worker published late")
	}
}

func TestL06DLiveFiniteWindow(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{PPRows: 1, RealTime: true})
	o := j.collect(uuid.NewString())
	row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "live", ObservationID: o.ID, RowIDs: []string{row.ID}}
	before, c := j.store.ReadEquipment(context.Background(), j.client.profile)
	if c != equipment.OK {
		t.Fatal(c)
	}
	started := time.Now()
	job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
	j.call("pppoe.collect-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString()}, foundation.Busy)
	var final *equipment.PPView
	seen := map[string]bool{}
	deadline := time.Now().Add(63 * time.Second)
	for time.Now().Before(deadline) {
		final = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
		if final.Read != nil {
			if len(final.Read.Rows) != 1 {
				t.Fatal("live accumulated historical rows")
			}
			seen[final.Read.Ended] = true
		}
		if final.Job.State != "RUNNING" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if final == nil || final.Job.State != "SUCCEEDED" || final.Read == nil || len(seen) < 2 {
		t.Fatal("live failed finite repeated reading", final.Job)
	}
	if final.Read.Duration > int64(60*time.Second) {
		t.Fatal("live exceeded60s", final.Read.Duration)
	}
	after, c := j.store.ReadEquipment(context.Background(), j.client.profile)
	if c != equipment.OK || len(after.Operations) != len(before.Operations) {
		t.Fatal("live sample wrote permanent receipt")
	}
	t.Logf("synthetic live elapsed=%s observed_batches=%d ssh_commands=%d last_result_bytes_bounded=%v", time.Since(started), len(seen), j.peer.Commands.Load(), equipment.PPReadWithinBudget(final.Read))
}

func TestL06DLiveCancelByExactOperation(t *testing.T) {
	hold := make(chan struct{}, 2)
	hold <- struct{}{}
	hold <- struct{}{}
	released := false
	defer func() {
		if !released {
			close(hold)
		}
	}()
	j := newPPJourney(t, sshfixture.Config{PPRows: 1, RealTime: true, Hold: hold})
	o := j.collect(uuid.NewString())
	row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "live", ObservationID: o.ID, RowIDs: []string{row.ID}}
	job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
	j.call("pppoe.read-cancel", equipment.PPInput{DeviceID: j.device, Operation: uuid.NewString()}, foundation.OK)
	status := j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
	if status.Job.State != "RUNNING" {
		t.Fatal("different operation cancelled live")
	}
	j.call("pppoe.read-cancel", equipment.PPInput{DeviceID: uuid.NewString(), Operation: in.Operation}, foundation.OK)
	status = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
	if status.Job.State != "RUNNING" {
		t.Fatal("different device cancelled live")
	}
	cancelled := j.call("pppoe.read-cancel", equipment.PPInput{DeviceID: j.device, Operation: in.Operation}, foundation.OK).Data.PPPoE
	if cancelled.Job == nil || cancelled.Job.State != "CANCELLED" || cancelled.Read != nil {
		t.Fatal("operation cancel failed")
	}
	close(hold)
	released = true
}

func TestL06DTopConsumptionIsFreshMeasuredAndPaged(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{PPRows: 17, RealTime: true})
	// No stored session reading is required: top is a fresh deliberate read.
	in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "top"}
	job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
	deadline := time.Now().Add(5 * time.Second)
	var result *equipment.PPView
	for time.Now().Before(deadline) {
		result = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
		if result.Job.State != "RUNNING" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if result.Job.State != "SUCCEEDED" || result.Read == nil || len(result.Read.Rows) != 16 || result.Total != 17 || result.Next == "" {
		t.Fatal("top result not bounded/paged", result.Job, result.Total)
	}
	if result.Read.Rows[0].Login != "synthetic-0008" || result.Read.Rows[0].Download == nil {
		t.Fatal("top did not order fresh counters", result.Read.Rows[0])
	}
	if result.Read.Cost.Commands != 2 || result.Read.Cost.Bytes <= 0 || result.Read.Cost.FirstCount != 17 || result.Read.Duration < int64(time.Second) {
		t.Fatal("top cost not measured", result.Read.Cost)
	}
	before := j.peer.Commands.Load()
	last := j.call("pppoe.read-page", equipment.PPInput{JobID: job.ID, Cursor: result.Next}, foundation.OK).Data.PPPoE
	if len(last.Read.Rows) != 1 || last.Next != "" || j.peer.Commands.Load() != before {
		t.Fatal("top page resampled or mixed result")
	}
	invalid := in
	invalid.Operation = uuid.NewString()
	invalid.RowIDs = []string{uuid.NewString()}
	j.call("pppoe.read-start", invalid, foundation.Invalid)
	if j.peer.Commands.Load() != before {
		t.Fatal("top accepted selected rows from UI")
	}
}

func TestL06DLogIsSelectedBoundedAndNotRawThroughIPC(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			j := newPPJourney(t, sshfixture.Config{Family: family, PPRows: 1, RealTime: true})
			o := j.collect(uuid.NewString())
			row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
			in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "log", ObservationID: o.ID, RowIDs: []string{row.ID}}
			job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
			var result *equipment.PPView
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				result = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
				if result.Job.State != "RUNNING" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if result.Job.State != "SUCCEEDED" || result.Read == nil || len(result.Read.Rows) != 2 || result.Read.Rows[0].Event != "auth_failed" {
				t.Fatal("log path failed", result.Job)
			}
			raw, _ := json.Marshal(result.Read)
			if strings.Contains(string(raw), "message") || strings.Contains(string(raw), "password") || result.Read.Rows[0].Login != "" {
				t.Fatal("raw log field or redundant identity crossed IPC")
			}
			count := j.peer.Commands.Load()
			invalid := in
			invalid.Operation = uuid.NewString()
			invalid.RowIDs = append(invalid.RowIDs, uuid.NewString())
			j.call("pppoe.read-start", invalid, foundation.Invalid)
			if count != j.peer.Commands.Load() {
				t.Fatal("log accepted multiple subjects")
			}
		})
	}
}

func TestL06DHealthWithoutObservationThroughIPC(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			j := newPPJourney(t, sshfixture.Config{Family: family, RealTime: true})
			before, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK {
				t.Fatal(c)
			}
			job := j.call("pppoe.read-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "health"}, foundation.OK).Data.PPPoE.Job
			var v *equipment.PPView
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
				if v.Job.State != "RUNNING" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if v.Job.State != "SUCCEEDED" || v.Read == nil || len(v.Read.Rows) != 2 {
				t.Fatal("finite health did not complete", v.Job)
			}
			r := v.Read
			p, i := r.Rows[0], r.Rows[1]
			if r.PoolsAvailable == nil || !*r.PoolsAvailable || r.UplinksAvailable == nil || !*r.UplinksAvailable || r.PoolTotal != 1 || r.UplinkTotal != 1 || p.Used == nil || *p.Used != 0 || p.Size == nil || *p.Size != 3 {
				t.Fatal("pool availability/count lost", r)
			}
			if i.TX == nil || i.RX == nil || i.TX.Min <= i.RX.Min || i.RXErrors == nil || *i.RXErrors != "0" || i.TXErrors != nil || r.Cost.Commands != 2 {
				t.Fatal("static direction or unknown counters lost", i)
			}
			after, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK || len(after.Operations) != len(before.Operations) {
				t.Fatal("ephemeral health wrote permanent receipts")
			}
			calls := j.peer.Commands.Load()
			j.call("pppoe.read-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "health", ObservationID: uuid.NewString()}, foundation.Invalid)
			if calls != j.peer.Commands.Load() {
				t.Fatal("invalid health reached network")
			}
		})
	}
}

func TestL06DEmptyReadsKeepJSONArrayThroughIPC(t *testing.T) {
	for _, scope := range []string{"top", "health", "log"} {
		t.Run(scope, func(t *testing.T) {
			j := newPPJourney(t, sshfixture.Config{PPRows: 1, RealTime: true, Command: func(command string) (string, bool) {
				if strings.Contains(command, `:put ("B\t1\t"`) {
					return "B\t1\t7.26\t2d1h1s\nZ\t0\tcomplete\t2d1h1s\n", true
				}
				if strings.Contains(command, `:put ("N\t1\t"`) {
					return "N\t1\t7.26\t2d1h1s\tfalse\tfalse\nZ\t0\tcomplete\t2d1h1s\n", true
				}
				if strings.Contains(command, `:put ("L\t1\t"`) {
					return "L\t1\t7.26\t2d1h1s\t0\nZ\t0\tcomplete\t2d1h1s\n", true
				}
				return "", false
			}})
			in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: scope}
			if scope == "log" {
				o := j.collect(uuid.NewString())
				page := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE
				in.ObservationID = o.ID
				in.RowIDs = []string{page.Rows[0].ID}
			}
			job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
			var v *equipment.PPView
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
				if v.Job.State != "RUNNING" {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if v.Job.State != "SUCCEEDED" || v.Read == nil {
				t.Fatal("empty complete result missing", v.Job)
			}
			if v.Read.Rows == nil {
				t.Fatal("empty IPC rows are null; UI length/filter requires JSON array")
			}
			if len(v.Read.Rows) != 0 {
				t.Fatal("empty result invented rows")
			}
		})
	}
}
