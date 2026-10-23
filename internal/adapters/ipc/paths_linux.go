package ipc

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
)

var errUnsafe = errors.New("unsafe runtime endpoint")

func RandomID() (string, error) {
	var b [32]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
func owned(info os.FileInfo, uid uint32) bool {
	v, ok := info.Sys().(*syscall.Stat_t)
	return ok && v.Uid == uid
}
func safeDir(path string, uid uint32) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(path)+len("/service.sock") > 107 {
		return errUnsafe
	}
	for p := path; ; p = filepath.Dir(p) {
		st, e := os.Lstat(p)
		if e != nil {
			return errUnsafe
		}
		if !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
			return errUnsafe
		}
		if p == path {
			if !owned(st, uid) || st.Mode().Perm() != 0700 {
				return errUnsafe
			}
		} else if st.Mode().Perm()&0022 != 0 && !(owned(st, 0) && st.Mode()&os.ModeSticky != 0) {
			return errUnsafe
		}
		if p == "/" {
			break
		}
	}
	return nil
}
func privateFile(path string, uid uint32, mode os.FileMode) (os.FileInfo, error) {
	st, e := os.Lstat(path)
	if e != nil || !owned(st, uid) || st.Mode().Perm() != 0600 || st.Mode()&os.ModeType != mode {
		return nil, errUnsafe
	}
	if v, ok := st.Sys().(*syscall.Stat_t); !ok || v.Nlink != 1 {
		return nil, errUnsafe
	}
	return st, nil
}
func peerUID(c *net.UnixConn) (uint32, error) {
	raw, e := c.SyscallConn()
	if e != nil {
		return 0, e
	}
	var uid uint32
	var inner error
	e = raw.Control(func(fd uintptr) {
		v, er := syscall.GetsockoptUcred(int(fd), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
		inner = er
		if er == nil {
			uid = v.Uid
		}
	})
	if e != nil {
		return 0, e
	}
	return uid, inner
}
func readCapability(dir string, uid uint32) (string, error) {
	p := filepath.Join(dir, "capability")
	st, e := privateFile(p, uid, 0)
	if e != nil {
		return "", e
	}
	fd, e := syscall.Open(p, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if e != nil {
		return "", errUnsafe
	}
	f := os.NewFile(uintptr(fd), p)
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(st, actual) || actual.Size() != 64 {
		return "", errUnsafe
	}
	var b [64]byte
	n, e := f.Read(b[:])
	if n != 64 || e != nil {
		return "", errUnsafe
	}
	if _, e = hex.DecodeString(string(b[:])); e != nil {
		return "", errUnsafe
	}
	return string(b[:]), nil
}
func removeOwned(path string, st os.FileInfo) {
	if st == nil {
		return
	}
	now, e := os.Lstat(path)
	if e == nil && os.SameFile(st, now) {
		_ = os.Remove(path)
	}
}
