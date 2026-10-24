package routerssh_test

import (
	"context"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
	"testing"
	"time"
)

func TestControlledSSH(t *testing.T) {
	for _, tc := range []struct {
		name, family, mode string
		want               equipment.Code
	}{
		{"v6", "v6", "", equipment.OK}, {"v7", "v7", "", equipment.OK}, {"auth", "v7", "auth", "REMOTE_AUTH_FAILED"}, {"permission", "v7", "permission", "REMOTE_PERMISSION_DENIED"}, {"malformed", "v7", "malformed", equipment.Partial}, {"partial", "v6", "partial", equipment.Partial}, {"large", "v7", "large", equipment.Limit}, {"interfaces-limit", "v7", "interfaces", equipment.OK}, {"unsupported", "v7", "unsupported", "UNSUPPORTED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, e := sshfixture.Start(sshfixture.Config{Family: tc.family, Mode: tc.mode})
			if e != nil {
				t.Fatal(e)
			}
			defer server.Close()
			d := equipment.Device{Address: "127.0.0.1", Port: server.Port()}
			adapter := routerssh.Adapter{}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			fp, c := adapter.Inspect(ctx, d)
			if c != equipment.OK || fp != server.Fingerprint() || server.Auth.Load() != 0 {
				t.Fatal("inspection authenticated or failed", c)
			}
			d.Fingerprint = fp
			o, c := adapter.Query(ctx, d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
			if c != tc.want {
				t.Fatalf("got %s want %s", c, tc.want)
			}
			if tc.want == equipment.OK {
				if o == nil || o.Family != tc.family || o.CPU != 12 || (len(o.Interfaces) != 1 && tc.mode != "interfaces") || o.Source == "" || o.At == "" || (o.Quality != "complete" && tc.mode != "interfaces") {
					t.Fatal("missing observed data")
				}
			} else if o != nil {
				t.Fatal("failed read published a result")
			}
			d.Fingerprint = "SHA256:wrong"
			before := server.Auth.Load()
			if _, c = adapter.Query(ctx, d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK }); c != equipment.TrustChanged || server.Auth.Load() != before {
				t.Fatal("changed key sent credentials", c)
			}
			if _, c = adapter.Inspect(ctx, d); c != equipment.TrustChanged {
				t.Fatal("changed pin silently relearned")
			}
		})
	}
}
func TestHandshakeDeadlineAndCancel(t *testing.T) {
	for _, mode := range []string{"handshake", "command"} {
		t.Run(mode, func(t *testing.T) {
			hold := make(chan struct{})
			cfg := sshfixture.Config{}
			if mode == "handshake" {
				cfg.Mode = mode
			} else {
				cfg.Hold = hold
			}
			server, e := sshfixture.Start(cfg)
			if e != nil {
				t.Fatal(e)
			}
			defer server.Close()
			d := equipment.Device{Address: "127.0.0.1", Port: server.Port(), Fingerprint: server.Fingerprint()}
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
			defer cancel()
			start := time.Now()
			_, c := (routerssh.Adapter{}).Query(ctx, d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
			if c != "DEADLINE_EXCEEDED" || time.Since(start) > time.Second {
				t.Fatal("deadline does not cover SSH", c)
			}
			ctx, cancel = context.WithCancel(context.Background())
			done := make(chan equipment.Code, 1)
			go func() {
				_, c := (routerssh.Adapter{}).Query(ctx, d, "reader", sshfixture.Password, func() equipment.Code { return equipment.OK })
				done <- c
			}()
			cancel()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("cancel leaked SSH")
			}
		})
	}
}
