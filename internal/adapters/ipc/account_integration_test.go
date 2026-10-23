package ipc

import (
	"bufio"
	"context"
	"fmt"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/foundation"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestIntegrationAccountClient(t *testing.T) {
	root, e := os.MkdirTemp("", "id-bf01-")
	if e != nil {
		t.Fatal(e)
	}
	defer os.RemoveAll(root)
	dirs := map[string]string{}
	for _, n := range []string{"runtime", "state", "client", "other", "password"} {
		dirs[n] = filepath.Join(root, n)
		if e = os.Mkdir(dirs[n], 0700); e != nil {
			t.Fatal(e)
		}
	}
	st, e := storage.Open(dirs["state"])
	if e != nil {
		t.Fatal(e)
	}
	svc := access.New(st, security.Crypto{}, time.Now, uint32(os.Getuid()), "activation")
	server, e := ListenAccess(dirs["runtime"], uint32(os.Getuid()), svc, foundation.Status{Instance: "instance"}, st.Close)
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	defer func() {
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	c := NewAccountClient(dirs["runtime"], dirs["client"], uint32(os.Getuid()))
	call := func(client *AccountClient, q Request, want foundation.Code) Response {
		t.Helper()
		r := client.Call(context.Background(), q)
		if r.Code != want {
			t.Fatalf("%s got %s want %s", q.Op, r.Code, want)
		}
		return r
	}
	call(c, Request{Op: "company.create", Name: "preauth"}, foundation.Unauthenticated)
	bootstrap := call(c, Request{Op: "access.bootstrap"}, foundation.OK)
	if !bootstrap.Data.Setup {
		t.Fatal("first use not offered")
	}
	reg := call(c, Request{Op: "access.register", Name: "technical", Password: "isolated process password"}, foundation.OK)
	if reg.Data.Recovery == "" {
		t.Fatal("recovery absent")
	}
	call(c, Request{Op: "access.login", Name: "technical", Password: "isolated process password", Remember: true}, foundation.OK)
	r := call(c, Request{Op: "company.create", Name: "Alpha"}, foundation.OK)
	id := r.Data.Companies[0].ID
	call(c, Request{Op: "company.select", Name: id}, foundation.OK)
	c2 := NewAccountClient(dirs["runtime"], dirs["client"], uint32(os.Getuid()))
	r = call(c2, Request{Op: "access.bootstrap"}, foundation.OK)
	if !r.Data.Authenticated || r.Data.Profile != id {
		t.Fatal("window reopen did not resume grant")
	}
	// H01: new client objects must not exhaust the live daemon at normal wall clock.
	for i := 0; i < 64; i++ {
		reopened := NewAccountClient(dirs["runtime"], dirs["client"], uint32(os.Getuid()))
		if r := reopened.Call(ctx, Request{Op: "access.bootstrap"}); r.Code != foundation.OK || !r.Data.Authenticated {
			t.Fatalf("reopen %d: %s", i+1, r.Code)
		}
	}
	call(NewAccountClient(dirs["runtime"], dirs["password"], uint32(os.Getuid())), Request{Op: "access.login", Name: "technical", Password: "isolated process password", Remember: true}, foundation.OK)
	// Independent OS processes exercise abandoned leases, including SIGKILL.
	for i := 0; i < 64; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestIntegrationOwnerChild$")
		cmd.Env = append(os.Environ(), "INFRADESKTOP_TEST_CHILD="+root,
			"INFRADESKTOP_STOLEN_SESSION="+c2.session,
			"INFRADESKTOP_STOLEN_PROFILE="+c2.profile,
			"INFRADESKTOP_STOLEN_ACTIVATION="+c2.activation)
		if i < 2 {
			cmd.Env = append(cmd.Env, "INFRADESKTOP_TEST_HOLD=1")
			pipe, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			wait := startOwnedChild(t, cmd)
			file := pipe.(*os.File)
			if err := file.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if line, err := bufio.NewReader(file).ReadString('\n'); err != nil || line != "ready\n" {
				t.Fatalf("child readiness: %q %v", line, err)
			}
			assertSession := func() {
				t.Helper()
				r := c2.exchange(ctx, Request{Op: "access.snapshot", Session: c2.session, Profile: c2.profile, Activation: c2.activation})
				if r.Code != foundation.OK || r.Data == nil || !r.Data.Authenticated {
					t.Fatalf("live parent session lost: %s", r.Code)
				}
			}
			assertSession()
			if err = cmd.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			_ = wait()
			assertSession()
		} else if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("process reopen %d: %v %s", i+1, err, out)
		}
	}
	alive := c2.exchange(ctx, Request{Op: "access.snapshot", Session: c2.session, Profile: c2.profile, Activation: c2.activation})
	if alive.Code != foundation.OK || !alive.Data.Authenticated {
		t.Fatalf("parent after 64 children: %s", alive.Code)
	}
	oldGrant, err := readPrivateToken(dirs["client"], uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	other := NewAccountClient(dirs["runtime"], dirs["other"], uint32(os.Getuid()))
	r = call(other, Request{Op: "access.bootstrap"}, foundation.OK)
	if r.Data.Authenticated {
		t.Fatal("cross client grant leaked")
	}
	// Wrong capability and synthetic legacy operations cannot access the product service.
	r = c.transport.exchange(ctx, Request{Version: 1, ContractRevision: ProductRevision, RequestID: "deny", Op: "access.snapshot", Capability: "wrong", Session: c.session, Profile: c.profile, Activation: c.activation})
	if r.Code != foundation.Unauthenticated {
		t.Fatal("capability bypass")
	}
	r = c.transport.exchange(ctx, Request{Version: 1, RequestID: "deny", Op: "foundation.status", Session: c.session, Profile: c.profile, Activation: c.activation})
	if r.Code != foundation.Invalid {
		t.Fatal("synthetic route exposed")
	}
	oldActivation := c.activation
	call(c2, Request{Op: "company.select", Name: id}, foundation.OK)
	cap, _ := readCapability(dirs["runtime"], uint32(os.Getuid()))
	r = c.transport.exchange(ctx, Request{Version: 1, ContractRevision: ProductRevision, RequestID: "old", Op: "access.snapshot", Capability: cap, Session: c.session, Profile: id, Activation: oldActivation})
	if r.Code != foundation.ProfileChanged {
		t.Fatal("late old activation accepted")
	}
	call(c2, Request{Op: "access.logout"}, foundation.OK)
	if r := c.exchange(ctx, Request{Op: "access.resume", Grant: oldGrant}); r.Code != foundation.Unauthenticated {
		t.Fatal("revoked grant resumed")
	}
	if r := c.exchange(ctx, Request{Op: "access.snapshot", Session: c.session, Profile: c.profile, Activation: c.activation}); r.Code != foundation.Unauthenticated {
		t.Fatal("revoked session accepted")
	}
	r = call(NewAccountClient(dirs["runtime"], dirs["client"], uint32(os.Getuid())), Request{Op: "access.bootstrap"}, foundation.OK)
	if r.Data.Authenticated {
		t.Fatal("logout resurrected")
	}
	if _, e = os.Stat(filepath.Join(dirs["client"], "remembered")); !os.IsNotExist(e) {
		t.Fatal("client token not removed")
	}
}

// Subprocess owns its IPC peer identity and exits without any session shutdown.
func TestIntegrationOwnerChild(t *testing.T) {
	root := os.Getenv("INFRADESKTOP_TEST_CHILD")
	if root == "" {
		return
	}
	c := NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uint32(os.Getuid()))
	if stolen := os.Getenv("INFRADESKTOP_STOLEN_SESSION"); stolen != "" {
		denied := c.exchange(context.Background(), Request{Op: "access.snapshot", Session: stolen, Profile: os.Getenv("INFRADESKTOP_STOLEN_PROFILE"), Activation: os.Getenv("INFRADESKTOP_STOLEN_ACTIVATION")})
		if denied.Code != foundation.Unauthenticated {
			t.Fatalf("different process used parent session: %s", denied.Code)
		}
	}
	r := c.Call(context.Background(), Request{Op: "access.bootstrap"})
	if r.Code != foundation.OK || r.Data == nil || !r.Data.Authenticated {
		t.Fatalf("resume: %s", r.Code)
	}
	if os.Getenv("INFRADESKTOP_TEST_HOLD") == "1" {
		fmt.Println("ready")
		select {}
	}
}

// Register cleanup immediately after Start, before any readiness read/assertion.
func startOwnedChild(t *testing.T, cmd *exec.Cmd) func() error {
	t.Helper()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	var waitError error
	wait := func() error { once.Do(func() { waitError = cmd.Wait() }); return waitError }
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = wait() })
	return wait
}

func TestIntegrationCleanupAfterFatal(t *testing.T) {
	if os.Getenv("INFRADESKTOP_CLEANUP_FAILURE") == "1" {
		cmd := exec.Command("/bin/sleep", "30")
		startOwnedChild(t, cmd)
		fmt.Printf("owned-child=%d\n", cmd.Process.Pid)
		t.Fatal("intentional readiness failure exercises registered cleanup")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestIntegrationCleanupAfterFatal$")
	cmd.Env = append(os.Environ(), "INFRADESKTOP_CLEANUP_FAILURE=1")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "intentional readiness failure") {
		t.Fatalf("wrong failure: %v %s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "owned-child=") {
			pid, e := strconv.Atoi(strings.TrimPrefix(line, "owned-child="))
			if e != nil {
				t.Fatal(e)
			}
			if _, e = os.Stat(fmt.Sprintf("/proc/%d", pid)); !os.IsNotExist(e) {
				t.Fatalf("child %d survived fatal cleanup: %v", pid, e)
			}
			return
		}
	}
	t.Fatal("owned child PID missing")
}
