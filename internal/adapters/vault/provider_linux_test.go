package vault

import (
	"bytes"
	"encoding/binary"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"testing"
)

func aclFixture(uid uint32, perm uint16) []byte {
	b := make([]byte, 44)
	binary.LittleEndian.PutUint32(b, 2)
	for i, tag := range []uint16{1, 2, 4, 16, 32} {
		row := b[4+i*8:]
		binary.LittleEndian.PutUint16(row, tag)
		p := uint16(0)
		if i == 0 || i == 1 || i == 3 {
			p = perm
		}
		binary.LittleEndian.PutUint16(row[2:], p)
		id := uint32(0xffffffff)
		if i == 1 {
			id = uid
		}
		binary.LittleEndian.PutUint32(row[4:], id)
	}
	return b
}
func TestSystemdMetadataObservedACL(t *testing.T) {
	for _, dir := range []bool{false, true} {
		st := unix.Stat_t{Uid: 0, Gid: 0, Mode: unix.S_IFREG | 0440, Nlink: 1, Size: 32}
		perm := uint16(4)
		if dir {
			st.Mode = unix.S_IFDIR | 0550
			perm = 5
		}
		acl := aclFixture(1000, perm)
		if !systemdMetadata(st, acl, 1000, dir) {
			t.Fatal("observed systemd ACL rejected")
		}
		for _, change := range []func(*unix.Stat_t, []byte){
			func(s *unix.Stat_t, b []byte) { s.Uid = 1000 }, func(s *unix.Stat_t, b []byte) { s.Gid = 1000 },
			func(s *unix.Stat_t, b []byte) { s.Mode |= 0004 }, func(s *unix.Stat_t, b []byte) { s.Mode |= 0020 },
			func(s *unix.Stat_t, b []byte) { binary.LittleEndian.PutUint32(b[16:], 1001) },
			func(s *unix.Stat_t, b []byte) { binary.LittleEndian.PutUint16(b[22:], 4) },
			func(s *unix.Stat_t, b []byte) { binary.LittleEndian.PutUint16(b[38:], 4) },
		} {
			bad := st
			b := bytes.Clone(acl)
			change(&bad, b)
			if systemdMetadata(bad, b, 1000, dir) {
				t.Fatal("unsafe metadata accepted")
			}
		}
		if systemdMetadata(st, append(acl, make([]byte, 8)...), 1000, dir) || systemdMetadata(st, nil, 1000, dir) || systemdMetadata(st, acl, 0, dir) {
			t.Fatal("extra/missing ACL or root service accepted")
		}
		if !dir {
			for _, change := range []func(*unix.Stat_t){func(s *unix.Stat_t) { s.Nlink = 2 }, func(s *unix.Stat_t) { s.Size = 31 }, func(s *unix.Stat_t) { s.Mode = unix.S_IFLNK | 0440 }} {
				bad := st
				change(&bad)
				if systemdMetadata(bad, acl, 1000, false) {
					t.Fatal("bad file accepted")
				}
			}
		}
	}
}
func TestSystemdRejectsUntrustedDirectoryAndDEVExposure(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, CredentialName)
	if e := os.WriteFile(p, bytes.Repeat([]byte{21}, 32), 0600); e != nil {
		t.Fatal(e)
	}
	t.Setenv("CREDENTIALS_DIRECTORY", dir)
	if b, e := Systemd(); e == nil || b != nil {
		t.Fatal("arbitrary directory accepted as systemd")
	}
	if b, e := ReadKey(p); e != nil || len(b) != 32 {
		t.Fatal("private DEV key rejected")
	}
	if e := os.Chmod(p, 0440); e != nil {
		t.Fatal(e)
	}
	if b, e := ReadKey(p); e == nil || b != nil {
		t.Fatal("systemd permissions relaxed DEV")
	}
	f, e := os.Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	replacement := p + ".new"
	os.WriteFile(replacement, bytes.Repeat([]byte{22}, 32), 0600)
	os.Rename(replacement, p)
	if b, e := stableKey(f, p, mustStat(t, f)); e == nil || b != nil {
		t.Fatal("replaced path accepted")
	}
}

func mustStat(t *testing.T, f *os.File) os.FileInfo {
	t.Helper()
	st, e := f.Stat()
	if e != nil {
		t.Fatal(e)
	}
	return st
}
func TestStableReadBindsAuthorizedSnapshot(t *testing.T) {
	for _, mode := range []os.FileMode{0600, 0644, 0660} {
		t.Run(mode.String(), func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "key")
			if e := os.WriteFile(p, bytes.Repeat([]byte{31}, 32), 0600); e != nil {
				t.Fatal(e)
			}
			f, e := os.Open(p)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			authorized := mustStat(t, f)
			if mode != 0600 {
				if e = os.Chmod(p, mode); e != nil {
					t.Fatal(e)
				}
			}
			b, e := stableKey(f, p, authorized)
			if mode == 0600 {
				if e != nil || len(b) != 32 {
					t.Fatal("unchanged private key rejected")
				}
			} else if e == nil || b != nil {
				t.Fatal("changed authorized metadata accepted")
			}
		})
	}
}
