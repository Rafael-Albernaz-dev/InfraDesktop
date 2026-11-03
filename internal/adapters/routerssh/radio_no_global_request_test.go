package routerssh_test

import (
	e "infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"testing"
)

func TestRadioNoGlobalRequestTenTicks(t *testing.T) {
	rich := recoveryGolden(t, "l07_ap_v6_real.txt")
	cpeRich := recoveryGolden(t, "l07_airos_xm_real.txt")
	ap, err := sshfixture.Start(sshfixture.Config{UnimplementedGlobalRequests: true, Command: func(cmd string) (string, bool) {
		if strings.Contains(cmd, "L07-TICK") {
			return recoveryTick, true
		}
		return rich, true
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer ap.Close()
	cpe, err := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) { return cpeRich, true }})
	if err != nil {
		t.Fatal(err)
	}
	defer cpe.Close()
	s, q, in, _ := recoveryHarness(t, ap, cpe)
	id := recoveryStart(t, s, q, in)
	v := recoveryStatus(t, s, q, id, func(v e.RadioView) bool { return v.Job.State != "RUNNING" })
	if v.Job.Code != e.OK || v.Cost == nil || len(v.Cost.Series) != 10 {
		t.Fatalf("ten ticks failed: job=%+v cost=%+v global=%d", v.Job, v.Cost, ap.GlobalRequests.Load())
	}
	for i, tick := range v.Cost.Series {
		if tick.Reason != "OK" {
			t.Errorf("tick%d: %s", i+1, tick.Reason)
		}
	}
	if ap.GlobalRequests.Load() != 0 || ap.Handshakes.Load() != 1 || cpe.Handshakes.Load() != 1 {
		t.Fatalf("global=%d handshakes AP=%d CPE=%d", ap.GlobalRequests.Load(), ap.Handshakes.Load(), cpe.Handshakes.Load())
	}
}
