package presentation

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/unix"
	"infradesktop/internal/modules/equipment"
	"io"
	"os"
	"path/filepath"
)

const ppExportBytes = 2 * 1024 * 1024

// ExportPPPoE accepts only immutable product IDs. No JS-supplied path, key or bytes.
func (a *App) ExportPPPoE(p, act, device, target, base string) string {
	a.mu.Lock()
	if a.ppExporting {
		a.mu.Unlock()
		return "BUSY"
	}
	a.ppExporting = true
	generation := a.ppGeneration
	picker, ctx := a.saveDialog, a.ctx
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.ppExporting = false; a.mu.Unlock() }()
	a.mu.Lock()
	initial := a.checkPPExportLocked(p, act, device, target, base)
	var diagnosticSnapshots []pdExportSnapshot
	if initial == "OK" {
		diagnosticSnapshots, initial = a.freezePPDiagnosticsLocked(p, act, device, target)
	}
	a.mu.Unlock()
	if initial != "OK" {
		return initial
	}
	path, e := picker(ctx, runtime.SaveDialogOptions{Title: "Exportar comparação PPPoE sanitizada", DefaultFilename: "pppoe-sanitizado.json", Filters: []runtime.FileFilter{{DisplayName: "JSON sanitizado", Pattern: "*.json"}}})
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if path == "" {
		return "EXPORT_CANCELLED"
	}
	var key [32]byte
	if _, e = rand.Read(key[:]); e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	in := equipment.PPInput{DeviceID: device, TargetID: target, BaseID: base, ExportKey: base64.RawStdEncoding.EncodeToString(key[:])}
	var nonce [8]byte
	if _, e = rand.Read(nonce[:]); e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	aliases := map[string]string{}
	alias := func(kind, token string) string {
		if token == "" {
			return ""
		}
		k := kind + token
		if v := aliases[k]; v != "" {
			return v
		}
		v := kind + "-" + hex.EncodeToString(nonce[:]) + "-" + itoa(len(aliases)+1)
		aliases[k] = v
		return v
	}
	// Each page and the final publish check use the authenticated daemon. No mutex
	// spans the picker or the entire export; local logout can invalidate generation.
	check := func() string {
		a.mu.Lock()
		defer a.mu.Unlock()
		if generation != a.ppGeneration {
			return "PROFILE_CHANGED"
		}
		if code := a.checkPPExportLocked(p, act, device, target, base); code != "OK" {
			return code
		}
		return a.checkPPDiagnosticsLocked(p, act, device, diagnosticSnapshots)
	}
	producer := func(w io.Writer) string {
		if c := check(); c != "OK" {
			return c
		}
		// Explicit allowlist: no provenance blob, free names, endpoints or remote IDs.
		metas := []map[string]any{}
		for index, id := range []string{base, target} {
			if id == "" {
				continue
			}
			r := a.ppCall("pppoe.observation-get", p, act, equipment.PPInput{DeviceID: device, ObservationID: id})
			if r.Code != "OK" || r.Data == nil || r.Data.Current == nil {
				return r.Code
			}
			o := r.Data.Current
			m := hmac.New(sha256.New, key[:])
			m.Write([]byte("observation\x00" + id))
			token := hex.EncodeToString(m.Sum(nil))
			role := []string{"base", "target"}[index]
			metas = append(metas, map[string]any{"role": role, "observation": alias("observation", token), "observed_at": o.Provenance.Ended, "count": o.Count, "quality": o.Quality.Status, "atomic": false})
		}
		header, _ := json.Marshal(map[string]any{"profile": "pppoe-sanitized-v1", "observations": metas, "diagnostics": pdExportMetadata(diagnosticSnapshots, key[:], alias), "limits": map[string]int{"rows": equipment.PPMaxRows, "wire_bytes": equipment.PPWireBytes, "observation_bytes": equipment.PPObservationBytes}})
		if _, e := w.Write(header[:len(header)-1]); e != nil {
			return "EXPORT_UNAVAILABLE"
		}
		for _, part := range []string{"rows", "comparison"} {
			field := "rows"
			if part == "comparison" {
				field = "deltas"
			}
			if _, e := io.WriteString(w, ",\""+field+"\":["); e != nil {
				return "EXPORT_UNAVAILABLE"
			}
			first := true
			in.Cursor = ""
			in.ExportPart = part
			seen := map[string]bool{}
			if part == "rows" || base != "" {
				for pages := 0; pages <= 256; pages++ {
					a.mu.Lock()
					if generation != a.ppGeneration {
						a.mu.Unlock()
						return "PROFILE_CHANGED"
					}
					r := a.ppCallLocked("pppoe.export-page", p, act, in)
					a.mu.Unlock()
					if r.Code != "OK" {
						return r.Code
					}
					if r.Data == nil {
						return "EXPORT_UNAVAILABLE"
					}
					values := []any{}
					for _, row := range r.Data.ExportRows {
						row.Observation = alias("observation", row.Observation)
						row.Row = alias("row", row.Row)
						row.Login = alias("login", row.Login)
						row.Address = alias("address", row.Address)
						row.Caller = alias("caller", row.Caller)
						values = append(values, row)
					}
					for _, d := range r.Data.ExportDeltas {
						d.BaseRow = alias("row", d.BaseRow)
						d.TargetRow = alias("row", d.TargetRow)
						values = append(values, d)
					}
					if len(aliases) > 16386 {
						return "EXPORT_UNAVAILABLE"
					}
					for _, v := range values {
						if !first {
							if _, e := io.WriteString(w, ","); e != nil {
								return "EXPORT_UNAVAILABLE"
							}
						}
						first = false
						b, _ := json.Marshal(v)
						if _, e := w.Write(b); e != nil {
							return "EXPORT_UNAVAILABLE"
						}
					}
					next := r.Data.Next
					if next == "" {
						break
					}
					if seen[next] || pages == 256 {
						return "EXPORT_UNAVAILABLE"
					}
					seen[next] = true
					in.Cursor = next
				}
			}
			if _, e := io.WriteString(w, "]"); e != nil {
				return "EXPORT_UNAVAILABLE"
			}
		}
		if code := a.writePPDiagnosticExport(w, p, act, in, diagnosticSnapshots, generation, key[:], alias, func() int { return len(aliases) }, check); code != "OK" {
			return code
		}
		if _, e := io.WriteString(w, "}\n"); e != nil {
			return "EXPORT_UNAVAILABLE"
		}
		return "OK"
	}
	publish := func(fn func() string) string {
		if c := check(); c != "OK" {
			return c
		}
		a.mu.Lock()
		defer a.mu.Unlock()
		if generation != a.ppGeneration {
			return "PROFILE_CHANGED"
		}
		return fn()
	}
	return writePPExport(path, producer, publish, a.ppExportFS)
}
func itoa(n int) string { b, _ := json.Marshal(n); return string(b) }

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
func writePPExport(path string, produce func(io.Writer) string, publish func(func() string) string, ops ppFileOps) string {
	return writePPExportFile(path, ".json", produce, publish, ops)
}
func writePPExportFile(path, extension string, produce func(io.Writer) string, publish func(func() string) string, ops ppFileOps) string {
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
	source, e := unix.Openat(dir, stage, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	defer unix.Close(source)
	var stageInfo unix.Stat_t
	if unix.Fstat(source, &stageInfo) != nil || stageInfo.Uid != uint32(os.Getuid()) || stageInfo.Mode&0077 != 0 {
		return "EXPORT_UNAVAILABLE"
	}
	defer func() {
		var st unix.Stat_t
		if unix.Fstatat(dir, stage, &st, unix.AT_SYMLINK_NOFOLLOW) == nil && st.Ino == stageInfo.Ino && st.Dev == stageInfo.Dev {
			_ = unix.Unlinkat(dir, stage, unix.AT_REMOVEDIR)
		}
	}()
	fd, e := unix.Openat(source, "data", unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
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
		return unix.Fstatat(d, name, &st, unix.AT_SYMLINK_NOFOLLOW) == nil && st.Ino == own.Ino && st.Dev == own.Dev
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
		e := ops.move(source, "data", dir, filepath.Base(path))
		if errors.Is(e, unix.EEXIST) {
			return "EXPORT_EXISTS"
		}
		if e != nil {
			return "EXPORT_UNAVAILABLE"
		}
		if ops.fsync(dir) != nil {
			if same(dir, filepath.Base(path)) && ops.remove(dir, filepath.Base(path)) == nil {
				_ = ops.fsync(dir)
				return "EXPORT_UNAVAILABLE"
			}
			return "EXPORT_DURABILITY_UNCERTAIN"
		}
		return "OK"
	})
}

// Every publication boundary revalidates ALL retained inputs through the daemon.
func (a *App) checkPPExportLocked(p, act, device, target, base string) string {
	if base != "" {
		return a.ppCallLocked("pppoe.compare", p, act, equipment.PPInput{DeviceID: device, BaseID: base, TargetID: target}).Code
	}
	return a.ppCallLocked("pppoe.observation-get", p, act, equipment.PPInput{DeviceID: device, ObservationID: target}).Code
}
