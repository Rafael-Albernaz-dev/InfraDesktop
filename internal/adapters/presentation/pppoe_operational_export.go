package presentation

import (
	"encoding/csv"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"infradesktop/internal/modules/equipment"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// CSV cells must remain literal text when opened by spreadsheet applications.
func ppCSVCell(value string) string {
	trim := strings.TrimLeftFunc(value, unicode.IsSpace)
	if strings.ContainsAny(value, "\t\r\n") || len(trim) > 0 && strings.ContainsAny(trim[:1], "=+-@") {
		return "'" + value
	}
	return value
}

// A nominal export of a single retained observation. The UI supplies IDs only;
// the native dialog owns the destination and discloses subscriber sensitivity.
func (a *App) ExportPPPoEOperational(p, act, device, observation string) string {
	a.mu.Lock()
	if a.ppExporting {
		a.mu.Unlock()
		return "BUSY"
	}
	a.ppExporting = true
	generation := a.ppGeneration
	picker, ctx := a.saveDialog, a.ctx
	initial := a.checkPPExportLocked(p, act, device, observation, "")
	a.mu.Unlock()
	defer func() { a.mu.Lock(); a.ppExporting = false; a.mu.Unlock() }()
	if initial != "OK" {
		return initial
	}
	path, e := picker(ctx, runtime.SaveDialogOptions{Title: "Exportar CSV operacional — dados sensíveis de assinantes", DefaultFilename: "pppoe-operacional.csv", Filters: []runtime.FileFilter{{DisplayName: "CSV operacional (dados sensíveis)", Pattern: "*.csv"}}})
	if e != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if path == "" {
		return "EXPORT_CANCELLED"
	}
	check := func() string {
		a.mu.Lock()
		defer a.mu.Unlock()
		if generation != a.ppGeneration {
			return "PROFILE_CHANGED"
		}
		return a.checkPPExportLocked(p, act, device, observation, "")
	}
	produce := func(w io.Writer) string {
		if c := check(); c != "OK" {
			return c
		}
		out := csv.NewWriter(w)
		if out.Write([]string{"login", "ip", "caller_id", "estado", "uptime_min_ns", "uptime_max_ns", "vlan", "servico", "perfil", "upload_limite_bit_s", "download_limite_bit_s", "contexto", "estado_secret"}) != nil {
			return "EXPORT_UNAVAILABLE"
		}
		in := equipment.PPInput{DeviceID: device, ObservationID: observation}
		seen := map[string]bool{}
		count := 0
		for pages := 0; pages <= equipment.PPMaxRows/equipment.PageSize; pages++ {
			a.mu.Lock()
			if generation != a.ppGeneration {
				a.mu.Unlock()
				return "PROFILE_CHANGED"
			}
			r := a.ppCallLocked("pppoe.sessions-list", p, act, in)
			a.mu.Unlock()
			if r.Code != "OK" {
				return r.Code
			}
			if r.Data == nil {
				return "EXPORT_UNAVAILABLE"
			}
			for _, row := range r.Data.Rows {
				count++
				if count > equipment.PPMaxRows {
					return "EXPORT_UNAVAILABLE"
				}
				state := row.State
				if state == "" {
					state = "ONLINE"
				}
				values := []string{row.Login, row.Address, row.Caller, state, "", "", "", "", "", "", "", "indisponível", "unavailable"}
				if r.Data.Current != nil && r.Data.Current.ContextOmitted {
					values[11] = "não incluído por limite"
				}
				if row.Uptime.Known {
					values[4] = strconv.FormatInt(row.Uptime.Min, 10)
					values[5] = strconv.FormatInt(row.Uptime.Max, 10)
				}
				if c := row.Context; c != nil {
					values[11] = "incluído"
					if c.VLAN != nil {
						values[6] = strconv.Itoa(*c.VLAN)
					}
					values[7] = c.Service
					if c.SecretState != "" {
						values[12] = c.SecretState
					}
					if c.SecretState == "active" {
						values[8] = c.Profile
					}
					values[9] = c.UploadLimit
					values[10] = c.DownloadLimit
				}
				for i := range values {
					values[i] = ppCSVCell(values[i])
				}
				if out.Write(values) != nil {
					return "EXPORT_UNAVAILABLE"
				}
			}
			if r.Data.Next == "" {
				out.Flush()
				if out.Error() != nil {
					return "EXPORT_UNAVAILABLE"
				}
				return "OK"
			}
			if seen[r.Data.Next] {
				return "EXPORT_UNAVAILABLE"
			}
			seen[r.Data.Next] = true
			in.Cursor = r.Data.Next
		}
		return "EXPORT_UNAVAILABLE"
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
	return writePPExportFile(path, ".csv", produce, publish, a.ppExportFS)
}
