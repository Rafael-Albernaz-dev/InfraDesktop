package routerssh_test

import (
	"github.com/google/uuid"
	e "infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type recoveryAP struct {
	scenario, rich string
	live           atomic.Bool
	ticks          atomic.Int32
	server         *sshfixture.Server
}

func newRecoveryAP(t *testing.T, scenario string) *recoveryAP {
	t.Helper()
	r := &recoveryAP{scenario: scenario, rich: recoveryGolden(t, "l07_ap_v6_real.txt")}
	var err error
	r.server, err = sshfixture.Start(sshfixture.Config{Command: r.command})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func newRecoveryCPE(t *testing.T) *sshfixture.Server {
	t.Helper()
	rich := recoveryGolden(t, "l07_airos_xm_real.txt")
	peer, err := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) { return rich, true }})
	if err != nil {
		t.Fatal(err)
	}
	return peer
}
func (r *recoveryAP) command(cmd string) (string, bool) {
	if !strings.Contains(cmd, "L07-TICK") {
		return r.rich, true
	}
	if r.live.Load() {
		i := r.ticks.Add(1)
		if defect := r.disrupt(i); defect != "" {
			return defect, true
		}
	}
	return recoveryTick, true
}
func (r *recoveryAP) disrupt(i int32) string {
	if r.scenario == "collectorDefect" && i == 1 {
		return "syntax error (line 1 column 1)\n"
	}
	if i == 1 || (r.scenario == "consecutive" && i == 2) {
		if r.scenario == "changedPin" && i == 1 {
			if err := r.server.RotateHostKey(); err != nil {
				panic(err)
			}
		}
		time.Sleep(3200 * time.Millisecond)
	}
	return ""
}
func runRecoveryScenario(t *testing.T, scenario string) {
	t.Helper()
	state := newRecoveryAP(t, scenario)
	ap := state.server
	defer ap.Close()
	cpe := newRecoveryCPE(t)
	defer cpe.Close()
	s, q, in, _ := recoveryHarness(t, ap, cpe)
	id := recoveryStart(t, s, q, in)
	measured := recoveryStatus(t, s, q, id, func(v e.RadioView) bool { return v.Job.State != "RUNNING" })
	if measured.Job.Code != e.OK || measured.Cost == nil || !measured.Cost.Enabled {
		t.Fatalf("legitimate measurement must enable live: %+v %+v", measured.Job, measured.Cost)
	}
	waitRecoveryConnections(ap, cpe)
	apBefore, authBefore := ap.Handshakes.Load(), ap.Auth.Load()
	cpeBefore := cpe.Handshakes.Load()
	state.live.Store(true)
	in.Mode = e.RadioLive
	in.Operation = uuid.NewString()
	id = recoveryStart(t, s, q, in)
	v := recoveryStatus(t, s, q, id, func(v e.RadioView) bool {
		return v.Job.State != "RUNNING" || (scenario == "isolated" && v.Cost != nil && len(v.Cost.Series) >= 3)
	})
	assertRecoveryOutcome(t, scenario, v, ap, authBefore)
	recoveryCancel(t, s, q, id)
	assertRecoveryCountsAndClosure(t, scenario, ap, cpe, apBefore, cpeBefore, v)
}
func waitRecoveryConnections(ap, cpe *sshfixture.Server) {
	until := time.Now().Add(time.Second)
	for (ap.ActiveConnections() != 0 || cpe.ActiveConnections() != 0) && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
}
func assertRecoveryOutcome(t *testing.T, scenario string, v e.RadioView, ap *sshfixture.Server, authBefore int32) {
	t.Helper()
	switch scenario {
	case "isolated", "consecutive":
		assertRecoveryTimeoutSequence(t, scenario, v)
	case "changedPin":
		if v.Job.Code != e.TrustChanged || ap.Auth.Load() != authBefore+1 {
			t.Fatalf("changed pin must fail before password: %+v auth=%d", v.Job, ap.Auth.Load()-authBefore)
		}
	case "collectorDefect":
		if v.Job.Code != e.CollectorDefect {
			t.Fatalf("collector defect is immediately fatal: %+v", v.Job)
		}
	}
}
func assertRecoveryTimeoutSequence(t *testing.T, scenario string, v e.RadioView) {
	t.Helper()
	switch scenario {
	case "isolated":
		if v.Job.State != "RUNNING" ||
			len(v.Cost.Series) < 3 ||
			!strings.Contains(v.Cost.Series[0].Reason,
				string(e.RadioCodeDeadlineExceeded)) ||
			v.Cost.Series[1].Reason != "OK" {
			t.Fatalf("one slow tick must recover before later success: %+v %+v", v.Job, v.Cost)
		}
	case "consecutive":
		if v.Job.Code != e.RadioCodeLiveAborted ||
			v.Cost == nil ||
			len(v.Cost.Series) != 2 ||
			!strings.Contains(v.Cost.Reason,
				string(e.RadioCodeDeadlineExceeded)) {
			t.Fatalf("second consecutive timeout must abort with actual reason: %+v %+v", v.Job, v.Cost)
		}
	}
}
func assertRecoveryCountsAndClosure(t *testing.T, scenario string, ap, cpe *sshfixture.Server, apBefore, cpeBefore int32, v e.RadioView) {
	t.Helper()
	want := int32(2)
	if scenario == "collectorDefect" {
		want = 1
	}
	if ap.Handshakes.Load()-apBefore != want || cpe.Handshakes.Load()-cpeBefore != 1 {
		t.Fatalf("AP handshakes=%d want=%d; healthy CPE=%d want=1", ap.Handshakes.Load()-apBefore, want, cpe.Handshakes.Load()-cpeBefore)
	}
	waitRecoveryConnections(ap, cpe)
	if ap.ActiveConnections() != 0 || cpe.ActiveConnections() != 0 {
		t.Fatal("sessions leaked after stop/abort")
	}
	t.Logf("actual peer: AP handshakes=%d (1 + reopen); CPE=1; state=%s code=%s", ap.Handshakes.Load()-apBefore, v.Job.State, v.Job.Code)
}
