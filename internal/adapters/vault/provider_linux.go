package vault

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const CredentialName = "infradesktop-vault-master"

var errCredential = errors.New("credential unavailable")

// Systemd accepts only the manager's root-owned read-only credential mount.
// The POSIX ACL grants this service UID read access, not the root group.
// Ordinary root:0440 files and arbitrary CREDENTIALS_DIRECTORY values fail closed.
func Systemd() ([]byte, error) {
	dir := os.Getenv("CREDENTIALS_DIRECTORY")
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || filepath.Dir(dir) != "/run/credentials" || !strings.HasSuffix(filepath.Base(dir), ".service") {
		return nil, errCredential
	}
	parent, err := openRootDirectory("/run/credentials")
	if err != nil {
		return nil, errCredential
	}
	defer parent.Close()
	fd, err := unix.Openat(int(parent.Fd()), filepath.Base(dir), unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, errCredential
	}
	directory := os.NewFile(uintptr(fd), dir)
	defer directory.Close()
	if !systemdObject(directory, true) {
		return nil, errCredential
	}
	var fs unix.Statfs_t
	if unix.Fstatfs(fd, &fs) != nil || fs.Type != unix.TMPFS_MAGIC || fs.Flags&unix.ST_RDONLY == 0 {
		return nil, errCredential
	}
	keyfd, err := unix.Openat(fd, CredentialName, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, errCredential
	}
	f := os.NewFile(uintptr(keyfd), CredentialName)
	defer f.Close()
	before, statErr := f.Stat()
	if statErr != nil || !systemdObject(f, false) {
		return nil, errCredential
	}
	b, err := stableKey(f, filepath.Join(dir, CredentialName), before)
	if err != nil {
		return nil, err
	}
	// Recheck ACLs and directory identity after reading; a changed path is not trusted.
	actual, e := os.Lstat(dir)
	opened, e2 := directory.Stat()
	if e != nil || e2 != nil || !os.SameFile(actual, opened) || !systemdObject(directory, true) || !systemdObject(f, false) {
		clear(b)
		return nil, errCredential
	}
	return b, nil
}

func openRootDirectory(path string) (*os.File, error) {
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return nil, e
	}
	current := os.NewFile(uintptr(fd), "/")
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		next, e := unix.Openat(int(current.Fd()), part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		current.Close()
		if e != nil {
			return nil, e
		}
		current = os.NewFile(uintptr(next), part)
		var st unix.Stat_t
		if unix.Fstat(next, &st) != nil || st.Uid != 0 || st.Mode&0022 != 0 {
			current.Close()
			return nil, errCredential
		}
	}
	return current, nil
}

func systemdObject(f *os.File, directory bool) bool {
	var st unix.Stat_t
	if unix.Fstat(int(f.Fd()), &st) != nil {
		return false
	}
	acl := make([]byte, 256)
	n, e := unix.Fgetxattr(int(f.Fd()), "system.posix_acl_access", acl)
	return e == nil && systemdMetadata(st, acl[:n], uint32(os.Geteuid()), directory)
}
func systemdMetadata(st unix.Stat_t, acl []byte, uid uint32, directory bool) bool {
	mode := uint32(unix.S_IFREG | 0440)
	perm := uint16(4)
	if directory {
		mode = unix.S_IFDIR | 0550
		perm = 5
	}
	if uid == 0 || st.Uid != 0 || st.Gid != 0 || st.Mode != mode || (!directory && (st.Nlink != 1 || st.Size != 32)) {
		return false
	}
	// Linux POSIX ACL xattr v2: owner, one named service UID, no group,
	// matching mask and no other access. Reject extra users or groups in the effective access ACL.
	if len(acl) != 44 || binary.LittleEndian.Uint32(acl) != 2 {
		return false
	}
	tags := []uint16{1, 2, 4, 16, 32}
	permissions := []uint16{perm, perm, 0, perm, 0}
	for i, tag := range tags {
		entry := acl[4+i*8 : 12+i*8]
		expected := uint32(0xffffffff)
		if tag == 2 {
			expected = uid
		}
		if binary.LittleEndian.Uint16(entry) != tag || binary.LittleEndian.Uint16(entry[2:]) != permissions[i] || binary.LittleEndian.Uint32(entry[4:]) != expected {
			return false
		}
	}
	return true
}

// ReadKey is the deliberately separate DEV provider. Its permissions are not
// relaxed for systemd, and the normal production composition never selects it.
func ReadKey(path string) ([]byte, error) {
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, errCredential
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return nil, errCredential
	}
	u, ok := st.Sys().(*syscall.Stat_t)
	if !ok || u.Uid != uint32(os.Geteuid()) || !st.Mode().IsRegular() || u.Mode&0077 != 0 || u.Mode&07000 != 0 || u.Nlink != 1 || st.Size() != 32 {
		return nil, errCredential
	}
	return stableKey(f, path, st)
}

// before is the snapshot whose metadata was authorized by the provider.
func stableKey(f *os.File, path string, before os.FileInfo) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(f, 33))
	after, e2 := f.Stat()
	named, e3 := os.Lstat(path)
	if e != nil || e2 != nil || e3 != nil || len(b) != 32 || !stableMetadata(before, after) || !stableMetadata(after, named) {
		clear(b)
		return nil, errCredential
	}
	return b, nil
}
func stableMetadata(a, b os.FileInfo) bool {
	x, ok := a.Sys().(*syscall.Stat_t)
	y, ok2 := b.Sys().(*syscall.Stat_t)
	return ok && ok2 && os.SameFile(a, b) && a.Mode().IsRegular() && b.Mode().IsRegular() && x.Mode == y.Mode && x.Uid == y.Uid && x.Gid == y.Gid && x.Nlink == 1 && y.Nlink == 1 && x.Size == 32 && y.Size == 32 && x.Mtim == y.Mtim && x.Ctim == y.Ctim
}
