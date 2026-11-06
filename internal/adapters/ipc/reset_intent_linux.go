package ipc

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

var ErrResetVerification = errors.New("reset verification required")

const resetIntentName = "reset-verification-required"

type ResetIntent struct {
	file      *os.File
	dir, path string
}

func (c *AccountClient) ResetIntentPending() (bool, error) {
	if _, err := os.Lstat(c.dir); os.IsNotExist(err) {
		return false, nil
	}
	if err := safeDir(c.dir, c.transport.uid); err != nil {
		return false, err
	}
	_, err := os.Lstat(filepath.Join(c.dir, resetIntentName))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (c *AccountClient) BeginResetIntent() (*ResetIntent, error) {
	if err := os.MkdirAll(c.dir, 0700); err != nil {
		return nil, err
	}
	if err := safeDir(c.dir, c.transport.uid); err != nil {
		return nil, err
	}
	path := filepath.Join(c.dir, resetIntentName)
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if errors.Is(err, syscall.EEXIST) {
		return nil, ErrResetVerification
	}
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	intent := &ResetIntent{file: file, dir: c.dir, path: path}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, err
	}
	if _, err = file.WriteString("verification-required\n"); err == nil {
		err = file.Sync()
	}
	if err == nil {
		err = syncDirectory(c.dir)
	}
	if err != nil {
		file.Close()
		return nil, err
	}
	return intent, nil
}

func (c *AccountClient) LockResetIntent() (*ResetIntent, error) {
	if err := safeDir(c.dir, c.transport.uid); err != nil {
		return nil, err
	}
	path := filepath.Join(c.dir, resetIntentName)
	before, err := privateFile(path, c.transport.uid, 0)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	actual, err := file.Stat()
	if err != nil || !os.SameFile(before, actual) {
		file.Close()
		return nil, errUnsafe
	}
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, ErrResetVerification
	}
	return &ResetIntent{file: file, dir: c.dir, path: path}, nil
}

func (i *ResetIntent) Clear() error {
	if err := os.Remove(i.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return syncDirectory(i.dir)
}
func (i *ResetIntent) Close() error { return i.file.Close() }
