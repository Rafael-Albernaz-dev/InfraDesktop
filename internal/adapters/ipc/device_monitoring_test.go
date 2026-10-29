package ipc

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestL06T2OperationalScopeThroughIPC(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			j := newPPJourney(t, sshfixture.Config{Family: family, PPRows: 1, RealTime: true, Command: func(cmd string) (string, bool) {
				if cmd == routerssh.DeviceAuthFailuresCommand {
					version := "7.14.3"
					if family == "v6" {
						version = "6.49.18"
					}
					return "A\t1\t" + version + "\t2d1h1s\nF\tauth-only-canary\t01:00:00\nF\tauth-only-canary\t01:00:01\nG\tauth\t1\nZ\t2\tcomplete\t2d1h1s\n", true
				}
				return "", false
			}})
			before, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK {
				t.Fatal(c)
			}
			input := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "operations"}
			job := j.call("pppoe.read-start", input, foundation.OK).Data.PPPoE.Job
			var v *equipment.PPView
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
				if v.Job.State != "RUNNING" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			if v.Job.State != "SUCCEEDED" || v.Read == nil || len(v.Read.Rows) == 0 || v.Read.Rows[0].Health == nil || v.Read.Rows[0].Health.CPU == nil || *v.Read.Rows[0].Health.CPU != 10 || v.Read.Availability["auth"] != "partial" {
				t.Fatal("operational result missing or unknown made zero", v.Job, v.Read)
			}
			if v.Read.Rows[0].Health.FastTrack == nil || *v.Read.Rows[0].Health.FastTrack != 1 || v.Read.Availability["ports"] != "complete" {
				t.Fatal("T3 FastTrack/ports did not cross real IPC", v.Read)
			}
			insights := j.call("pppoe.monitor-current", equipment.PPInput{DeviceID: j.device, Scope: "equipment"}, foundation.OK).Data.PPPoE
			var port bool
			for _, r := range insights.Read.Rows {
				if r.Kind == "port" && r.Port != nil && r.Port.SpeedBPS != nil && *r.Port.SpeedBPS == 100000000 {
					port = true
				}
			}
			if !port {
				t.Fatal("saved port did not cross IPC", insights.Read)
			}
			auth := false
			for _, r := range v.Read.Rows {
				if r.Kind == "auth_failure" {
					auth = true
					if r.Login != "auth-only-canary" || r.Used == nil || *r.Used != 2 {
						t.Fatal("auth projection", r)
					}
				}
			}
			if !auth {
				t.Fatal("ranked auth result missing")
			}
			calls := j.peer.Commands.Load()
			j.call("pppoe.read-page", equipment.PPInput{JobID: job.ID}, foundation.OK)
			local := j.call("pppoe.monitor-current", equipment.PPInput{DeviceID: j.device, Scope: "series"}, foundation.OK).Data.PPPoE
			if len(local.Read.Rows) != 1 || local.Read.Rows[0].Point.Health == nil || local.Read.Rows[0].Point.Online != nil {
				t.Fatal("health-only point fabricated session count", local)
			}
			j.call("pppoe.monitor-current", equipment.PPInput{DeviceID: j.device, Scope: "versions"}, foundation.OK)
			if j.peer.Commands.Load() != calls {
				t.Fatal("status/page/local history started SSH")
			}
			invalid := input
			invalid.Operation = uuid.NewString()
			invalid.RowIDs = []string{uuid.NewString()}
			j.call("pppoe.read-start", invalid, foundation.Invalid)
			invalid = equipment.PPInput{DeviceID: j.device, Scope: "shell"}
			j.call("pppoe.monitor-current", invalid, foundation.Invalid)
			after, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK || len(before.Operations) != len(after.Operations) {
				t.Fatal("monitoring consumed permanent receipt")
			}
			if e := filepath.WalkDir(filepath.Join(j.root, "state"), func(path string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if !d.IsDir() {
					body, e := os.ReadFile(path)
					if e != nil {
						return e
					}
					if strings.Contains(string(body), "auth-only-canary") {
						t.Error("auth identity persisted instead of minimal series")
					}
				}
				return nil
			}); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestL06T2CursorRejectsChangedSeries(t *testing.T) {
	j := newPPJourney(t, sshfixture.Config{Family: "v7", PPRows: 1, RealTime: true})
	target, c := j.store.ReadPPTarget(context.Background(), j.client.profile, j.device, false)
	if c != equipment.OK {
		t.Fatal(c)
	}
	publish := func(at time.Time) {
		t.Helper()
		result := &equipment.PPReadResult{Scope: "operations", Ended: equipment.PPTime(at), Rows: []equipment.PPReadRow{{Kind: "resource", Health: &equipment.DeviceHealth{Version: "7.14.3"}}}}
		if c := j.store.SaveDeviceMonitoring(context.Background(), j.client.profile, target.Device, target.Credential.Metadata.Revision, result); c != equipment.OK {
			t.Fatal(c)
		}
	}
	now := time.Now().UTC()
	for i := 0; i < 17; i++ {
		publish(now.Add(time.Duration(-i) * time.Minute))
	}
	input := equipment.PPInput{DeviceID: j.device, Scope: "series"}
	first := j.call("pppoe.monitor-current", input, foundation.OK).Data.PPPoE
	if first.Next == "" {
		t.Fatal("missing next cursor")
	}
	publish(now.Add(time.Minute))
	input.Cursor = first.Next
	j.call("pppoe.monitor-current", input, foundation.Code(equipment.CursorInvalid))
}
