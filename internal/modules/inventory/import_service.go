package inventory

import (
	"context"
	"errors"

	"infradesktop/internal/modules/equipment"
)

func (s *Service) handleImport(ctx context.Context, q Command, v *View) Code {
	if q.Op == "inventory.import-preview" {
		return s.importPreview(ctx, q, v)
	}
	return s.importConfirm(ctx, q, v)
}

func (s *Service) importPreview(ctx context.Context, q Command, v *View) Code {
	if q.Import == nil {
		return Invalid
	}
	rows, code := s.previewRows(q.Import)
	if code != OK {
		return code
	}
	for i := range rows {
		if rows[i].Block == "" {
			rows[i].Block = q.Import.DefaultBlock
		}
		if rows[i].Credential == "" {
			rows[i].Credential = q.Import.DefaultCredential
		}
	}
	st, code := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if code != OK {
		return code
	}
	v.ImportItems = reviewImport(&st, rows)
	return OK
}

func (s *Service) previewRows(input *equipment.ImportInput) ([]equipment.ImportRow, Code) {
	if len(input.Rows) > 0 {
		if len(input.Rows) > importRowsLimit || len(input.Data) > 0 || input.Filename != "" {
			return nil, Invalid
		}
		return append([]equipment.ImportRow(nil), input.Rows...), OK
	}
	if s.Parser == nil || len(input.Data) > 2*1024*1024 {
		return nil, Invalid
	}
	rows, err := s.Parser.Parse(input.Filename, input.Data)
	if errors.Is(err, ErrImportSecret) {
		return nil, equipment.ImportSecret
	}
	if err != nil || len(rows) > importRowsLimit {
		return nil, Invalid
	}
	return rows, OK
}

func importStamp(digester ImportDigester, rows []equipment.ImportRow) (string, Code) {
	digest, err := digester.Digest(rows)
	if err != nil {
		return "", Invalid
	}
	return "inventory.import-confirm:" + digest, OK
}

func (s *Service) importConfirm(ctx context.Context, q Command, v *View) Code {
	if q.Import == nil || s.Digester == nil || !ValidID(q.Operation) || len(q.Import.Rows) == 0 || len(q.Import.Rows) > importRowsLimit {
		return Invalid
	}
	if len(q.Import.Data) != 0 || q.Import.Filename != "" || q.Import.DefaultBlock != "" || q.Import.DefaultCredential != "" {
		return Invalid
	}
	stamp, code := importStamp(s.Digester, q.Import.Rows)
	if code != OK {
		return code
	}
	return s.Store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		return s.commitImport(st, q, stamp, v)
	})
}

func (s *Service) commitImport(st *State, q Command, stamp string, v *View) Code {
	if replayed, code := replayImport(st, q.Operation, stamp, v); replayed {
		return code
	}
	if len(st.Operations) >= MaxOperations {
		return Limit
	}
	items := reviewImport(st, q.Import.Rows)
	receipt := equipment.ImportReceipt{}
	for _, item := range items {
		if item.Status == importIgnored {
			receipt.Ignored++
			continue
		}
		if item.Status != importReady {
			return Revision
		}
		receipt.Created++
	}
	if receipt.Created == 0 {
		return Invalid
	}
	if len(st.Devices)+receipt.Created > MaxDevices {
		return Limit
	}
	for _, item := range items {
		if item.Status == importIgnored {
			continue
		}
		id, code := s.IDs.ID()
		if code != OK {
			return code
		}
		d, reason := importDevice(item.Row, id, item.CredentialID)
		if reason != "" || DeviceByID(st, id) != nil {
			return Revision
		}
		st.Devices = append(st.Devices, d)
	}
	st.Operations = append(st.Operations, Operation{ID: q.Operation, Stamp: stamp, ImportReceipt: &receipt})
	v.ImportReceipt = &receipt
	return OK
}

func replayImport(st *State, operation, stamp string, v *View) (bool, Code) {
	for _, op := range st.Operations {
		if op.ID != operation {
			continue
		}
		if op.Stamp != stamp {
			return true, Revision
		}
		if op.ImportReceipt == nil {
			return true, Unavailable
		}
		v.ImportReceipt = op.ImportReceipt
		return true, OK
	}
	return false, OK
}
