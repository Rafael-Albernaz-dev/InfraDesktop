package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"testing"
	"time"
)

func TestRadioPersistentOneHandshakeTenTicks(t *testing.T) {
	api, ok := any(routerssh.Adapter{}).(equipment.RadioSessionRemote)
	for _, family := range []equipment.RadioFamily{equipment.RadioRouterOSWirelessV6, equipment.RadioAirOSV6, equipment.RadioIntelbrasWOM} {
		t.Run(string(family), func(t *testing.T) { assertPersistentRadioFamily(t, api, ok, family) })
	}
}
func TestRadioPersistentCancellationClosesInflight(t *testing.T) {
	api, ok := any(routerssh.Adapter{}).(equipment.RadioSessionRemote)
	if !ok {
		t.Fatal("missing persistent lifecycle")
	}
	hold := make(chan struct{})
	peer, e := sshfixture.Start(sshfixture.Config{Hold: hold})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	defer close(hold)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s,
		c := api.OpenRadioSession(ctx,
		equipment.Device{Role: equipment.RoleAP,
			Address:     "127.0.0.1",
			Port:        peer.Port(),
			Fingerprint: peer.Fingerprint()},
		"reader",
		sshfixture.Password,
		equipment.RadioRouterOSWirelessV6,
		&equipment.RadioLink{MAC: "AA:BB:CC:DD:EE:FF"},
		func() equipment.Code { return equipment.OK })
	if c != equipment.OK {
		t.Fatal(c)
	}
	defer s.Close()
	done := make(chan equipment.Code, 1)
	go func() { _, c := s.Initial(); done <- c }()
	until := time.Now().Add(time.Second)
	for peer.Commands.Load() == 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if peer.Commands.Load() != 1 {
		t.Fatal("no inflight exec")
	}
	cancel()
	select {
	case c := <-done:
		if c == equipment.OK {
			t.Fatal("cancelled read succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("connection survived cancellation")
	}
}

func TestRadioPersistentPinAndExpiration(t *testing.T) {
	raw, e := os.ReadFile("radioparse/testdata/l07_airos_xm_real.txt")
	if e != nil {
		t.Fatal(e)
	}
	peer, e := sshfixture.Start(sshfixture.Config{Command: func(string) (string, bool) { return string(raw), true }})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	d := equipment.Device{Role: equipment.RoleCPE, Address: "127.0.0.1", Port: peer.Port(), Fingerprint: "wrong-pin"}
	check := func() equipment.Code { return equipment.OK }
	if _,
		c := (routerssh.Adapter{}).OpenRadioSession(context.Background(),
		d,
		"reader",
		sshfixture.Password,
		equipment.RadioAirOSV6,
		nil,
		check); c != equipment.TrustChanged ||
		peer.Auth.Load() != 0 {
		t.Fatal("secret crossed changed pin", c, peer.Auth.Load())
	}
	d.Fingerprint = peer.Fingerprint()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	s, c := (routerssh.Adapter{}).OpenRadioSession(ctx, d, "reader", sshfixture.Password, equipment.RadioAirOSV6, nil, check)
	if c != equipment.OK {
		t.Fatal(c)
	}
	defer s.Close()
	if _, c = s.Initial(); c != equipment.OK {
		t.Fatal(c)
	}
	<-ctx.Done()
	if _, c = s.Tick(); c == equipment.OK {
		t.Fatal("tick after expiration")
	}
	s.Close()
	until := time.Now().Add(time.Second)
	for peer.ActiveConnections() != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if peer.ActiveConnections() != 0 {
		t.Fatal("expired connection leaked")
	}
}

func TestRadioPersistentCommandDeadlineClosesConnection(t *testing.T) {
	hold := make(chan struct{})
	defer func() {
		select {
		case <-hold:
		default:
			close(hold)
		}
	}()
	peer, e := sshfixture.Start(sshfixture.Config{Hold: hold})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	s,
		c := (routerssh.Adapter{}).OpenRadioSession(ctx,
		equipment.Device{Role: equipment.RoleCPE,
			Address:     "127.0.0.1",
			Port:        peer.Port(),
			Fingerprint: peer.Fingerprint()},
		"reader",
		sshfixture.Password,
		equipment.RadioAirOSV6,
		nil,
		func() equipment.Code { return equipment.OK })
	if c != equipment.OK {
		t.Fatal(c)
	}
	defer s.Close()
	started := time.Now()
	_, c = s.Initial()
	elapsed := time.Since(started)
	if c != equipment.RadioCodeDeadlineExceeded || elapsed < 2900*time.Millisecond || elapsed > 4*time.Second {
		t.Fatal("command3s bound lost", c, elapsed)
	}
	close(hold)
	until := time.Now().Add(time.Second)
	for peer.ActiveConnections() != 0 && time.Now().Before(until) {
		time.Sleep(time.Millisecond)
	}
	if peer.ActiveConnections() != 0 {
		t.Fatal("failed command retained connection")
	}
}
