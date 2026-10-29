package ipc

import (
	"github.com/google/uuid"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
	"time"
)

func TestL06V7CollectorFailureThroughRealClient(t *testing.T) {
	for _, tc := range []struct{ name, scope, output, code, reason string }{
		{"operations syntax", "operations", "syntax error (line 1 column 1304)\r\n", "COLLECTOR_DEFECT", "O:syntax"},
		{"auth syntax", "auth", "expected end of command (line 1 column 1470)\r\n", "COLLECTOR_DEFECT", "A:syntax"},
		{"pool echo", "operations", "O\t1\t7.14.3\t2d1h1s\n9\nP\tlab\t198.51.100.1-198.51.100.10\t8\nG\tpools\tcomplete\nZ\t1\tcomplete\t2d1h1s\n", "PARTIAL", "P:echo-mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := newPPJourney(t, sshfixture.Config{Family: "v7", PPRows: 1, RealTime: true, Command: func(cmd string) (string, bool) {
				if (tc.scope == "operations" && cmd == routerssh.DeviceOperationalCommand) || (tc.scope == "auth" && cmd == routerssh.DeviceAuthFailuresCommand) {
					return tc.output, true
				}
				return "", false
			}})
			job := j.call("pppoe.read-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "operations"}, foundation.OK).Data.PPPoE.Job
			var r Response
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				r = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK)
				if r.Data.PPPoE.Job.State != "RUNNING" {
					break
				}
				time.Sleep(time.Millisecond)
			}
			v := r.Data.PPPoE
			if string(v.Job.Code) != tc.code || v.Read != nil || v.Job.State != "FAILED" {
				t.Fatalf("collector defect or partial not preserved: %+v read=%v", v.Job, v.Read != nil)
			}
			if v.Job.PPPoE == nil || v.Job.PPPoE.Quality.Reason != tc.reason || r.ErrorDetail == nil || r.ErrorDetail.Reason != tc.reason {
				t.Fatal("closed reason missing across async real IPC client", v.Job.PPPoE, r.ErrorDetail)
			}
		})
	}
}
