package presentation

import (
	"io"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/modules/equipment"
)

const inventoryImportMaxBytes = 2 * 1024 * 1024

type InventoryImportResult struct {
	EquipmentResult
	Filename string `json:"filename,omitempty"`
}

// ChooseInventoryImport reads only a file selected by the user and sends bytes to the service.
func (a *App) ChooseInventoryImport(profile, activation, block, credential string) InventoryImportResult {
	a.mu.Lock()
	picker, ctx := a.openDialog, a.ctx
	a.mu.Unlock()
	if picker == nil {
		return InventoryImportResult{EquipmentResult: EquipmentResult{Code: "IMPORT_UNAVAILABLE"}}
	}
	name, err := picker(ctx, runtime.OpenDialogOptions{Title: "Escolher arquivo de equipamentos",
		Filters: []runtime.FileFilter{{DisplayName: "Planilha ou dados", Pattern: "*.xlsx;*.csv;*.json"}}})
	if err != nil {
		return InventoryImportResult{EquipmentResult: EquipmentResult{Code: "IMPORT_UNAVAILABLE"}}
	}
	if name == "" {
		return InventoryImportResult{EquipmentResult: EquipmentResult{Code: "IMPORT_CANCELLED"}}
	}
	data, err := readInventoryFile(name)
	if err != nil {
		return InventoryImportResult{EquipmentResult: EquipmentResult{Code: "IMPORT_FILE_INVALID"}}
	}
	input := &equipment.ImportInput{Filename: filepath.Base(name), Data: data, DefaultBlock: block, DefaultCredential: credential}
	result := a.equipmentCall("inventory.import-preview", profile, activation, ipc.EquipmentInput{Import: input})
	return InventoryImportResult{EquipmentResult: result, Filename: filepath.Base(name)}
}

func readInventoryFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > inventoryImportMaxBytes {
		return nil, os.ErrInvalid
	}
	data, err := io.ReadAll(io.LimitReader(f, inventoryImportMaxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > inventoryImportMaxBytes {
		return nil, os.ErrInvalid
	}
	return data, nil
}

func (a *App) ReviewInventoryImport(profile, activation string, rows []equipment.ImportRow) EquipmentResult {
	return a.equipmentCall("inventory.import-preview", profile, activation,
		ipc.EquipmentInput{Import: &equipment.ImportInput{Rows: rows}})
}

func (a *App) ConfirmInventoryImport(profile, activation, operation string, rows []equipment.ImportRow) EquipmentResult {
	return a.equipmentCall("inventory.import-confirm", profile, activation,
		ipc.EquipmentInput{Operation: operation, Import: &equipment.ImportInput{Rows: rows}})
}
