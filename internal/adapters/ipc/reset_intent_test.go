package ipc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResetIntentSurvivesClientAndSerializesVerification(t *testing.T) {
	dir := filepath.Join(shortResetRoot(t), "client")
	client := NewAccountClient("/unused", dir, uint32(os.Getuid()))
	pending, err := client.ResetIntentPending()
	if err != nil || pending {
		t.Fatal("unexpected intent", err)
	}
	lease, err := client.BeginResetIntent()
	if err != nil {
		t.Fatal(err)
	}
	other := NewAccountClient("/unused", dir, uint32(os.Getuid()))
	if pending, err = other.ResetIntentPending(); err != nil || !pending {
		t.Fatal("intent not visible", err)
	}
	if _, err = other.BeginResetIntent(); err != ErrResetVerification {
		t.Fatal("second reset accepted", err)
	}
	if _, err = other.LockResetIntent(); err != ErrResetVerification {
		t.Fatal("verification crossed active reset", err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	lease, err = other.LockResetIntent()
	if err != nil || lease == nil {
		t.Fatal("restart cannot verify", err)
	}
	if err = lease.Clear(); err != nil {
		t.Fatal(err)
	}
	if err = lease.Close(); err != nil {
		t.Fatal(err)
	}
	if pending, err = client.ResetIntentPending(); err != nil || pending {
		t.Fatal("verified intent retained", err)
	}
}
func TestResetIntentRejectsSymlink(t *testing.T) {
	dir := filepath.Join(shortResetRoot(t), "client")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target")
	if err := os.WriteFile(target, []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, resetIntentName)); err != nil {
		t.Fatal(err)
	}
	client := NewAccountClient("/unused", dir, uint32(os.Getuid()))
	if _, err := client.LockResetIntent(); err == nil {
		t.Fatal("symlink accepted")
	}
	if _, err := client.BeginResetIntent(); err != ErrResetVerification {
		t.Fatal("symlink replaced", err)
	}
}

func shortResetRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("", "reset-fixture-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(root); err != nil {
			t.Error(err)
		}
	})
	return root
}
