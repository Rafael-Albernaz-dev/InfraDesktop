package storage

import (
	"context"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func privateDir(t *testing.T) string {
	t.Helper()
	d := t.TempDir()
	if e := os.Chmod(d, 0700); e != nil {
		t.Fatal(e)
	}
	return d
}

const password = "isolated test password ç"

func TestAccountJourneySQLite(t *testing.T) {
	dir := privateDir(t)
	st, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { st.Close() }()
	ctx := context.Background()
	now := time.Now()
	clock := func() time.Time { return now }
	crypto := security.Crypto{}
	svc := access.New(st, crypto, clock, 1000, "activation-one")
	call := func(q access.Command, want access.Code) access.View {
		t.Helper()
		v, c := svc.Handle(ctx, 1000, q)
		if c != want {
			t.Fatalf("%s got %s want %s", q.Op, c, want)
		}
		return v
	}
	reg := call(access.Command{Op: "access.register", Name: "technical", Password: password}, access.OK)
	if len(reg.Recovery) != 64 {
		t.Fatal("one-time recovery missing")
	}
	call(access.Command{Op: "access.register", Name: "second", Password: password}, access.Conflict)
	now = now.Add(31 * time.Second)
	call(access.Command{Op: "access.login", Name: "technical", Password: "incorrect long password"}, access.Unauthorized)
	call(access.Command{Op: "access.login", Name: "technical", Password: password}, access.Limited)
	now = now.Add(31 * time.Second)
	login := call(access.Command{Op: "access.login", Name: "technical", Password: password, Remember: true}, access.OK)
	if login.Session == "" || login.Grant == "" || !login.Authenticated {
		t.Fatal("distinct authorization missing")
	}
	q := access.Command{Session: login.Session, Activation: login.Activation, Profile: login.Profile}
	q.Op = "company.create"
	q.Operation = companyX
	q.Name = "Alpha ' ? ; ç"
	v := call(q, access.OK)
	if len(v.Companies) != 1 {
		t.Fatal("company not persisted")
	}
	first := v.Companies[0].ID
	if len(first) != 36 {
		t.Fatal("stable UUID missing")
	}
	q.Name = "Beta"
	q.Operation = companyY
	v = call(q, access.OK)
	second := v.Companies[1].ID
	q.Op = "company.select"
	q.Name = first
	v = call(q, access.OK)
	q.Op = "access.snapshot"
	call(q, access.Changed)
	q.Profile = v.Profile
	q.Activation = v.Activation
	q.Op = "preferences.theme"
	q.Theme = "dark"
	call(q, access.OK)
	q.Op = "company.select"
	q.Name = second
	v = call(q, access.OK)
	q.Op = "access.snapshot"
	call(q, access.Changed)
	st.Close()
	st, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	svc = access.New(st, crypto, clock, 1000, "activation-restarted")
	call(access.Command{Op: "access.snapshot", Session: login.Session, Profile: second, Activation: v.Activation}, access.Unauthorized)
	resumed := call(access.Command{Op: "access.resume", Grant: login.Grant}, access.OK)
	if resumed.Theme != "dark" || resumed.Profile != second || len(resumed.Companies) != 2 || resumed.Companies[0].ID != first {
		t.Fatal("restart lost durable state")
	}
	q = access.Command{Op: "access.logout", Session: resumed.Session, Profile: resumed.Profile, Activation: resumed.Activation}
	// An actual SQLite transaction failure must not report success or revoke only memory.
	if _, e = st.db.Exec("CREATE TRIGGER fail_logout BEFORE UPDATE ON account BEGIN SELECT RAISE(FAIL,'injected disk write failure'); END"); e != nil {
		t.Fatal(e)
	}
	call(q, access.Unavailable)
	q.Op = "access.snapshot"
	call(q, access.OK)
	if _, e = st.db.Exec("DROP TRIGGER fail_logout"); e != nil {
		t.Fatal(e)
	}
	q.Op = "access.logout"
	call(q, access.OK)
	svc = access.New(st, crypto, clock, 1000, "again")
	call(access.Command{Op: "access.resume", Grant: login.Grant}, access.Unauthorized)
	now = now.Add(31 * time.Second)
	login = call(access.Command{Op: "access.login", Name: "technical", Password: password, Remember: true}, access.OK)
	now = now.Add(31 * time.Second)
	call(access.Command{Op: "access.recover", Name: "technical", Password: "a new isolated password", Recovery: strings.Repeat("0", 64)}, access.Unauthorized)
	now = now.Add(31 * time.Second)
	// Recovery rollback also preserves old password, grant and recovery verifier.
	if _, e = st.db.Exec("CREATE TRIGGER fail_reset BEFORE UPDATE ON account BEGIN SELECT RAISE(FAIL,'injected rollback'); END"); e != nil {
		t.Fatal(e)
	}
	call(access.Command{Op: "access.recover", Name: "technical", Password: "a new isolated password", Recovery: reg.Recovery}, access.Unavailable)
	if _, e = st.db.Exec("DROP TRIGGER fail_reset"); e != nil {
		t.Fatal(e)
	}
	now = now.Add(31 * time.Second)
	reset := call(access.Command{Op: "access.recover", Name: "technical", Password: "a new isolated password", Recovery: reg.Recovery}, access.OK)
	if reset.Recovery == reg.Recovery || len(reset.Recovery) != 64 {
		t.Fatal("recovery not rotated")
	}
	call(access.Command{Op: "access.resume", Grant: login.Grant}, access.Unauthorized)
	call(access.Command{Op: "access.snapshot", Session: login.Session, Profile: login.Profile, Activation: login.Activation}, access.Unauthorized)
	st.Close()
	st, e = Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	svc = access.New(st, crypto, clock, 1000, "last-restart")
	now = now.Add(31 * time.Second)
	call(access.Command{Op: "access.recover", Name: "technical", Password: "a new isolated password", Recovery: reg.Recovery}, access.Unauthorized)
	now = now.Add(31 * time.Second)
	call(access.Command{Op: "access.login", Name: "technical", Password: password}, access.Unauthorized)
	now = now.Add(31 * time.Second)
	ephemeral := call(access.Command{Op: "access.login", Name: "technical", Password: "a new isolated password"}, access.OK)
	if ephemeral.Grant != "" {
		t.Fatal("ephemeral login created grant")
	}
	svc = access.New(st, crypto, clock, 1000, "last")
	call(access.Command{Op: "access.resume", Grant: ephemeral.Grant}, access.Unauthorized)
	diag, e := st.Diagnostics()
	if e != nil || diag != "journal=wal foreign_keys=1 busy_timeout=1000 pool=1" {
		t.Fatalf("sqlite config %s %v", diag, e)
	}
	t.Log(diag)
}
func TestConcurrentFirstRegistration(t *testing.T) {
	st, e := Open(privateDir(t))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	svc := access.New(st, security.Crypto{}, time.Now, 1000, "one")
	var wg sync.WaitGroup
	results := make(chan access.Code, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, c := svc.Handle(context.Background(), 1000, access.Command{Op: "access.register", Name: "technical", Password: password})
			results <- c
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for c := range results {
		if c == access.OK {
			success++
		} else if c != access.Busy && c != access.Conflict && c != access.Limited {
			t.Fatal(c)
		}
	}
	if success != 1 {
		t.Fatalf("registered %d accounts", success)
	}
}
func TestDatabaseFailsClosed(t *testing.T) {
	dir := privateDir(t)
	p := filepath.Join(dir, "infradesktop.db")
	raw := []byte("invalid existing database must survive")
	if e := os.WriteFile(p, raw, 0600); e != nil {
		t.Fatal(e)
	}
	if s, e := Open(dir); e == nil {
		s.Close()
		t.Fatal("invalid DB opened")
	}
	b, _ := os.ReadFile(p)
	if string(b) != string(raw) {
		t.Fatal("invalid DB replaced")
	}
	dir = privateDir(t)
	s, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.db.Exec("UPDATE schema_migrations SET checksum='invalid'"); e != nil {
		t.Fatal(e)
	}
	s.Close()
	if s, e = Open(dir); e == nil {
		s.Close()
		t.Fatal("bad migration accepted")
	}
}

func TestPrivateStateAndMigrationOwner(t *testing.T) {
	dir := privateDir(t)
	st, e := Open(dir)
	if e != nil {
		t.Fatal(e)
	}
	if other, e := Open(dir); e == nil {
		other.Close()
		t.Fatal("two migration owners allowed")
	}
	st.Close()
	link := filepath.Join(privateDir(t), "linked-state")
	if e = os.Symlink(dir, link); e != nil {
		t.Fatal(e)
	}
	if other, e := Open(link); e == nil {
		other.Close()
		t.Fatal("symlink state allowed")
	}
	if e = os.Chmod(dir, 0755); e != nil {
		t.Fatal(e)
	}
	if other, e := Open(dir); e == nil {
		other.Close()
		t.Fatal("public state allowed")
	}
}

type slowCrypto struct {
	security.Crypto
	started, release chan struct{}
}

func (s slowCrypto) Verify(p string, v access.Verifier) bool {
	close(s.started)
	<-s.release
	return s.Crypto.Verify(p, v)
}
func TestKDFBoundAndStatusResponsive(t *testing.T) {
	st, e := Open(privateDir(t))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	now := time.Now()
	clock := func() time.Time { return now }
	crypto := slowCrypto{started: make(chan struct{}), release: make(chan struct{})}
	svc := access.New(st, crypto, clock, 1000, "one")
	_, c := svc.Handle(context.Background(), 1000, access.Command{Op: "access.register", Name: "technical", Password: password})
	if c != access.OK {
		t.Fatal(c)
	}
	now = now.Add(31 * time.Second)
	done := make(chan access.Code, 1)
	go func() {
		_, c := svc.Handle(context.Background(), 1000, access.Command{Op: "access.login", Name: "technical", Password: password})
		done <- c
	}()
	<-crypto.started
	_, c = svc.Handle(context.Background(), 1000, access.Command{Op: "access.login", Name: "technical", Password: password})
	if c != access.Busy {
		t.Fatal("KDF not bounded")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, c = svc.Handle(ctx, 1000, access.Command{Op: "access.bootstrap"})
	if c != access.OK {
		t.Fatal("status blocked by KDF")
	}
	close(crypto.release)
	if <-done != access.OK {
		t.Fatal("login failed")
	}
}
