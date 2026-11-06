package presentation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"infradesktop/internal/adapters/importfile"
)

func TestInventoryTemplatesRoundTripAllFormats(t *testing.T) {
	for _, format := range []string{"csv", "json", "xlsx"} {
		data, err := inventoryTemplate(format)
		if err != nil {
			t.Fatal(format, err)
		}
		rows, err := (importfile.Parser{}).Parse("modelo."+format, data)
		if err != nil || len(rows) != 1 || rows[0].Name != "AP-LESTE-1" || rows[0].Address != "192.0.2.31" ||
			rows[0].Port != "22" || rows[0].Credential != "leitura-radio" {
			t.Fatalf("%s: %#v %v", format, rows, err)
		}
	}
}

func TestSaveInventoryTemplateDoesNotOverwriteAndRespectsCancel(t *testing.T) {
	app := New(nil)
	path := filepath.Join(t.TempDir(), "modelo.csv")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return path, nil }
	if code := app.SaveInventoryTemplate("csv"); code != "OK" {
		t.Fatal(code)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := app.SaveInventoryTemplate("csv"); code != "EXPORT_UNAVAILABLE" {
		t.Fatal(code)
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("file changed", err)
	}
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return "", nil }
	if code := app.SaveInventoryTemplate("json"); code != "EXPORT_CANCELLED" {
		t.Fatal(code)
	}
}

func TestSelectedInventoryFileRejectsOversize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.csv")
	if err := os.WriteFile(path, make([]byte, inventoryImportMaxBytes+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readInventoryFile(path); err == nil {
		t.Fatal("oversized file accepted")
	}
}
