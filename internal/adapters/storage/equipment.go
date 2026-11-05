package storage

import (
	"context"
	"infradesktop/internal/modules/equipment"
)

// All callbacks are short and serialized with active-unit changes. Network IO is
// never performed inside this transaction or while retaining the active unit.
func (s *Store) ReadEquipment(ctx context.Context, id string) (equipment.State, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != id {
		return equipment.State{}, equipment.Changed
	}
	if s.checkActive() != nil {
		return equipment.State{}, equipment.Unavailable
	}
	st, e := s.loadEquipment(ctx, s.unit, id)
	if e != nil {
		return equipment.State{}, equipment.Unavailable
	}
	return st, equipment.OK
}
func (s *Store) loadEquipment(ctx context.Context, q queryer, id string) (equipment.State, error) {
	st := equipment.State{Installation: s.installation, Company: id, Credentials: []equipment.SealedCredential{}, Devices: []equipment.Device{}, Operations: []equipment.Operation{}}
	if err := loadEquipmentCredentials(ctx, q, &st); err != nil {
		return st, err
	}
	if err := loadEquipmentDevices(ctx, q, &st); err != nil {
		return st, err
	}
	if err := loadEquipmentOperations(ctx, q, &st); err != nil {
		return st, err
	}
	return st, nil
}
func (s *Store) UpdateEquipment(ctx context.Context, id string, fn func(*equipment.State) equipment.Code) equipment.Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != id {
		return equipment.Changed
	}
	if s.checkActive() != nil {
		return equipment.Unavailable
	}
	tx, err := s.unit.BeginTx(ctx, nil)
	if err != nil {
		return equipment.Unavailable
	}
	defer tx.Rollback()
	state, err := s.loadEquipment(ctx, tx, id)
	if err != nil {
		return equipment.Unavailable
	}
	old, err := cloneEquipmentState(state)
	if err != nil {
		return equipment.Unavailable
	}
	if code := fn(&state); code != equipment.OK {
		return code
	}
	if code := persistEquipmentChanges(ctx, tx, old, state); code != equipment.OK {
		return code
	}
	s.observe("before-equipment-commit")
	if tx.Commit() != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}
