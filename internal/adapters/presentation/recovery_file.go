package presentation

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"strings"
)

// SaveRecovery has no JS-supplied path or contents. The native picker is the
// sole source of a destination; no mutex is held while a person chooses it.
func (a *App) SaveRecovery() string {
	a.mu.Lock()
	if a.saving {
		a.mu.Unlock()
		return "BUSY"
	}
	if c := a.checkRecovery(); c != "OK" {
		a.mu.Unlock()
		return c
	}
	a.saving = true
	generation := a.recoveryGeneration
	ctx := a.ctx
	picker := a.saveDialog
	a.mu.Unlock()
	path, err := picker(ctx, runtime.SaveDialogOptions{Title: "Salvar código de recuperação", DefaultFilename: "codigo-recuperacao.txt", Filters: []runtime.FileFilter{{DisplayName: "Texto (*.txt)", Pattern: "*.txt"}}})
	a.mu.Lock()
	defer a.mu.Unlock()
	defer func() { a.saving = false }()
	if err != nil {
		return "SAVE_UNAVAILABLE"
	}
	if path == "" {
		return "CANCELLED"
	}
	if generation != a.recoveryGeneration {
		return "PROFILE_CHANGED"
	}
	if c := a.checkRecovery(); c != "OK" {
		return c
	}
	return writeRecovery(path, a.recovery)
}

// Open every destination ancestor without following symbolic links. Keep the
// directory descriptor through publication, cleanup and fsync (no path races).
func recoveryDir(path string) (int, error) {
	fd, e := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if e != nil {
		return -1, e
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		if part == "" {
			continue
		}
		next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if err != nil {
			return -1, err
		}
		fd = next
	}
	return fd, nil
}
func writeRecovery(path, code string) string {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Ext(path) != ".txt" {
		return "SAVE_INVALID_DESTINATION"
	}
	dir, e := recoveryDir(filepath.Dir(path))
	if e != nil {
		return "SAVE_UNAVAILABLE"
	}
	defer unix.Close(dir)
	var random [16]byte
	if _, e = rand.Read(random[:]); e != nil {
		return "SAVE_UNAVAILABLE"
	}
	stage := ".infradesktop-recovery-" + hex.EncodeToString(random[:])
	if unix.Mkdirat(dir, stage, 0700) != nil {
		return "SAVE_UNAVAILABLE"
	}
	source, e := unix.Openat(dir, stage, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return "SAVE_UNAVAILABLE"
	}
	defer unix.Close(source)
	var stageInfo unix.Stat_t
	if unix.Fstat(source, &stageInfo) != nil || stageInfo.Uid != uint32(os.Getuid()) || stageInfo.Mode&0077 != 0 {
		return "SAVE_UNAVAILABLE"
	}
	defer func() {
		var current unix.Stat_t
		if unix.Fstatat(dir, stage, &current, unix.AT_SYMLINK_NOFOLLOW) == nil && stageInfo.Ino == current.Ino && stageInfo.Dev == current.Dev {
			_ = unix.Unlinkat(dir, stage, unix.AT_REMOVEDIR)
		}
	}()
	temp := "code"
	fd, e := unix.Openat(source, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if e != nil {
		return "SAVE_UNAVAILABLE"
	}
	f := os.NewFile(uintptr(fd), temp)
	defer f.Close()
	var own unix.Stat_t
	if e = unix.Fstat(fd, &own); e != nil {
		return "SAVE_UNAVAILABLE"
	}
	defer func() {
		var current unix.Stat_t
		if unix.Fstatat(source, temp, &current, unix.AT_SYMLINK_NOFOLLOW) == nil && own.Ino == current.Ino && own.Dev == current.Dev {
			_ = unix.Unlinkat(source, temp, 0)
		}
	}()
	if e = f.Chmod(0600); e != nil {
		return "SAVE_UNAVAILABLE"
	}
	if _, e = f.WriteString(code + "\n\nCódigo de recuperação InfraDesktop. Guarde offline em local seguro.\n"); e != nil {
		return "SAVE_UNAVAILABLE"
	}
	if e = f.Sync(); e != nil {
		return "SAVE_UNAVAILABLE"
	}
	e = unix.Renameat2(source, temp, dir, filepath.Base(path), unix.RENAME_NOREPLACE)
	if errors.Is(e, unix.EEXIST) {
		return "SAVE_EXISTS"
	}
	if e != nil {
		return "SAVE_UNAVAILABLE"
	}
	if unix.Fsync(dir) != nil {
		return "SAVE_UNAVAILABLE"
	}
	return "OK"
}
