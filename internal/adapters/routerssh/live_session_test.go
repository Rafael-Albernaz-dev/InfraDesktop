package routerssh_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/pulse"
	"infradesktop/internal/testsupport/sshfixture"
)

func livePeer(t *testing.T, cfg sshfixture.Config) (*sshfixture.Server, equipment.Device) {
	t.Helper()
	peer, err := sshfixture.Start(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(peer.Close)
	return peer, equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
}
func TestLiveReusesConnectionAndCountsBothRoles(t *testing.T) {
	for _, role := range []equipment.Role{equipment.RoleConcentrator, equipment.RoleAP} {
		t.Run(string(role), func(t *testing.T) {
			prefix := `:put ("L\t1\t".[/system resource get uptime]."\t".[/system resource get cpu-load]."\t".`
			command := prefix + `[:len [/ppp active find where service=pppoe]])`
			if role == equipment.RoleAP {
				command = prefix + `[:len [/interface wireless registration-table find]])`
			}
			peer, d := livePeer(t, sshfixture.Config{Command: func(cmd string) (string, bool) {
				if cmd != command {
					return "invalid command", true
				}
				return "L\t1\t1d2h3m4s\t99\t1234\r\n", true
			}})
			session, fault := routerssh.OpenLive(context.Background(), d, "reader", sshfixture.Password)
			if fault != pulse.ReadOK {
				t.Fatal(fault)
			}
			defer session.Close()
			for range 3 {
				reading, fault := session.Read(context.Background(), pulse.ReadTarget{Role: role})
				if fault != pulse.ReadOK || reading.CPU != 99 || reading.Clients != 1234 || reading.Uptime != 26*time.Hour+3*time.Minute+4*time.Second {
					t.Fatal(reading, fault)
				}
			}
			if peer.Auth.Load() != 1 || peer.Handshakes.Load() != 1 || peer.Commands.Load() != 3 {
				t.Fatal("connection not reused")
			}
		})
	}
}
func TestLiveChangedKeySendsNoPassword(t *testing.T) {
	peer, d := livePeer(t, sshfixture.Config{})
	d.Fingerprint = "SHA256:untrusted"
	session, fault := routerssh.OpenLive(context.Background(), d, "reader", sshfixture.Password)
	if session != nil || fault != pulse.ReadKeyChanged || peer.Auth.Load() != 0 {
		t.Fatalf("session=%v fault=%s passwords=%d", session != nil, fault, peer.Auth.Load())
	}
}
func TestLiveRefusedPasswordIsClassified(t *testing.T) {
	peer, d := livePeer(t, sshfixture.Config{Mode: "auth"})
	session, fault := routerssh.OpenLive(context.Background(), d, "reader", sshfixture.Password)
	if session != nil || fault != pulse.ReadRefused || peer.Auth.Load() != 1 {
		t.Fatal("refusal misclassified", fault)
	}
}
func TestLiveInvalidOutputLosesConnection(t *testing.T) {
	for _, output := range []string{"L\t1\tbad\t7\t2\n", "L\t1\t1h\t101\t2\n", "L\t1\t1h\t7\t-1\n",
		"L\t1\t1h\t7\t2\nL\t1\t1h\t7\t2\n", strings.Repeat("x", 4097)} {
		t.Run(output[:min(len(output), 20)], func(t *testing.T) {
			_, d := livePeer(t, sshfixture.Config{Command: func(string) (string, bool) { return output, true }})
			session, fault := routerssh.OpenLive(context.Background(), d, "reader", sshfixture.Password)
			if fault != pulse.ReadOK {
				t.Fatal(fault)
			}
			defer session.Close()
			_, fault = session.Read(context.Background(), pulse.ReadTarget{Role: equipment.RoleConcentrator})
			if fault != pulse.ReadLost {
				t.Fatal("invalid data accepted", fault)
			}
		})
	}
}

func TestLiveAndManualReadsUseIndependentConnections(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	_, d := livePeer(t, sshfixture.Config{Family: "v6", Command: func(command string) (string, bool) {
		if !strings.HasPrefix(command, `:put ("L\t1\t"`) {
			return "", false
		}
		close(started)
		<-release
		return "L\t1\t1h\t7\t9\n", true
	}})
	session, fault := routerssh.OpenLive(context.Background(), d, "reader", sshfixture.Password)
	if fault != pulse.ReadOK {
		close(release)
		t.Fatal(fault)
	}
	defer session.Close()
	done := make(chan pulse.ReadFault, 1)
	go func() {
		_, fault := session.Read(context.Background(), pulse.ReadTarget{Role: equipment.RoleConcentrator})
		done <- fault
	}()
	<-started
	observation, code := (routerssh.Adapter{}).Query(context.Background(), d, "reader", sshfixture.Password,
		func() equipment.Code { return equipment.OK })
	close(release)
	if code != equipment.OK || observation == nil {
		t.Fatal("manual read interrupted by collector", code)
	}
	if fault := <-done; fault != pulse.ReadOK {
		t.Fatal("collector interrupted by manual read", fault)
	}
}

func TestLiveReadsRouterOSUptimeWithAnyLineEnding(t *testing.T) {
	for _, ending := range []string{"", "\n", "\r", "\r\n", "\r\n\r\n"} {
		t.Run(fmt.Sprintf("ending_%q", ending), func(t *testing.T) {
			_, d := livePeer(t, sshfixture.Config{Command: func(string) (string, bool) {
				return "L\t1\t24w3d09:30:59\t1\t6" + ending, true
			}})
			session, fault := routerssh.OpenLive(context.Background(), d, "reader", sshfixture.Password)
			if fault != pulse.ReadOK {
				t.Fatal(fault)
			}
			defer session.Close()
			reading, fault := session.Read(context.Background(), pulse.ReadTarget{Role: equipment.RoleConcentrator})
			wantUptime := (24*7+3)*24*time.Hour + 9*time.Hour + 30*time.Minute + 59*time.Second
			if fault != pulse.ReadOK || reading.CPU != 1 || reading.Clients != 6 || reading.Uptime != wantUptime {
				t.Fatalf("reading=%+v fault=%s want uptime=%s", reading, fault, wantUptime)
			}
		})
	}
}
