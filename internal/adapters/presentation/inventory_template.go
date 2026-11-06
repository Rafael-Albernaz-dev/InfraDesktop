package presentation

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"io"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func inventoryTemplate(format string) ([]byte, error) {
	fields := []string{"Nome", "IP", "Porta", "Papel", "Bloco", "Credencial"}
	values := []string{"AP-LESTE-1", "192.0.2.31", "22", "AP", "Torre Leste", "leitura-radio"}
	switch format {
	case "csv":
		var b bytes.Buffer
		w := csv.NewWriter(&b)
		if err := w.Write(fields); err != nil {
			return nil, err
		}
		if err := w.Write(values); err != nil {
			return nil, err
		}
		w.Flush()
		return b.Bytes(), w.Error()
	case "json":
		return json.MarshalIndent([]map[string]string{{"name": values[0], "address": values[1], "port": values[2],
			"role": values[3], "block": values[4], "credential": values[5]}}, "", "  ")
	case "xlsx":
		return inventoryXLSXTemplate(fields, values)
	default:
		return nil, os.ErrInvalid
	}
}

func inventoryXLSXTemplate(fields, values []string) ([]byte, error) {
	files := templateXLSXParts()
	var sheet bytes.Buffer
	sheet.WriteString(`<?xml version="1.0"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for row, cells := range [][]string{fields, values} {
		writeTemplateRow(&sheet, row+1, cells)
	}
	sheet.WriteString(`</sheetData></worksheet>`)
	files["xl/worksheets/sheet1.xml"] = sheet.String()
	var out bytes.Buffer
	z := zip.NewWriter(&out)
	for name, data := range files {
		w, err := z.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = io.WriteString(w, data); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (a *App) SaveInventoryTemplate(format string) string {
	data, err := inventoryTemplate(format)
	if err != nil {
		return "IMPORT_FILE_INVALID"
	}
	a.mu.Lock()
	picker, ctx := a.saveDialog, a.ctx
	a.mu.Unlock()
	if picker == nil {
		return "EXPORT_UNAVAILABLE"
	}
	path, err := picker(ctx, runtime.SaveDialogOptions{Title: "Salvar modelo de equipamentos", DefaultFilename: "equipamentos." + format,
		Filters: []runtime.FileFilter{{DisplayName: "Modelo de equipamentos", Pattern: "*." + format}}})
	if err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	if path == "" {
		return "EXPORT_CANCELLED"
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "EXPORT_UNAVAILABLE"
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return "EXPORT_UNAVAILABLE"
	}
	return "OK"
}
