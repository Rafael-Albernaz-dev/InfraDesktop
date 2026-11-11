package presentation

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/unix"

	"infradesktop/internal/modules/equipment"

	"infradesktop/internal/testsupport/sshfixture"

	"os"
	"path/filepath"
	"strings"

	"testing"
)

func (s *ppExportJourney) assertDiagnosticGroups(t *testing.T) {
	if len(s.file["observations"].([]any)) != 2 || len(s.file["deltas"].([]any)) == 0 || s.file["limits"] == nil {
		t.Fatal("export lacks useful comparison context")
	}
	if len(s.file["diagnostics"].([]any)) != 3 || len(s.file["diagnostic_rows"].([]any)) != 10 {
		t.Fatal("missing sanitized diagnostic scopes", s.file["diagnostics"], s.file["diagnostic_rows"])
	}
	diagnosticGroups := map[string]bool{}
	for _, rawMeta := range s.file["diagnostics"].([]any) {
		meta := rawMeta.(map[string]any)
		if meta["status"] != "available" {
			t.Fatal("matching scope excluded", meta)
		}
		diagnosticGroups[meta["diagnostic"].(string)] = true
	}
	for _, rawRow := range s.file["diagnostic_rows"].([]any) {
		if !diagnosticGroups[rawRow.(map[string]any)["diagnostic"].(string)] {
			t.Fatal("diagnostic row lost frozen group")
		}
	}
	for _, id := range s.diagnosticIDs {
		if bytes.Contains(s.raw, []byte(id)) {
			t.Fatal("raw diagnostic ID exported")
		}
	}

}

func (s *ppExportJourney) assertObservationAliases(t *testing.T) {
	groups := map[string]string{}
	for i, value := range s.file["observations"].([]any) {
		meta := value.(map[string]any)
		want := []string{"base", "target"}[i]
		if meta["role"] != want {
			t.Fatal("metadata lacks explicit role", meta)
		}
		alias, ok := meta["observation"].(string)
		if !ok || alias == "" {
			t.Fatal("metadata lacks observation alias")
		}
		groups[alias] = want
	}
	for _, value := range s.file["rows"].([]any) {
		if groups[value.(map[string]any)["observation"].(string)] == "" {
			t.Fatal("row group cannot be identified")
		}
	}
	rowAliases := map[string]bool{}
	for _, value := range s.file["rows"].([]any) {
		rowAliases[value.(map[string]any)["row"].(string)] = true
	}
	for _, value := range s.file["deltas"].([]any) {
		d := value.(map[string]any)
		for _, k := range []string{"base_row", "target_row"} {
			if id, ok := d[k].(string); ok && !rowAliases[id] {
				t.Fatal("delta cannot be related to exported row")
			}
		}
	}
	s.assertPingSourceAliases(t, groups, rowAliases)

}

func (s *ppExportJourney) assertSanitizedAndTargetOnly(t *testing.T) {
	for _, canary := range []string{"synthetic-", "198.51.", "02:00:", s.peer.Fingerprint(), sshfixture.Password, s.d.ID, s.a.ID, s.b.ID,
		"export_key", "provenance"} {
		if bytes.Contains(s.raw, []byte(canary)) {
			t.Fatal("export leaked", canary)
		}
	}
	info, _ := os.Stat(s.destination)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	targetOnly := filepath.Join(s.root, "target-only.json")
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return targetOnly, nil }
	if code := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, ""); code != "OK" {
		t.Fatal(code)
	}
	singleRaw, _ := os.ReadFile(targetOnly)
	var single map[string]any
	if json.Unmarshal(singleRaw, &single) != nil {
		t.Fatal("target-only JSON")
	}
	metas := single["observations"].([]any)
	if len(metas) != 1 || metas[0].(map[string]any)["role"] != "target" {
		t.Fatal("target-only role")
	}
	for _, value := range single["rows"].([]any) {
		if value.(map[string]any)["observation"] != metas[0].(map[string]any)["observation"] {
			t.Fatal("target-only group")
		}
	}
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return s.destination, nil }
	if c := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, s.a.ID); c != "EXPORT_EXISTS" {
		t.Fatal("overwrite", c)
	}
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return "", nil }
	if c := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, s.a.ID); c != "EXPORT_CANCELLED" {
		t.Fatal(c)
	}
	if s.peer.Auth.Load() != s.auths || s.peer.Commands.Load() != s.commands {
		t.Fatal("local export connected before replacement tests")
	}

}

func (s *ppExportJourney) readOperationalCSV(t *testing.T) ([]byte, [][]string, string) {
	csvPath := filepath.Join(s.root, "operational.csv")
	s.app.saveDialog = func(_ context.Context, o runtime.SaveDialogOptions) (string, error) {
		if !strings.Contains(o.Title, "dados sensíveis") || !strings.Contains(o.Title, "assinantes") {
			t.Fatal("missing native sensitive-data warning", o.Title)
		}
		return csvPath, nil
	}
	if code := s.app.ExportPPPoEOperational(s.profile, s.activation, s.d.ID, s.b.ID); code != "OK" {
		t.Fatal("operational export", code)
	}
	csvRaw, e := os.ReadFile(csvPath)
	if e != nil {
		t.Fatal(e)
	}
	records, e := csv.NewReader(bytes.NewReader(csvRaw)).ReadAll()
	if e != nil {
		t.Fatal(e)
	}

	return csvRaw, records, csvPath
}

func (s *ppExportJourney) assertOperationalCSV(t *testing.T, csvRaw []byte, records [][]string, csvPath string) {
	expected := []string{"login", "ip", "caller_id", "estado", "uptime_min_ns", "uptime_max_ns", "vlan", "servico", "perfil",
		"upload_limite_bit_s", "download_limite_bit_s", "contexto", "estado_secret"}
	if strings.Join(records[0], "|") != strings.Join(expected, "|") || len(records) != s.b.Count+1 ||
		!strings.HasPrefix(records[1][0], "synthetic-") {
		t.Fatal("operational columns/rows/identity", len(records))
	}
	for _, row := range records[1:] {
		if row[11] != "não incluído por limite" || strings.Join(row[6:11], "") != "" {
			t.Fatal("CSV omission became unknown zero or unmarked blank")
		}
	}
	if st, e := os.Stat(csvPath); e != nil || st.Mode().Perm() != 0600 {
		t.Fatal("operational permissions", e)
	}
	if strings.Contains(string(csvRaw), "password") || strings.Contains(string(csvRaw), "fingerprint") ||
		strings.Contains(string(csvRaw), "127.0.0.1") {
		t.Fatal("forbidden operational metadata")
	}

}

func (s *ppExportJourney) assertOperationalGuards(t *testing.T) {
	if code := s.app.ExportPPPoEOperational(s.profile, s.activation, s.d.ID, s.b.ID); code != "EXPORT_EXISTS" {
		t.Fatal("operational overwrite allowed", code)
	}
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return "", nil }
	if code := s.app.ExportPPPoEOperational(s.profile, s.activation, s.d.ID, s.b.ID); code != "EXPORT_CANCELLED" {
		t.Fatal("operational cancel", code)
	}
	finalCSV := filepath.Join(s.root, "stale.csv")
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return finalCSV, nil }
	s.app.ppExportFS = ppFileOps{sync: func(fd int) error { s.app.mu.Lock(); s.app.ppGeneration++; s.app.mu.Unlock(); return unix.Fsync(fd) }}
	if code := s.app.ExportPPPoEOperational(s.profile, s.activation, s.d.ID, s.b.ID); code != "PROFILE_CHANGED" {
		t.Fatal("stale operational publish", code)
	}
	if _, e := os.Stat(finalCSV); !os.IsNotExist(e) {
		t.Fatal("stale csv published")
	}
	s.app.ppExportFS = ppFileOps{}
	if s.peer.Auth.Load() != s.auths || s.peer.Commands.Load() != s.commands {
		t.Fatal("operational export caused remote work")
	}

}

func (s *ppExportJourney) assertReplacementAndLogout(t *testing.T) {
	var e error
	// A deliberate refresh during the picker cannot mix a new current into pages.
	replaced := filepath.Join(s.root, "diagnostic-replaced-picker.json")
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) {
		s.collectDiagnostic(t, equipment.PDCatalog)
		return replaced, nil
	}
	if code := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, s.a.ID); code != "OBSERVATION_UNAVAILABLE" {
		t.Fatal("picker replacement accepted", code)
	}
	if _, err := os.Stat(replaced); !os.IsNotExist(err) {
		t.Fatal("picker replacement published file", err)
	}
	// Repeat at the final file-sync boundary, before publication revalidation.
	replacedFinal := filepath.Join(s.root, "diagnostic-replaced-final.json")
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return replacedFinal, nil }
	mutated := false
	s.app.ppExportFS = ppFileOps{sync: func(fd int) error {
		if !mutated {
			mutated = true
			s.collectDiagnostic(t, equipment.PDCatalog)
		}
		return unix.Fsync(fd)
	}}
	if code := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, s.a.ID); code != "OBSERVATION_UNAVAILABLE" {
		t.Fatal("final replacement accepted", code)
	}
	if _, err := os.Stat(replacedFinal); !os.IsNotExist(err) {
		t.Fatal("final replacement published file", err)
	}
	s.app.ppExportFS = ppFileOps{}
	s.auths, s.commands = s.peer.Auth.Load(), s.peer.Commands.Load()
	late := filepath.Join(s.root, "late.json")
	s.app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { s.app.Logout(); return late, nil }
	if c := s.app.ExportPPPoE(s.profile, s.activation, s.d.ID, s.b.ID, s.a.ID); c != "PROFILE_CHANGED" {
		t.Fatal(c)
	}
	if _, e = os.Stat(late); !os.IsNotExist(e) {
		t.Fatal("context created file")
	}
	if s.peer.Auth.Load() != s.auths || s.peer.Commands.Load() != s.commands {
		t.Fatal("local export connected")
	}
}

func (s *ppExportJourney) assertPingSourceAliases(t *testing.T, groups map[string]string, rowAliases map[string]bool) {
	for _, value := range s.file["diagnostics"].([]any) {
		meta := value.(map[string]any)
		if meta["scope"] == equipment.PDPing {
			if groups[meta["source_observation"].(string)] != "target" || !rowAliases[meta["source_row"].(string)] {
				t.Fatal("ping source detached from exported observation/row", meta)
			}
		}
	}

}
