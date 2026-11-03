package presentation

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/sys/unix"
	"infradesktop/internal/modules/equipment"
	"io"
	"os"
	"path/filepath"
)

func writeJSONValues(w io.Writer, values []any, first *bool) string {
	for _, v := range values {
		if !*first {
			if _, err := io.WriteString(w, ","); err != nil {
				return "EXPORT_UNAVAILABLE"
			}
		}
		*first = false
		b, _ := json.Marshal(v)
		if _, err := w.Write(b); err != nil {
			return "EXPORT_UNAVAILABLE"
		}
	}
	return "OK"
}

func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

// Every publication boundary revalidates ALL retained inputs through the daemon.
func (a *App) checkPPExportLocked(p, act, device, target, base string) string {
	if base != "" {
		return a.ppCallLocked(
			"pppoe.compare", p, act,
			equipment.PPInput{DeviceID: device, BaseID: base, TargetID: target},
		).Code
	}
	return a.ppCallLocked(
		"pppoe.observation-get", p, act,
		equipment.PPInput{DeviceID: device, ObservationID: target},
	).Code
}

type ppFileOps struct {
	sync   func(int) error
	rename func(int, string, int, string, uint) error
	unlink func(int, string, int) error
	close  func(*os.File) error
}

func (o ppFileOps) fsync(fd int) error {
	if o.sync != nil {
		return o.sync(fd)
	}
	return unix.Fsync(fd)
}
func (o ppFileOps) move(a int, b string, c int, d string) error {
	if o.rename != nil {
		return o.rename(a, b, c, d, unix.RENAME_NOREPLACE)
	}
	return unix.Renameat2(a, b, c, d, unix.RENAME_NOREPLACE)
}
func (o ppFileOps) remove(fd int, p string) error {
	if o.unlink != nil {
		return o.unlink(fd, p, 0)
	}
	return unix.Unlinkat(fd, p, 0)
}

type ppLimitWriter struct {
	w io.Writer
	n int
}

func (w *ppLimitWriter) Write(b []byte) (int, error) {
	if len(b) > ppExportBytes-w.n {
		return 0, errors.New("export limit")
	}
	n, e := w.w.Write(b)
	w.n += n
	return n, e
}
func writePPExport(
	path string, produce func(io.Writer) string, publish func(func() string) string, ops ppFileOps,
) string {
	return writePPExportFile(path, ".json", produce, publish, ops)
}
func writePPExportFile(
	path, extension string,
	produce func(io.Writer) string,
	publish func(func() string) string,
	ops ppFileOps,
) string {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Ext(path) != extension {
		return "EXPORT_INVALID_DESTINATION"
	}
	dir, e := recoveryDir(filepath.Dir(path))
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	defer unix.Close(dir)
	var random [16]byte
	if _, e = rand.Read(random[:]); e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	stage := ".infradesktop-pppoe-" + hex.EncodeToString(random[:])
	if unix.Mkdirat(dir, stage, 0700) != nil {
		return "EXPORT_UNAVAILABLE"
	}
	source, e := unix.Openat(
		dir, stage, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0,
	)
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	defer unix.Close(source)
	var stageInfo unix.Stat_t
	if unix.Fstat(source, &stageInfo) != nil ||
		stageInfo.Uid != uint32(os.Getuid()) || stageInfo.Mode&0077 != 0 {
		return "EXPORT_UNAVAILABLE"
	}
	defer func() {
		var st unix.Stat_t
		if unix.Fstatat(dir, stage, &st, unix.AT_SYMLINK_NOFOLLOW) == nil &&
			st.Ino == stageInfo.Ino && st.Dev == stageInfo.Dev {
			_ = unix.Unlinkat(dir, stage, unix.AT_REMOVEDIR)
		}
	}()
	return writeStagedExportFile(dir, source, path, produce, publish, ops)
}

func writeStagedExportFile(
	dir, source int,
	path string,
	produce func(io.Writer) string,
	publish func(func() string) string,
	ops ppFileOps,
) string {
	flags := unix.O_WRONLY | unix.O_CREAT | unix.O_EXCL | unix.O_NOFOLLOW | unix.O_CLOEXEC
	fd, e := unix.Openat(source, "data", flags, 0600)
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	f := os.NewFile(uintptr(fd), "data")
	defer f.Close()
	var own unix.Stat_t
	if unix.Fstat(fd, &own) != nil {
		return "EXPORT_UNAVAILABLE"
	}
	same := func(d int, name string) bool {
		var st unix.Stat_t
		return unix.Fstatat(d, name, &st, unix.AT_SYMLINK_NOFOLLOW) == nil &&
			st.Ino == own.Ino && st.Dev == own.Dev
	}
	defer func() {
		if same(source, "data") {
			_ = ops.remove(source, "data")
		}
	}()
	if f.Chmod(0600) != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if c := produce(&ppLimitWriter{w: f}); c != "OK" {
		return c
	}
	if ops.fsync(fd) != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if ops.close != nil {
		e = ops.close(f)
	} else {
		e = f.Close()
	}
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	return publish(func() string {
		return publishStagedExport(source, dir, filepath.Base(path), same, ops)
	})
}

func publishStagedExport(
	source, dir int, baseName string, same func(int, string) bool, ops ppFileOps,
) string {
	e := ops.move(source, "data", dir, baseName)
	if errors.Is(e, unix.EEXIST) {
		return "EXPORT_EXISTS"
	}
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if ops.fsync(dir) != nil {
		if same(dir, baseName) && ops.remove(dir, baseName) == nil {
			_ = ops.fsync(dir)
			return "EXPORT_UNAVAILABLE"
		}
		return "EXPORT_DURABILITY_UNCERTAIN"
	}
	return "OK"
}
