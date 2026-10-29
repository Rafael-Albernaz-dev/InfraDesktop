package ipc

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestL06TTrafficThroughIPCStopAndCancel(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		t.Run(family, func(t *testing.T) {
			j := newPPJourney(t, sshfixture.Config{Family: family, PPRows: 1, RealTime: true})
			o := j.collect(uuid.NewString())
			row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
			before, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK {
				t.Fatal(c)
			}
			in := equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "traffic", ObservationID: o.ID, RowIDs: []string{row.ID}}
			job := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
			replay := j.call("pppoe.read-start", in, foundation.OK).Data.PPPoE.Job
			if replay.ID != job.ID {
				t.Fatal("start replay opened another stream")
			}
			var v *equipment.PPView
			until := time.Now().Add(5 * time.Second)
			for time.Now().Before(until) {
				v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
				if v.Read != nil {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			if v.Read == nil || len(v.Read.Rows) != 1 || v.Read.Rows[0].Traffic == nil || v.Read.Rows[0].Traffic.CoveredNS <= 0 || v.Read.Rows[0].Download == nil {
				t.Fatal("traffic did not traverse IPC/service/SSH", v)
			}
			points := len(v.Read.Rows[0].Traffic.Points)
			foreignActivation := uuid.NewString()
			denied := j.client.exchange(context.Background(), Request{Op: "pppoe.read-stop", Session: j.client.session, Profile: j.client.profile, Activation: foreignActivation, ExpectedProfile: j.client.profile, ExpectedActivation: foreignActivation, PPPoE: &equipment.PPInput{JobID: job.ID}})
			if denied.Code != foundation.ProfileChanged {
				t.Fatal("foreign activation must not stop another observation", denied.Code)
			}
			stillRunning := j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
			if stillRunning.Job.State != "RUNNING" {
				t.Fatal("foreign stop changed the legitimate job", stillRunning.Job)
			}
			j.call("pppoe.read-stop", equipment.PPInput{JobID: job.ID}, foundation.OK)
			v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
			if v.Read == nil || len(v.Read.Rows[0].Traffic.Points) != points {
				t.Fatal("stop discarded the last plot")
			}
			commands := j.peer.Commands.Load()
			j.call("pppoe.read-page", equipment.PPInput{JobID: job.ID}, foundation.OK)
			if commands != j.peer.Commands.Load() {
				t.Fatal("cache/page started SSH")
			}
			j.call("pppoe.read-cancel", equipment.PPInput{JobID: job.ID}, foundation.OK)
			if v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE; v.Read != nil {
				t.Fatal("exit retained plot")
			}
			after, c := j.store.ReadEquipment(context.Background(), j.client.profile)
			if c != equipment.OK || len(before.Operations) != len(after.Operations) {
				t.Fatal("traffic mutated permanent ledger")
			}
		})
	}
}
func TestL06TTrafficSixteenWorstRowsFitRealFrame(t *testing.T) {
	n := 1e20
	rate := &equipment.PPRate{Min: n, Max: n + 100000}
	row := equipment.PPReadRow{Kind: "session", RowID: uuid.NewString(), Login: strings.Repeat("x", 256), Download: rate, Upload: rate, Traffic: &equipment.PPTraffic{DownloadBytes: "18446744073709551615", UploadBytes: "18446744073709551615", CoveredNS: 60000000000, CoveredMinNS: 50000000000, Gaps: 13, AverageDownload: rate, AverageUpload: rate, PeakDownload: rate, PeakUpload: rate}}
	for i := 0; i < 13; i++ {
		row.Traffic.Points = append(row.Traffic.Points, equipment.PPTrafficPoint{T: int64(i * 5000), D: &n, U: &n, Gap: true})
	}
	body, e := json.Marshal(row)
	if e != nil || len(body) > equipment.PPRowBytes {
		t.Fatal("traffic row exceeds unchanged budget", len(body), e)
	}
	read := &equipment.PPReadResult{Scope: "traffic", Started: "2026-10-28T00:00:00.123456789Z", Ended: "2026-10-28T00:01:00.123456789Z"}
	for i := 0; i < 16; i++ {
		read.Rows = append(read.Rows, row)
	}
	response := Response{Version: 1, RequestID: strings.Repeat("r", 128), Code: foundation.OK, Data: &Data{ContractRevision: ProductRevision, Profile: uuid.NewString(), Activation: uuid.NewString(), Authenticated: true, PPPoE: &equipment.PPView{Read: read, Total: 16, Job: &equipment.Job{ID: uuid.NewString(), State: "RUNNING", Code: equipment.OK}}}}
	body, e = json.Marshal(response)
	if e != nil || len(body) > 48*1024 {
		t.Fatal("16 traffic rows exceed actual IPC envelope", len(body), e)
	}
	t.Logf("row=%d frame=%d bytes", lenMustJSON(row), len(body))
}
func lenMustJSON(v any) int { b, _ := json.Marshal(v); return len(b) }

func TestL06TTrafficInterruptedSSHRetainsGap(t *testing.T) {
	var calls atomic.Int32
	j := newPPJourney(t, sshfixture.Config{Family: "v6", PPRows: 1, RealTime: true, DisconnectCommand: func(cmd string) bool { return strings.Contains(cmd, `:put ("B\t1\t"`) && calls.Add(1) >= 3 }})
	o := j.collect(uuid.NewString())
	row := j.call("pppoe.sessions-list", equipment.PPInput{DeviceID: j.device, ObservationID: o.ID}, foundation.OK).Data.PPPoE.Rows[0]
	job := j.call("pppoe.read-start", equipment.PPInput{DeviceID: j.device, Revision: 2, Operation: uuid.NewString(), Scope: "traffic", ObservationID: o.ID, RowIDs: []string{row.ID}}, foundation.OK).Data.PPPoE.Job
	until := time.Now().Add(7 * time.Second)
	var v *equipment.PPView
	for time.Now().Before(until) {
		v = j.call("pppoe.read-status", equipment.PPInput{JobID: job.ID}, foundation.OK).Data.PPPoE
		if v.Job.State != "RUNNING" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if v.Job.State != "FAILED" || v.Job.Code != equipment.Partial || v.Read == nil || len(v.Read.Rows) != 1 || v.Read.Rows[0].Traffic == nil || v.Read.Rows[0].Traffic.Gaps != 1 || v.Read.Rows[0].Download != nil {
		t.Fatal("interrupted stream lost explicit gap/history", v.Job, v.Read)
	}
}
