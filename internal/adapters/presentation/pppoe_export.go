package presentation

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"infradesktop/internal/modules/equipment"
	"io"
)

const ppExportBytes = 2 * 1024 * 1024

type ppExportSession struct {
	app        *App
	profile    string
	activation string
	device     string
	target     string
	base       string
	generation uint64
	key        [32]byte
	nonce      [8]byte
	aliases    map[string]string
	snapshots  []pdExportSnapshot
	input      equipment.PPInput
}

// ExportPPPoE accepts only immutable product IDs. No JS-supplied path, key or bytes.
func (a *App) ExportPPPoE(p, act, device, target, base string) string {
	sess, code := a.beginPPExport(p, act, device, target, base)
	if code != "OK" {
		return code
	}
	defer func() { a.mu.Lock(); a.ppExporting = false; a.mu.Unlock() }()
	path, code := a.pickPPExportPath()
	if code != "OK" {
		return code
	}
	if code = sess.initCrypto(); code != "OK" {
		return code
	}
	return writePPExport(path, sess.produce, sess.publish, a.ppExportFS)
}

func (a *App) beginPPExport(
	p, act, device, target, base string,
) (*ppExportSession, string) {
	a.mu.Lock()
	if a.ppExporting {
		a.mu.Unlock()
		return nil, "BUSY"
	}
	a.ppExporting = true
	generation := a.ppGeneration
	initial := a.checkPPExportLocked(p, act, device, target, base)
	var snapshots []pdExportSnapshot
	if initial == "OK" {
		snapshots, initial = a.freezePPDiagnosticsLocked(p, act, device, target)
	}
	a.mu.Unlock()
	if initial != "OK" {
		a.mu.Lock()
		a.ppExporting = false
		a.mu.Unlock()
		return nil, initial
	}
	return &ppExportSession{
		app:        a,
		profile:    p,
		activation: act,
		device:     device,
		target:     target,
		base:       base,
		generation: generation,
		aliases:    map[string]string{},
		snapshots:  snapshots,
	}, "OK"
}

func (a *App) pickPPExportPath() (string, string) {
	a.mu.Lock()
	picker, ctx := a.saveDialog, a.ctx
	a.mu.Unlock()
	path, err := picker(ctx, runtime.SaveDialogOptions{
		Title:           "Exportar comparação PPPoE sanitizada",
		DefaultFilename: "pppoe-sanitizado.json",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON sanitizado", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", "EXPORT_UNAVAILABLE"
	}
	if path == "" {
		return "", "EXPORT_CANCELLED"
	}
	return path, "OK"
}

func (s *ppExportSession) initCrypto() string {
	if _, err := rand.Read(s.key[:]); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if _, err := rand.Read(s.nonce[:]); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	s.input = equipment.PPInput{
		DeviceID:  s.device,
		TargetID:  s.target,
		BaseID:    s.base,
		ExportKey: base64.RawStdEncoding.EncodeToString(s.key[:]),
	}
	return "OK"
}

func (s *ppExportSession) alias(kind, token string) string {
	if token == "" {
		return ""
	}
	k := kind + token
	if v := s.aliases[k]; v != "" {
		return v
	}
	v := kind + "-" + hex.EncodeToString(s.nonce[:]) + "-" + itoa(len(s.aliases)+1)
	s.aliases[k] = v
	return v
}

func (s *ppExportSession) check() string {
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	if s.generation != s.app.ppGeneration {
		return "PROFILE_CHANGED"
	}
	if code := s.app.checkPPExportLocked(
		s.profile, s.activation, s.device, s.target, s.base,
	); code != "OK" {
		return code
	}
	return s.app.checkPPDiagnosticsLocked(s.profile, s.activation, s.device, s.snapshots)
}

func (s *ppExportSession) publish(fn func() string) string {
	if c := s.check(); c != "OK" {
		return c
	}
	s.app.mu.Lock()
	defer s.app.mu.Unlock()
	if s.generation != s.app.ppGeneration {
		return "PROFILE_CHANGED"
	}
	return fn()
}

func (s *ppExportSession) produce(w io.Writer) string {
	if c := s.check(); c != "OK" {
		return c
	}
	if c := s.writeHeader(w); c != "OK" {
		return c
	}
	for _, part := range []string{"rows", "comparison"} {
		if c := s.writeExportPart(w, part); c != "OK" {
			return c
		}
	}
	code := s.app.writePPDiagnosticExport(
		w, s.profile, s.activation, s.input, s.snapshots, s.generation,
		s.key[:], s.alias, func() int { return len(s.aliases) }, s.check,
	)
	if code != "OK" {
		return code
	}
	if _, err := io.WriteString(w, "}\n"); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	return "OK"
}

func (s *ppExportSession) writeHeader(w io.Writer) string {
	metas := []map[string]any{}
	for index, id := range []string{s.base, s.target} {
		if id == "" {
			continue
		}
		r := s.app.ppCall(
			"pppoe.observation-get", s.profile, s.activation,
			equipment.PPInput{DeviceID: s.device, ObservationID: id},
		)
		if r.Code != "OK" || r.Data == nil || r.Data.Current == nil {
			return r.Code
		}
		o := r.Data.Current
		m := hmac.New(sha256.New, s.key[:])
		m.Write([]byte("observation\x00" + id))
		token := hex.EncodeToString(m.Sum(nil))
		role := []string{"base", "target"}[index]
		metas = append(metas, map[string]any{
			"role":        role,
			"observation": s.alias("observation", token),
			"observed_at": o.Provenance.Ended,
			"count":       o.Count,
			"quality":     o.Quality.Status,
			"atomic":      false,
		})
	}
	header, _ := json.Marshal(map[string]any{
		"profile":      "pppoe-sanitized-v1",
		"observations": metas,
		"diagnostics":  pdExportMetadata(s.snapshots, s.key[:], s.alias),
		"limits": map[string]int{
			"rows":              equipment.PPMaxRows,
			"wire_bytes":        equipment.PPWireBytes,
			"observation_bytes": equipment.PPObservationBytes,
		},
	})
	if _, err := w.Write(header[:len(header)-1]); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	return "OK"
}

func (s *ppExportSession) writeExportPart(w io.Writer, part string) string {
	field := "rows"
	if part == "comparison" {
		field = "deltas"
	}
	if _, err := io.WriteString(w, ",\""+field+"\":["); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if part == "rows" || s.base != "" {
		if code := s.streamExportPages(w, part); code != "OK" {
			return code
		}
	}
	if _, err := io.WriteString(w, "]"); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	return "OK"
}

func (s *ppExportSession) streamExportPages(w io.Writer, part string) string {
	first := true
	s.input.Cursor = ""
	s.input.ExportPart = part
	seen := map[string]bool{}
	for pages := 0; pages <= 256; pages++ {
		values, next, code := s.fetchExportPageValues()
		if code != "OK" {
			return code
		}
		if code = writeJSONValues(w, values, &first); code != "OK" {
			return code
		}
		if next == "" {
			return "OK"
		}
		if seen[next] || pages == 256 {
			return "EXPORT_UNAVAILABLE"
		}
		seen[next] = true
		s.input.Cursor = next
	}
	return "OK"
}

func (s *ppExportSession) fetchExportPageValues() ([]any, string, string) {
	s.app.mu.Lock()
	if s.generation != s.app.ppGeneration {
		s.app.mu.Unlock()
		return nil, "", "PROFILE_CHANGED"
	}
	r := s.app.ppCallLocked("pppoe.export-page", s.profile, s.activation, s.input)
	s.app.mu.Unlock()
	if r.Code != "OK" {
		return nil, "", r.Code
	}
	if r.Data == nil {
		return nil, "", "EXPORT_UNAVAILABLE"
	}
	values := s.collectAliasedExportValues(r.Data)
	if len(s.aliases) > 16386 {
		return nil, "", "EXPORT_UNAVAILABLE"
	}
	return values, r.Data.Next, "OK"
}

func (s *ppExportSession) collectAliasedExportValues(data *equipment.PPView) []any {
	values := []any{}
	for _, row := range data.ExportRows {
		row.Observation = s.alias("observation", row.Observation)
		row.Row = s.alias("row", row.Row)
		row.Login = s.alias("login", row.Login)
		row.Address = s.alias("address", row.Address)
		row.Caller = s.alias("caller", row.Caller)
		values = append(values, row)
	}
	for _, d := range data.ExportDeltas {
		d.BaseRow = s.alias("row", d.BaseRow)
		d.TargetRow = s.alias("row", d.TargetRow)
		values = append(values, d)
	}
	return values
}
