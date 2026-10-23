package storage

import (
	"context"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/modules/access"
	"sync"
	"testing"
	"time"
)

func TestBF03RenewRecoveryRealResetAndRollback(t *testing.T) {
	j := newJourney(t)
	ctx := context.Background()
	before, _ := j.st.Read(ctx)
	q := j.q
	q.Op = "access.renew-recovery"
	q.Password = password
	noSession := q
	noSession.Session = ""
	j.call(noSession, access.Unauthorized)
	wrongOwner := q
	wrongOwner.Owner = "other"
	j.call(wrongOwner, access.Unauthorized)
	stale := q
	stale.Activation = "old"
	j.call(stale, access.Changed)
	q.Password = "incorrect isolated password"
	j.call(q, access.Unauthorized)
	q.Password = password
	j.call(q, access.Limited)
	j.now = j.now.Add(31 * time.Second)
	j.st.db.Exec("CREATE TRIGGER fail_renew BEFORE UPDATE ON account BEGIN SELECT RAISE(FAIL,'injected rotation commit failure'); END")
	j.call(q, access.Unavailable)
	current, _ := j.st.Read(ctx)
	if !(security.Crypto{}).Equal(before.Account.Recovery, current.Account.Recovery) {
		t.Fatal("failed commit invalidated prior code")
	}
	j.st.db.Exec("DROP TRIGGER fail_renew")
	j.now = j.now.Add(31 * time.Second)
	issued := j.call(q, access.OK)
	j.call(noSession, access.Unauthorized)
	j.call(wrongOwner, access.Unauthorized)
	j.call(stale, access.Changed)
	if issued.Recovery == "" || !issued.Authenticated {
		t.Fatal("one-time issuance missing")
	}
	current, _ = j.st.Read(ctx)
	if current.Account.Epoch != before.Account.Epoch || current.Account.Theme != before.Account.Theme || current.Account.Profile != before.Account.Profile || len(current.Grants) != len(before.Grants) {
		t.Fatal("rotation changed unrelated account state")
	}
	if string(current.Account.Recovery) == issued.Recovery || len(current.Account.Recovery) != 32 {
		t.Fatal("raw secret persisted")
	}
	j.call(q, access.Busy)
	theme := j.q
	theme.Op = "preferences.theme"
	theme.Theme = "dark"
	v := j.call(theme, access.OK)
	if v.Recovery != "" {
		t.Fatal("status reexposes code")
	}
	j.create(companyX, "X", access.Busy)
	ack := j.q
	ack.Op = "access.ack-recovery"
	j.call(ack, access.OK)
	// A subsequent authenticated snapshot still works, including the same session.
	snap := j.q
	snap.Op = "access.snapshot"
	j.call(snap, access.OK)
	// The real reset rejects the old digest and accepts the new emitted code.
	j.now = j.now.Add(31 * time.Second)
	reset := access.Command{Op: "access.recover", Name: "technical", Password: "new isolated test password", Recovery: j.recovery}
	j.call(reset, access.Unauthorized)
	j.now = j.now.Add(31 * time.Second)
	reset.Recovery = issued.Recovery
	j.call(reset, access.OK)
	j.call(snap, access.Unauthorized)
	j.now = j.now.Add(31 * time.Second)
	j.call(reset, access.Unauthorized)
}

type renewalCrypto struct {
	security.Crypto
	mu               sync.Mutex
	calls            int
	started, release chan struct{}
}

func (c *renewalCrypto) Verify(p string, v access.Verifier) bool {
	c.mu.Lock()
	c.calls++
	n := c.calls
	c.mu.Unlock()
	if n == 2 {
		close(c.started)
		<-c.release
	}
	return c.Crypto.Verify(p, v)
}
func TestBF03RenewalRevalidatesEpochWithoutBlockingSnapshot(t *testing.T) {
	st, e := Open(privateDir(t))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	now := time.Now()
	crypto := &renewalCrypto{started: make(chan struct{}), release: make(chan struct{})}
	svc := access.New(st, crypto, func() time.Time { return now }, 1000, "a")
	ctx := context.Background()
	svc.Handle(ctx, 1000, access.Command{Op: "access.register", Name: "technical", Password: password})
	now = now.Add(31 * time.Second)
	login, c := svc.Handle(ctx, 1000, access.Command{Op: "access.login", Name: "technical", Password: password, Owner: "one"})
	if c != access.OK {
		t.Fatal(c)
	}
	q := access.Command{Op: "access.renew-recovery", Owner: "one", Session: login.Session, Profile: login.Profile, Activation: login.Activation, Password: password}
	done := make(chan access.Code, 1)
	go func() { _, c := svc.Handle(ctx, 1000, q); done <- c }()
	<-crypto.started
	if _, c = svc.Handle(ctx, 1000, q); c != access.Busy {
		t.Fatal("concurrent KDF not rejected")
	}
	snap := q
	snap.Op = "access.snapshot"
	if _, c = svc.Handle(ctx, 1000, snap); c != access.OK {
		t.Fatal("snapshot blocked")
	}
	logout := q
	logout.Op = "access.logout"
	if _, c = svc.Handle(ctx, 1000, logout); c != access.OK {
		t.Fatal(c)
	}
	close(crypto.release)
	if c = <-done; c != access.Unauthorized {
		t.Fatal("revoked epoch issued code", c)
	}
}

func TestBF03LostResponseRequiresExplicitReissue(t *testing.T) {
	j := newJourney(t)
	q := j.q
	q.Op = "access.renew-recovery"
	q.Password = password
	first := j.call(q, access.OK)
	j.call(q, access.Busy)
	q.Replace = true
	second := j.call(q, access.OK)
	if first.Recovery == second.Recovery {
		t.Fatal("replacement did not rotate")
	}
	ack := j.q
	ack.Op = "access.ack-recovery"
	j.call(ack, access.OK)
	j.now = j.now.Add(31 * time.Second)
	reset := access.Command{Op: "access.recover", Name: "technical", Password: password, Recovery: first.Recovery}
	j.call(reset, access.Unauthorized)
	j.now = j.now.Add(31 * time.Second)
	reset.Recovery = second.Recovery
	j.call(reset, access.OK)
}
