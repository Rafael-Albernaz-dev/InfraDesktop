package storage

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

func ownedPrivate(path string, dir bool) error {
	st, e := os.Lstat(path)
	if e != nil {
		return e
	}
	v, ok := st.Sys().(*syscall.Stat_t)
	if !ok || v.Uid != uint32(os.Getuid()) || st.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe state owner")
	}
	if dir {
		if !st.IsDir() || st.Mode().Perm() != 0700 {
			return errors.New("unsafe state directory")
		}
	} else if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || v.Nlink != 1 {
		return errors.New("unsafe database file")
	}
	for p := filepath.Dir(path); ; p = filepath.Dir(p) {
		info, e := os.Lstat(p)
		if e != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("unsafe state ancestor")
		}
		v, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("invalid state ancestor")
		}
		if info.Mode().Perm()&0022 != 0 && !(v.Uid == 0 && info.Mode()&os.ModeSticky != 0) {
			return errors.New("writable state ancestor")
		}
		if p == "/" {
			break
		}
	}
	return nil
}

func lockState(dir string) (*os.File, error) {
	path := filepath.Join(dir, "database.lock")
	fd, e := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_CLOEXEC|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	f := os.NewFile(uintptr(fd), path)
	if e = ownedPrivate(path, false); e != nil {
		f.Close()
		return nil, e
	}
	if e = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		f.Close()
		return nil, errors.New("state already owned by another daemon")
	}
	return f, nil
}
