package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"strings"
	"testing"
	"time"
)

func TestL07LiveLoadCoversFiveSecondSample(t *testing.T) {
	peer, e := sshfixture.Start(sshfixture.Config{Command: func(command string) (string, bool) {
		if command != routerssh.RadioLoadCommand {
			return "", false
		}
		// Historical fixed window is exactly five seconds. New one-shot probe
		// produces a closed frame, and repeats on the same observer connection.
		if strings.Contains(routerssh.RadioLoadCommand, ":for") {
			time.Sleep(5 * time.Second)
			return "L07-LOAD\ncpu=6\ncpu=6\ncpu=6\ncpu=6\ncpu=6\ncpu=6\nL07-LOAD-DONE\n", true
		}
		return "L07-LOAD\ncpu=6\nL07-LOAD-DONE\n", true
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer peer.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ready := make(chan struct{})
	stop := make(chan struct{})
	result := make(chan equipment.RadioLoad, 1)
	d := equipment.Device{Address: "127.0.0.1", Port: peer.Port(), Fingerprint: peer.Fingerprint()}
	go func() {
		var load equipment.RadioLoad
		var c equipment.Code
		switch p := any(routerssh.Adapter{}).(type) {
		case interface {
			ObserveRadioLoad(context.Context, equipment.Device, string, string, func() equipment.Code, func(), <-chan struct{}) (equipment.RadioLoad, equipment.Code)
		}:
			load, c = p.ObserveRadioLoad(ctx, d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK }, func() { close(ready) }, stop)
		case interface {
			ObserveRadioLoad(context.Context, equipment.Device, string, string, func() equipment.Code, func()) (equipment.RadioLoad, equipment.Code)
		}:
			load, c = p.ObserveRadioLoad(ctx, d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK }, func() { close(ready) })
		}
		if c != equipment.OK {
			t.Errorf("probe: %s", c)
		}
		result <- load
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("not ready")
	}
	start := time.Now()
	time.Sleep(5 * time.Second)
	end := time.Now()
	close(stop)
	select {
	case load := <-result:
		if load.Started.After(start) || load.Ended.Before(end) || len(load.CPU) < 3 || load.PingSent < 3 {
			t.Fatalf("probe does not surround five second sample: start=%v end=%v load=%+v", start, end, load)
		}
	case <-ctx.Done():
		t.Fatal("probe failed to finish")
	}
}
