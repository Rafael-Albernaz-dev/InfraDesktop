package routerssh_test

import (
	e "infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRadioReopenWithoutRichRead(t *testing.T) {
	rich := recoveryGolden(t, "l07_ap_v6_real.txt")
	cpeRich := recoveryGolden(t, "l07_airos_xm_real.txt")
	var richCalls, ticks atomic.Int32
	var failedStarted, reopenedAt atomic.Int64
	ap, err := sshfixture.Start(sshfixture.Config{OnHandshake: func() {
		if richCalls.Load() > 0 {
			reopenedAt.Store(time.Now().UnixNano())
		}
	}, Command: func(cmd string) (string, bool) {
		if strings.Contains(cmd, "L07-TICK") {
			if ticks.Add(1) == 1 {
				failedStarted.Store(time.Now().UnixNano())
				time.Sleep(3200 * time.Millisecond)
			}
			return recoveryTick, true
		}
		richCalls.Add(1)
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
	if v.Job.Code != e.OK || v.Cost == nil || len(v.Cost.Series) != 10 || v.Cost.Series[1].Reason != "OK" {
		t.Fatalf("recovery failed: %+v %+v", v.Job, v.Cost)
	}
	if richCalls.Load() != 1 || ap.Handshakes.Load() != 2 {
		t.Fatalf("rich=%d want1; handshakes=%d want2", richCalls.Load(), ap.Handshakes.Load())
	}
	if waited := time.Duration(reopenedAt.Load() - failedStarted.Load()); waited < 3900*time.Millisecond {
		t.Fatal("reopened before normal interval after failed command", waited)
	}

}

func TestRadioPartialTransportAndJob(t *testing.T) {
	for _, reason := range []string{"stderr", "exit", "markers"} {
		t.Run(reason, func(t *testing.T) {
			rich := recoveryGolden(t, "l07_ap_v6_real.txt")
			cpeRich := recoveryGolden(t, "l07_airos_xm_real.txt")
			config := sshfixture.Config{Command: func(string) (string, bool) { return rich, true }}
			if reason == "stderr" {
				config.Stderr = "synthetic private banner"
			}
			if reason == "exit" {
				config.ExitStatus = 1
			}
			if reason == "markers" {
				config.Command = func(string) (string, bool) { return "synthetic incomplete", true }
			}
			ap, err := sshfixture.Start(config)
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
			if v.Job.Code != e.Partial ||
				v.Job.Failure == nil ||
				v.Job.Failure.Reason != reason ||
				v.Job.Failure.Bytes == 0 ||
				!strings.Contains(v.Cost.Reason,
					"("+reason+")") {
				t.Fatalf("partial detail discarded: %+v %+v", v.Job, v.Cost)
			}
			if v.APCode != e.Partial {
				t.Fatal("failed side not published", v.APCode)
			}
		})
	}
}

func TestRadioReopenedLinkMismatchIsFatal(t *testing.T) {
	for _, field := range []string{"mac", "interface", "bssid"} {
		t.Run(field, func(t *testing.T) {
			rich := recoveryGolden(t, "l07_ap_v6_real.txt")
			cpeRich := recoveryGolden(t, "l07_airos_xm_real.txt")
			var ticks, richCalls atomic.Int32
			bad := recoveryTick
			switch field {
			case "mac":
				bad = strings.Replace(bad, "02:00:00:00:10:01", "AA:BB:CC:DD:EE:01", 1)
			case "interface":
				bad = strings.Replace(bad, "interface=wlan1", "interface=wlan2", 1)
			case "bssid":
				bad = strings.Replace(bad, "02:00:00:00:10:05", "64:D1:54:1D:88:30", 1)
			}
			ap, err := sshfixture.Start(sshfixture.Config{Command: func(cmd string) (string, bool) {
				if strings.Contains(cmd, "L07-TICK") {
					i := ticks.Add(1)
					if i == 1 {
						time.Sleep(3200 * time.Millisecond)
					}
					if i >= 2 {
						return bad, true
					}
					return recoveryTick, true
				}
				richCalls.Add(1)
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
			if v.Job.Code != e.RadioCodeLinkUnconfirmed || ap.Handshakes.Load() != 2 || richCalls.Load() != 1 || len(v.Cost.Series) != 2 {
				t.Fatalf("mismatch retried/partial/rich: job=%+v handshakes=%d rich=%d", v.Job, ap.Handshakes.Load(), richCalls.Load())
			}
		})
	}
}
