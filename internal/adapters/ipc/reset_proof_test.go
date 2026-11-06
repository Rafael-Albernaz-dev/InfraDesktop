package ipc

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestResetProofRequiresRootRegularUnlinkedMetadata(t *testing.T) {
	valid := syscall.Stat_t{Uid: 0, Gid: 0, Mode: syscall.S_IFREG | 0644, Nlink: 1, Size: 64}
	if !validResetProofStat(valid) {
		t.Fatal("valid root file rejected")
	}
	cases := map[string]func(*syscall.Stat_t){
		"owner":     func(s *syscall.Stat_t) { s.Uid = 1000 },
		"group":     func(s *syscall.Stat_t) { s.Gid = 1000 },
		"symlink":   func(s *syscall.Stat_t) { s.Mode = syscall.S_IFLNK | 0777 },
		"hard link": func(s *syscall.Stat_t) { s.Nlink = 2 },
		"writable":  func(s *syscall.Stat_t) { s.Mode = syscall.S_IFREG | 0666 },
		"length":    func(s *syscall.Stat_t) { s.Size = 63 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			v := valid
			mutate(&v)
			if validResetProofStat(v) {
				t.Fatal("untrusted proof accepted")
			}
		})
	}
}
func TestResetProofRejectsForeignOwnerAndSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	nonce := strings.Repeat("a", 64)
	name := filepath.Join(dir, "reset-"+nonce)
	if err := os.WriteFile(name, []byte(nonce), 0644); err != nil {
		t.Fatal(err)
	}
	proof := resetFileProof{dir: dir, uid: uint32(os.Getuid())}
	if os.Getuid() == 0 {
		t.Fatal("requires unprivileged synthetic test user")
	}
	if proof.Consume(nonce) {
		t.Fatal("foreign-owned proof accepted")
	}
	if err := os.Remove(name); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(nonce), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Fatal(err)
	}
	if proof.Consume(nonce) {
		t.Fatal("symlink proof accepted")
	}
	if _, err := os.Lstat(name); err != nil {
		t.Fatal("refused proof was modified", err)
	}
}
