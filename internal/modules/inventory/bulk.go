package inventory

import (
	"context"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
)

func (s *Service) bulkUpdate(ctx context.Context, q Command, v *View) Code {
	if c := validateBulk(q); c != OK {
		return c
	}
	raw, err := json.Marshal(q.Bulk)
	if err != nil {
		return Invalid
	}
	stamp := "inventory.bulk-update:" + string(raw)
	mutated := false
	c := s.Store.UpdateEquipment(ctx, q.Scope.Profile, func(st *State) Code {
		return applyBulk(st, q, stamp, v, &mutated)
	})
	if c == OK && mutated {
		for _, item := range q.Bulk.Items {
			s.Invalidate(q.Scope.Profile, item.ID, false)
		}
	}
	return c
}

func validateBulk(q Command) Code {
	if !ValidID(q.Operation) || q.Bulk == nil || len(q.Bulk.Items) == 0 || len(q.Bulk.Items) > MaxDevices {
		return Invalid
	}
	if (q.Bulk.Block == nil) == (q.Bulk.CredentialID == nil) {
		return Invalid
	}
	if q.Bulk.Block != nil && !equipment.ValidBlock(*q.Bulk.Block) {
		return Invalid
	}
	if q.Bulk.CredentialID != nil && !ValidID(*q.Bulk.CredentialID) {
		return Invalid
	}
	seen := map[string]bool{}
	for _, item := range q.Bulk.Items {
		if !ValidID(item.ID) || item.Revision == 0 || seen[item.ID] {
			return Invalid
		}
		seen[item.ID] = true
	}
	return OK
}

func applyBulk(st *State, q Command, stamp string, v *View, mutated *bool) Code {
	if replayed, code := replayBulk(st, q, stamp, v); replayed {
		return code
	}
	if len(st.Operations) >= MaxOperations {
		return Limit
	}
	if q.Bulk.CredentialID != nil && CredentialByID(st, *q.Bulk.CredentialID) == nil {
		return MissingCredential
	}
	for _, item := range q.Bulk.Items {
		d := DeviceByID(st, item.ID)
		if d == nil || d.Revision != item.Revision {
			return Revision
		}
	}
	return commitBulk(st, q, stamp, v, mutated)
}

func replayBulk(st *State, q Command, stamp string, v *View) (bool, Code) {
	for _, op := range st.Operations {
		if op.ID != q.Operation {
			continue
		}
		if op.Stamp != stamp {
			return true, Revision
		}
		var receipt equipment.BulkReceipt
		if json.Unmarshal([]byte(op.Entity), &receipt) != nil {
			return true, Unavailable
		}
		v.BulkReceipt = &receipt
		return true, OK
	}
	return false, OK
}

func commitBulk(st *State, q Command, stamp string, v *View, mutated *bool) Code {
	receipt := equipment.BulkReceipt{Items: make([]equipment.BulkItem, 0, len(q.Bulk.Items))}
	for _, item := range q.Bulk.Items {
		d := DeviceByID(st, item.ID)
		if q.Bulk.Block != nil {
			d.Block = *q.Bulk.Block
		}
		if q.Bulk.CredentialID != nil && d.CredentialID != *q.Bulk.CredentialID {
			d.CredentialID = *q.Bulk.CredentialID
			d.Result, d.LastAttempt = nil, nil
		}
		d.Revision++
		receipt.Items = append(receipt.Items, equipment.BulkItem{ID: d.ID, Revision: d.Revision})
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		return Unavailable
	}
	st.Operations = append(st.Operations, Operation{ID: q.Operation, Stamp: stamp, Entity: string(body)})
	v.BulkReceipt, *mutated = &receipt, true
	return OK
}
