package presentation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"io"
)

type pdExportSnapshot struct {
	Scope, ID, SourceObservation, SourceRow, Status, At, Quality string
	Count                                                        int
}

// Called with a.mu held. Only immutable local IDs enter page requests; all source
// identifiers are frozen before the picker and revalidated through publication.
func (a *App) freezePPDiagnosticsLocked(profile, activation, device, target string) ([]pdExportSnapshot, string) {
	original := a.ppCallLocked("pppoe.observation-get", profile, activation, equipment.PPInput{DeviceID: device, ObservationID: target})
	if original.Code != "OK" || original.Data == nil || original.Data.Current == nil {
		return nil, "EXPORT_UNAVAILABLE"
	}
	snapshots := []pdExportSnapshot{}
	for _, scope := range []string{equipment.PDCatalog, equipment.PDRadius, equipment.PDPing} {
		response := a.ppCallLocked("pppoe.diagnostic-current", profile, activation, equipment.PPInput{DeviceID: device, Scope: scope})
		if response.Code != "OK" {
			return nil, response.Code
		}
		if response.Data == nil {
			return nil, "EXPORT_UNAVAILABLE"
		}
		snapshot := pdExportSnapshot{Scope: scope, Status: "not_collected"}
		if diagnostic := response.Data.Diagnostic; diagnostic != nil {
			snapshot.ID = diagnostic.ID
			snapshot.SourceObservation = diagnostic.SourceObservation
			snapshot.SourceRow = diagnostic.SourceRow
			snapshot.Status = "context_mismatch"
			if equipment.PDCompatibleExport(original.Data.Current, diagnostic) {
				snapshot.Status = "available"
				snapshot.At = diagnostic.Provenance.Ended
				snapshot.Count = diagnostic.Count
				snapshot.Quality = diagnostic.Quality.Status
			}
		}
		snapshots = append(snapshots, snapshot)
	}
	return snapshots, "OK"
}
func (a *App) checkPPDiagnosticsLocked(profile, activation, device string, snapshots []pdExportSnapshot) string {
	for _, snapshot := range snapshots {
		response := a.ppCallLocked("pppoe.diagnostic-current", profile, activation, equipment.PPInput{DeviceID: device, Scope: snapshot.Scope})
		if response.Code != "OK" || response.Data == nil {
			return "EXPORT_UNAVAILABLE"
		}
		current := response.Data.Diagnostic
		if current == nil {
			if snapshot.ID != "" {
				return "OBSERVATION_UNAVAILABLE"
			}
			continue
		}
		if current.ID != snapshot.ID || current.SourceObservation != snapshot.SourceObservation || current.SourceRow != snapshot.SourceRow {
			return "OBSERVATION_UNAVAILABLE"
		}
	}
	return "OK"
}
func pdExportMetadata(snapshots []pdExportSnapshot, key []byte, alias func(string, string) string) []map[string]any {
	result := []map[string]any{}
	for _, snapshot := range snapshots {
		meta := map[string]any{"scope": snapshot.Scope, "status": snapshot.Status}
		if snapshot.Status == "available" {
			meta["diagnostic"] = alias("diagnostic", pdExportToken(key, "diagnostic", snapshot.ID))
			meta["observed_at"] = snapshot.At
			meta["count"] = snapshot.Count
			meta["quality"] = snapshot.Quality
			meta["atomic"] = false
			if snapshot.SourceObservation != "" {
				meta["source_observation"] = alias("observation", pdExportToken(key, "observation", snapshot.SourceObservation))
				meta["source_row"] = alias("row", pdExportToken(key, "row", snapshot.SourceObservation+snapshot.SourceRow))
			}
		}
		result = append(result, meta)
	}
	return result
}
func pdExportToken(key []byte, kind, value string) string {
	if value == "" {
		return ""
	}
	m := hmac.New(sha256.New, key)
	_, _ = m.Write([]byte(kind + "\x00" + value))
	return hex.EncodeToString(m.Sum(nil))
}

func (a *App) writePPDiagnosticExport(w io.Writer, p, act string, in equipment.PPInput, snapshots []pdExportSnapshot, generation uint64, key []byte, alias func(string, string) string, aliasCount func() int, check func() string) string {
	if _, err := io.WriteString(w, `,"diagnostic_rows":[`); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	first := true
	for _, snapshot := range snapshots {
		if snapshot.Status != "available" {
			continue
		}
		in.Scope = snapshot.Scope
		in.DiagnosticID = snapshot.ID
		in.ExportPart = "diagnostics"
		in.Cursor = ""
		seen := map[string]bool{}
		for page := 0; page <= 128; page++ {
			if code := check(); code != "OK" {
				return code
			}
			a.mu.Lock()
			if generation != a.ppGeneration {
				a.mu.Unlock()
				return "PROFILE_CHANGED"
			}
			result := a.ppCallLocked("pppoe.export-page", p, act, in)
			a.mu.Unlock()
			if result.Code != "OK" {
				return result.Code
			}
			if result.Data == nil {
				return "EXPORT_UNAVAILABLE"
			}
			for _, row := range result.Data.ExportDiagnostics {
				row.ID = alias("diagnostic-row", row.ID)
				row.RemoteID = alias("diagnostic-remote", row.RemoteID)
				row.Name = alias("diagnostic-name", row.Name)
				row.Parent = alias("diagnostic-name", row.Parent)
				row.ParentID = alias("diagnostic-row", row.ParentID)
				row.Address = alias("address", row.Address)
				if aliasCount() > 16386 {
					return "EXPORT_UNAVAILABLE"
				}
				record := struct {
					Diagnostic string          `json:"diagnostic"`
					Row        equipment.PDRow `json:"row"`
				}{Diagnostic: alias("diagnostic", pdExportToken(key, "diagnostic", snapshot.ID)), Row: row}
				if !first {
					if _, err := io.WriteString(w, ","); err != nil {
						return "EXPORT_UNAVAILABLE"
					}
				}
				first = false
				body, err := json.Marshal(record)
				if err != nil {
					return "EXPORT_UNAVAILABLE"
				}
				if _, err = w.Write(body); err != nil {
					return "EXPORT_UNAVAILABLE"
				}
			}
			next := result.Data.Next
			if next == "" {
				break
			}
			if seen[next] || page == 128 {
				return "EXPORT_UNAVAILABLE"
			}
			seen[next] = true
			in.Cursor = next
		}
	}
	if _, err := io.WriteString(w, "]"); err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	return "OK"
}
