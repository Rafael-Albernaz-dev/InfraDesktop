package access

import (
	"context"
	"strings"
	"testing"
	"time"
)

func passwordFixture() (*Service, *resetStore, *time.Time) {
	svc, store, _, now := resetFixture()
	svc.sessions["current"] = session{Owner: "one", UID: 1000, Epoch: 4, Expires: now.Add(time.Hour)}
	svc.sessions["other"] = session{Owner: "two", UID: 1000, Epoch: 4, Expires: now.Add(time.Hour)}
	return svc, store, now
}
func passwordCommand() Command {
	return Command{Op: "access.change-password", Owner: "one", Session: "current", Profile: "company",
		Activation: "activation", CurrentPassword: "old fixture", Password: "new fixture"}
}
func TestChangePasswordRequiresCurrentPasswordAndPreservesCurrentSession(t *testing.T) {
	svc, store, _ := passwordFixture()
	q := passwordCommand()
	wrong := q
	wrong.CurrentPassword = "incorrect"
	view, code := svc.Handle(context.Background(), 1000, wrong)
	if code != Unauthorized || view.ErrorReason != WrongPasswordReason {
		t.Fatal("wrong password not distinguished", code, view.ErrorReason)
	}
	if string(store.state.Account.Password.Hash) != "old fixture" {
		t.Fatal("wrong password changed verifier")
	}
	store.state.NextAttempt = time.Time{}
	view, code = svc.Handle(context.Background(), 1000, q)
	if code != OK || !view.Authenticated || store.state.Account.Epoch != 5 {
		t.Fatal("change failed", code)
	}
	if !svc.crypto.Verify("new fixture", store.state.Account.Password) || len(store.state.Grants) != 0 {
		t.Fatal("new verifier or grant revocation missing")
	}
	if string(store.state.Account.Recovery) != "old code" {
		t.Fatal("recovery was changed")
	}
	q.Op = "access.snapshot"
	q.CurrentPassword = ""
	q.Password = ""
	if _, code = svc.Handle(context.Background(), 1000, q); code != OK {
		t.Fatal("current session lost", code)
	}
	q.Session = "other"
	q.Owner = "two"
	if _, code = svc.Handle(context.Background(), 1000, q); code != Unauthorized {
		t.Fatal("other session survived", code)
	}
}
func TestExpiredSessionIsNotIncorrectPassword(t *testing.T) {
	for _, op := range []string{"access.change-password", "access.renew-recovery"} {
		svc, _, now := passwordFixture()
		q := passwordCommand()
		q.Op = op
		q.Password = "incorrect"
		if op == "access.change-password" {
			q.Password = "valid new password"
			q.CurrentPassword = "incorrect"
		}
		*now = now.Add(2 * time.Hour)
		view, code := svc.Handle(context.Background(), 1000, q)
		if code != Unauthorized || view.ErrorReason != "" {
			t.Fatal("expired session reported as incorrect password", code, view.ErrorReason)
		}
	}
}
func TestChangePasswordRejectsInvalidNewPassword(t *testing.T) {
	svc, _, _ := passwordFixture()
	q := passwordCommand()
	q.Password = "short"
	if _, code := svc.Handle(context.Background(), 1000, q); code != Invalid {
		t.Fatal("short password accepted", code)
	}
}

func TestChangePasswordBoundaries(t *testing.T) {
	for _, tc := range []struct {
		length int
		want   Code
	}{{7, Invalid}, {8, OK}, {128, OK}, {129, Invalid}} {
		svc, _, _ := passwordFixture()
		q := passwordCommand()
		q.Password = strings.Repeat("p", tc.length)
		if _, code := svc.Handle(context.Background(), 1000, q); code != tc.want {
			t.Fatalf("length %d got %s want %s", tc.length, code, tc.want)
		}
	}
}
func TestChangePasswordRejectsConcurrentAccountChange(t *testing.T) {
	svc, store, _ := passwordFixture()
	crypto := &blockedResetCrypto{resetCrypto: &resetCrypto{}, started: make(chan struct{}), release: make(chan struct{})}
	svc.crypto = crypto
	done := make(chan Code, 1)
	go func() { _, code := svc.Handle(context.Background(), 1000, passwordCommand()); done <- code }()
	<-crypto.started
	store.state.Account.Password.Hash = []byte("concurrent fixture")
	close(crypto.release)
	if code := <-done; code != Unauthorized {
		t.Fatal("stale verifier accepted", code)
	}
	if string(store.state.Account.Password.Hash) != "concurrent fixture" {
		t.Fatal("concurrent account overwritten")
	}
}

func TestChangePasswordRejectsEmptyCurrentPassword(t *testing.T) {
	svc, _, _ := passwordFixture()
	q := passwordCommand()
	q.CurrentPassword = ""
	view, code := svc.Handle(context.Background(), 1000, q)
	if code != Unauthorized || view.ErrorReason != WrongPasswordReason {
		t.Fatal("empty current password accepted", code)
	}
}

func TestChangePasswordRejectsSessionRevokedDuringHash(t *testing.T) {
	svc, store, _ := passwordFixture()
	crypto := &blockedResetCrypto{resetCrypto: &resetCrypto{}, started: make(chan struct{}), release: make(chan struct{})}
	svc.crypto = crypto
	done := make(chan Code, 1)
	go func() { _, code := svc.Handle(context.Background(), 1000, passwordCommand()); done <- code }()
	<-crypto.started
	svc.mu.Lock()
	delete(svc.sessions, "current")
	svc.mu.Unlock()
	close(crypto.release)
	if code := <-done; code != Unauthorized {
		t.Fatal("revoked session accepted", code)
	}
	if string(store.state.Account.Password.Hash) != "old fixture" {
		t.Fatal("revoked session changed password")
	}
}
