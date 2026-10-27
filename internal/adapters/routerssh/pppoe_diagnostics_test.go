package routerssh_test

import (
	"context"
	"encoding/json"
	"fmt"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func catalogBody(version string) string {
	return "D\t1\tcatalog\t" + version + "\t2d1h1s\t7\n" +
		"I\t*1\tether1\tether\ttrue\tfalse\tfalse\n" +
		"I\t*2\taccess\tvlan\ttrue\tfalse\tfalse\n" +
		"I\t*3\tinner\tvlan\ttrue\tfalse\tfalse\n" +
		"V\t*2\taccess\tether1\t100\ttrue\ttrue\tfalse\n" +
		"V\t*3\tinner\taccess\t200\tfalse\ttrue\tfalse\n" +
		"S\t*1\tservice-a\tinner\tfalse\tfalse\t100-115,120,122\ttrue\n" +
		"S\t*2\tservice-b\tinner\tfalse\tfalse\t?\t?\nZ\t7\tcomplete\t2d1h1s\n"
}
func radiusBody(version string) string {
	// Configuration and TSV codec are synthetic. Counter values independently
	// follow the manufacturer's RADIUS monitor example, not a RouterOS transcript.
	return "D\t1\tradius\t" + version + "\t2d1h1s\t2\nA\ttrue\ttrue\n" +
		"R\t*1\t198.51.100.20\tppp\tfalse\t1812\t1813\tudp\t0\t10\t4\t1\t15\t5\t0\t0s\nZ\t2\tcomplete\t2d1h1s\n"
}

func TestPPDiagnosticCatalogAndRadiusThroughPinnedSSH(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		version := "7.26"
		if family == "v6" {
			version = "6.49.19"
		}
		t.Run(family, func(t *testing.T) {
			peer, err := sshfixture.Start(sshfixture.Config{Command: func(command string) (string, bool) {
				switch command {
				case routerssh.PDCatalogCommand:
					return catalogBody(version), true
				case routerssh.PDRadiusCommand:
					return radiusBody(version), true
				}
				return "", false
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer peer.Close()
			d := equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
			for _, scope := range []string{equipment.PDCatalog, equipment.PDRadius} {
				o, c := (routerssh.Adapter{}).CollectPPDiagnostic(context.Background(), d, "reader", sshfixture.Password, scope, nil, func() equipment.Code { return equipment.OK })
				if c != equipment.OK || o == nil || o.Quality.Status != "complete" {
					t.Fatal(scope, o, c)
				}
				if scope == equipment.PDCatalog {
					equipment.ResolvePDRelations(o.Rows)
					if len(o.Rows) != 7 || o.Rows[4].Relation != "resolved" || o.Rows[5].ParentID == "" || o.Rows[6].AcceptUntagged != nil {
						t.Fatal(o.Rows)
					}
				}
				if scope == equipment.PDRadius {
					if len(o.Rows) != 2 || o.Rows[1].Counters == nil || o.Rows[1].Counters.Rejects != "1" || !o.Rows[1].Counters.LastRTT.Known {
						t.Fatal(o.Rows)
					}
				}
			}
			if peer.Auth.Load() != 2 || peer.Commands.Load() != 2 {
				t.Fatal("scope refresh amplified", peer.Auth.Load(), peer.Commands.Load())
			}
		})
	}
}

func TestPPDiagnosticCommandsNeverRequestSensitiveOrWholeObjects(t *testing.T) {
	for _, command := range []string{routerssh.PDCatalogCommand, routerssh.PDRadiusCommand} {
		for _, forbidden := range []string{" secret", " password", "/log", "reset-counters", "print detail", ":put $stats", " get $i]", "/ppp secret", "sensitive", "debug"} {
			if strings.Contains(command, forbidden) {
				t.Fatalf("forbidden projection %q", forbidden)
			}
		}
		if !strings.Contains(command, "458752") || !strings.Contains(command, ":local eligible") {
			t.Fatal("missing pre-collection limits")
		}
	}
	if !strings.Contains(routerssh.PDRadiusCommand, "once as-value") || !strings.Contains(routerssh.PDRadiusCommand, `($stats->"rejects")`) {
		t.Fatal("missing narrow monitor projection")
	}
	// An unexpected field/canary from a malformed peer is rejected, never retained.
	bad := strings.Replace(radiusBody("7.26"), "Z\t2", "secret\tPRIVATE-CANARY\nZ\t2", 1)
	o, c := routerssh.ParsePPDiagnostic(equipment.PDRadius, bad)
	encoded, _ := json.Marshal(o)
	if c != equipment.OK || o.Quality.Status != "partial" || len(o.Rows) != 0 || strings.Contains(string(encoded), "PRIVATE-CANARY") {
		t.Fatal("unexpected raw field leaked", o, c)
	}
}

func TestPPDiagnosticParserLimitsFailuresAndRanges(t *testing.T) {
	for _, services := range []string{"", "ppp;hotspot"} {
		body := strings.Replace(radiusBody("7.26"), "\tppp\tfalse", "\t"+services+"\ttrue", 1)
		o, c := routerssh.ParsePPDiagnostic(equipment.PDRadius, body)
		if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != 2 || o.Rows[1].Services != strings.ReplaceAll(services, ";", ",") {
			t.Fatal("valid empty/explicit service set rejected", services, o, c)
		}
	}
	for _, services := range []string{"unknown-canary", "ppp,unknown", "ppp,", "+ppp"} {
		body := strings.Replace(radiusBody("7.26"), "\tppp\tfalse", "\t"+services+"\ttrue", 1)
		o, c := routerssh.ParsePPDiagnostic(equipment.PDRadius, body)
		if c != equipment.OK || o.Quality.Status != "partial" || len(o.Rows) != 0 {
			t.Fatal("unknown radius service accepted", o, c)
		}
	}
	for _, tc := range []struct{ body, status string }{
		{strings.Replace(catalogBody("7.26"), "Z\t7\tcomplete", "Z\t7\tchurn", 1), "partial"},
		{strings.Split(catalogBody("7.26"), "Z\t")[0], "partial"},
		{"D\t1\tcatalog\t7.26\t2d1h1s\t2049\nZ\t0\trows\t2d1h1s\n", "limited"},
		{strings.Replace(catalogBody("7.26"), "100-115,120,122", "100-1", 1), "partial"},
	} {
		o, c := routerssh.ParsePPDiagnostic(equipment.PDCatalog, tc.body)
		if c != equipment.OK || o.Quality.Status != tc.status || len(o.Rows) != 0 {
			t.Fatal(o, c)
		}
	}
	for _, tc := range []struct {
		value, want string
		valid       bool
	}{{"120,100-115;114-119", "100-120", true}, {"?", "?", true}, {"", "", true}, {"0", "", false}, {"4095", "", false}, {"1,+2", "", false}, {"1-2-3", "", false}, {"2-1", "", false}, {"1,", "", false}} {
		got, ok := routerssh.ParsePDRange(tc.value)
		if ok != tc.valid || ok && got != tc.want {
			t.Fatal(tc, got, ok)
		}
	}
	var body strings.Builder
	fmt.Fprintf(&body, "D\t1\tcatalog\t7.26\t2d1h1s\t%d\n", equipment.PPMaxRows)
	for i := 0; i < equipment.PPMaxRows; i++ {
		fmt.Fprintf(&body, "I\t*%x\tdynamic-%d\tpppoe-in\ttrue\tfalse\ttrue\n", i+1, i)
	}
	fmt.Fprintf(&body, "Z\t%d\tcomplete\t2d1h1s\n", equipment.PPMaxRows)
	o, c := routerssh.ParsePPDiagnostic(equipment.PDCatalog, body.String())
	if c != equipment.OK || o.Quality.Status != "complete" || len(o.Rows) != equipment.PPMaxRows {
		t.Fatal("dynamic catalog incorrectly limited to128", c, o.Quality)
	}
}

func pingTarget(family string) *equipment.PPPingTarget {
	version := "7.26"
	if family == "v6" {
		version = "6.49.19"
	}
	o, _ := routerssh.ParsePPPoE(pingFresh(version, "2d1h1s", "1h1s", "198.51.100.1", "1"))
	return &equipment.PPPingTarget{Observation: *o, Row: o.Rows[0], Age: equipment.PPInterval{Known: true, Min: 0, Max: int64(100 * time.Millisecond)}, Prepared: time.Now()}
}
func pingFresh(version, up, session, address, sid string) string {
	return "H\t1\t1\t" + version + "\t" + up + "\nR\t*1\t" + sid + "\treader\t" + address + "\tcaller\t" + session + "\nZ\t1\tcomplete\t" + up + "\n"
}
func pingReplies(address string) string {
	// Derived synthetic3-probe peer response, not an independent firmware capture.
	return "SEQ HOST SIZE TTL TIME STATUS\n0 " + address + " 64 64 1ms45us\n1 " + address + " 64 64 1ms89us\n2 " + address + " 64 64 1ms163us\nsent=3 received=3 packet-loss=0% min-rtt=1ms45us avg-rtt=1ms99us max-rtt=1ms163us\n"
}

func TestPPDiagnosticPingContextAndIdentityBeforeAnyProbe(t *testing.T) {
	for _, family := range []string{"v6", "v7"} {
		for _, scenario := range []string{"valid", "custom-vrf", "unknown-menu", "address-reused", "session-reused", "duplicate-address", "other-ppp-service-conflict", "reboot", "unknown-uptime", "revoked-before-probe", "permission"} {
			t.Run(family+"/"+scenario, func(t *testing.T) {
				target := pingTarget(family)
				expected, ok := routerssh.BuildPDPingCommand(target)
				if !ok {
					t.Fatal("valid target refused")
				}
				if family == "v7" && !strings.Contains(expected, "vrf=main") || family == "v6" && !strings.Contains(expected, "routing-table=main") {
					t.Fatal("wrong family template")
				}
				// On ROS6, /tool ping abbreviates ping-speed, which rejects
				// count. Its ICMP tool is /ping. Keep ROS7's proven path too.
				pingCommand := "/tool ping address="
				if family == "v6" {
					pingCommand = "/ping address="
				}
				if !strings.Contains(expected, "else={"+pingCommand+"198.51.100.1 count=3 interval=200ms size=64 ttl=64 ") {
					t.Fatal("wrong ICMP command: v6 must use /ping, not the ping-speed abbreviation")
				}
				if !strings.Contains(expected, "[/ppp active print count-only where address=198.51.100.1]!=1") || strings.Contains(expected, "count-only where service=pppoe and address=") {
					t.Fatal("address uniqueness restricted to only PPPoE")
				}
				var probes atomic.Int32
				var revoked atomic.Bool
				peer, err := sshfixture.Start(sshfixture.Config{Command: func(command string) (string, bool) {
					switch command {
					case routerssh.PDPingContextV6, routerssh.PDPingContextV7:
						if scenario == "custom-vrf" {
							return "C\t" + family + "\t1\t0\t0\n", true
						}
						if scenario == "unknown-menu" {
							return "failure: bad command name vrf\n", true
						}
						if scenario == "permission" {
							return "failure: not enough permissions\n", true
						}
						if scenario == "revoked-before-probe" {
							revoked.Store(true)
						}
						return "C\t" + family + "\t0\t0\t0\n", true
					case routerssh.PPPoECommand:
						t.Error("ping reread global PPP table")
						return "", false
					default:
						if strings.Contains(command, pingCommand) {
							if !strings.Contains(command, "get *1 name") || !strings.Contains(command, "$ru>") || !strings.Contains(command, "$su>") || !strings.Contains(command, "disabled!=yes and invalid!=yes") {
								t.Error("final identity/context guard missing")
							}
							if scenario != "valid" {
								return "E\tsession\n", true
							}
							probes.Add(3)
							return pingReplies("198.51.100.1"), true
						}
					}
					return "", false
				}})
				if err != nil {
					t.Fatal(err)
				}
				defer peer.Close()
				d := equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
				result, c := (routerssh.Adapter{}).CollectPPDiagnostic(context.Background(), d, "reader", sshfixture.Password, equipment.PDPing, target, func() equipment.Code {
					if revoked.Load() {
						return equipment.Changed
					}
					return equipment.OK
				})
				if scenario == "valid" {
					if c != equipment.OK || result == nil || result.Rows[0].Ping.Received != 3 || probes.Load() != 3 {
						t.Fatal("positive did not reach exactly one finite requested run", result, c, probes.Load())
					}
				} else if c == equipment.OK || probes.Load() != 0 {
					t.Fatal("negative reached probe", scenario, c, probes.Load())
				}
			})
		}
	}
}

func TestPPDiagnosticPingParserIndependentExampleAndFailures(t *testing.T) {
	// Independent published RouterOS7.26 example, https://manual.mikrotik.com/docs/diagnostics-monitoring-and-troubleshooting/ping/
	// Only numeric response structure is transcribed; this test does not exercise RouterOS syntax.
	body := "SEQ HOST SIZE TTL TIME STATUS\n0 1.1.1.1 56 60 1ms45us\n1 1.1.1.1 56 60 1ms89us\n2 1.1.1.1 56 60 1ms163us\nsent=3 received=3 packet-loss=0% min-rtt=1ms45us avg-rtt=1ms99us max-rtt=1ms163us\n"
	o, c := routerssh.ParsePDPing(body, 3, "1.1.1.1")
	if c != equipment.OK || o.Min == nil || *o.Min != 1045000 || o.Received != 3 {
		t.Fatal(o, c)
	}
	wrapped := strings.Replace(body, " avg-rtt=", "\r\n    avg-rtt=", 1)
	o, c = routerssh.ParsePDPing("0\r\n0\r\n0\r\n1\r\n"+wrapped, 3, "1.1.1.1")
	if c != equipment.OK || o.Min == nil || *o.Min != 1045000 || o.Received != 3 {
		t.Fatal("wrapped finite summary rejected", o, c)
	}
	for _, extra := range []string{"received=3", "unknown=0", "max-rtt=", "0 1.1.1.1 56 60 1ms"} {
		if _, c := routerssh.ParsePDPing(wrapped+extra+"\n", 3, "1.1.1.1"); c == equipment.OK {
			t.Fatal("invalid summary continuation accepted", extra)
		}
	}
	timeouts := "SEQ HOST SIZE TTL TIME STATUS\n0 198.51.100.1 timeout\n1 198.51.100.1 timeout\n2 198.51.100.1 timeout\nsent=3 received=0 packet-loss=100%\n"
	o, c = routerssh.ParsePDPing(timeouts, 3, "198.51.100.1")
	if c != equipment.OK || o.Received != 0 || o.Min != nil || o.Loss != 100 {
		t.Fatal("no reply fabricated RTT/offline", o, c)
	}
	// ROS6.49.18 reply supplied from the authorized pilot; only the IP is
	// replaced. Its integer loss is rounded down, not a malformed result.
	v6 := "SEQ HOST SIZE TTL TIME STATUS\n0 198.51.100.1 64 64 1ms\n1 198.51.100.1 timeout\n2 198.51.100.1 64 64 1ms\nsent=3 received=2 packet-loss=33% min-rtt=1ms avg-rtt=1ms max-rtt=1ms\n"
	o, c = routerssh.ParsePDPing(v6, 3, "198.51.100.1")
	if c != equipment.OK || o.Sent != 3 || o.Received != 2 || o.Loss < 33 || o.Loss > 34 || o.Min == nil || *o.Min != int64(time.Millisecond) || len(o.Probes) != 3 || o.Probes[1].Status != "timeout" || o.Probes[1].RTT != nil {
		t.Fatal("valid v6 partial loss rejected or timeout fabricated RTT", o, c)
	}
	for _, bad := range []string{strings.Replace(body, "received=3", "received=2", 1), strings.Replace(body, "packet-loss=0%", "packet-loss=99%", 1), strings.Replace(body, "1 1.1.1.1", "0 1.1.1.1", 1), strings.Split(body, "sent=")[0], body + "extra\n", strings.Replace(body, "1ms45us", "+1ms", 1)} {
		if _, c := routerssh.ParsePDPing(bad, 3, "1.1.1.1"); c == equipment.OK {
			t.Fatal("malformed run accepted", bad)
		}
	}
	for _, value := range []string{"1ms1ms", "1us1ms", "+1ms", "-0s", "1ms+1us", "999999999999999999s", "1.0000000001s"} {
		if routerssh.ParsePDRTTInterval(value).Known {
			t.Fatal("unsafe RTT", value)
		}
	}
	for _, value := range []string{"0ms", "1ms45us", "1.5ms", "00:00:00.001", "1ns"} {
		if !routerssh.ParsePDRTTInterval(value).Known {
			t.Fatal("valid RTT", value)
		}
	}
}

func TestPPDiagnosticPingDeadlineCancelAndByteBudget(t *testing.T) {
	for _, scenario := range []string{"deadline", "cancel", "bytes"} {
		t.Run(scenario, func(t *testing.T) {
			target := pingTarget("v7")
			_, _ = routerssh.BuildPDPingCommand(target)
			entered, release := make(chan struct{}), make(chan struct{})
			released := false
			var attempts atomic.Int32
			peer, err := sshfixture.Start(sshfixture.Config{Command: func(value string) (string, bool) {
				switch value {
				case routerssh.PDPingContextV7:
					return "C\tv7\t0\t0\t0\n", true
				case routerssh.PPPoECommand:
					return pingFresh("7.26", "2d1h1s", "1h1s", "198.51.100.1", "1"), true
				}
				if strings.Contains(value, "/tool ping address=") {
					attempts.Add(1)
					close(entered)
					if scenario == "bytes" {
						return strings.Repeat("x", 8193), true
					}
					<-release
					return pingReplies("198.51.100.1"), true
				}
				return "", false
			}})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if !released {
					close(release)
				}
				peer.Close()
			}()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			completed := make(chan equipment.Code, 1)
			go func() {
				_, code := (routerssh.Adapter{}).CollectPPDiagnostic(ctx, equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}, "reader", sshfixture.Password, equipment.PDPing, target, func() equipment.Code { return equipment.OK })
				completed <- code
			}()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("probe barrier not reached")
			}
			started := time.Now()
			if scenario == "cancel" {
				cancel()
			}
			var code equipment.Code
			select {
			case code = <-completed:
			case <-time.After(equipment.PDPingBudget + time.Second):
				t.Fatal("finite ping did not finish/close its SSH resources")
			}
			if scenario == "bytes" {
				if code != equipment.Limit {
					t.Fatal("8KiB sublimit not enforced", code)
				}
			} else if code != "DEADLINE_EXCEEDED" {
				t.Fatal("deadline/cancel did not abort SSH run", scenario, code)
			}
			if scenario == "deadline" && time.Since(started) < equipment.PDPingBudget-100*time.Millisecond {
				t.Fatal("deadline returned before actual5s subdeadline")
			}
			if attempts.Load() != 1 {
				t.Fatal("uncertain ping was retried", attempts.Load())
			}
			close(release)
			released = true
		})
	}
	// Unsafe interpolation cannot produce a remote command.
	for _, identity := range []string{`1";:error "injected`, "token$other", "token\nline"} {
		target := pingTarget("v7")
		target.Row.SessionID = identity
		if _, ok := routerssh.BuildPDPingCommand(target); ok {
			t.Fatal("unsafe session token interpolated")
		}
	}
}
