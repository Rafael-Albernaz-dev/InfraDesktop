package storage

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"

	"infradesktop/internal/modules/equipment"
)

func cloneEquipmentState(st equipment.State) (equipment.State, error) {
	var clone equipment.State
	b, err := json.Marshal(st)
	if err != nil {
		return clone, err
	}
	err = json.Unmarshal(b, &clone)
	return clone, err
}
func persistEquipmentChanges(ctx context.Context, tx *sql.Tx, old, current equipment.State) equipment.Code {
	if len(current.WrappedKey) > 0 && !bytes.Equal(current.WrappedKey, old.WrappedKey) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO vault_key VALUES(1,?)
ON CONFLICT(singleton) DO UPDATE SET envelope=excluded.envelope`, current.WrappedKey); err != nil {
			return equipment.Unavailable
		}
	}
	if c := persistEquipmentCredentials(ctx, tx, old, current); c != equipment.OK {
		return c
	}
	if c := persistEquipmentDefaults(ctx, tx, old, current); c != equipment.OK {
		return c
	}
	if c := persistEquipmentDevices(ctx, tx, old.Devices, current.Devices); c != equipment.OK {
		return c
	}
	return persistEquipmentOperations(ctx, tx, old.Operations, current.Operations)
}
func persistEquipmentCredentials(ctx context.Context, tx *sql.Tx, old, current equipment.State) equipment.Code {
	if c := deleteRemovedCredentials(ctx, tx, old.Credentials, current.Credentials); c != equipment.OK {
		return c
	}
	previous := map[string][]byte{}
	for _, c := range old.Credentials {
		b, err := json.Marshal(c)
		if err != nil {
			return equipment.Unavailable
		}
		previous[c.Metadata.ID] = b
	}
	for _, c := range current.Credentials {
		b, err := json.Marshal(c)
		if err != nil {
			return equipment.Unavailable
		}
		if bytes.Equal(b, previous[c.Metadata.ID]) {
			continue
		}
		m := c.Metadata
		_, err = tx.ExecContext(ctx, `INSERT INTO credentials VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET
   alias=excluded.alias,username=excluded.username,revision=excluded.revision,envelope=excluded.envelope`,
			m.ID, m.Alias, m.User, m.Revision, c.Envelope)
		if err != nil {
			return equipment.Unavailable
		}
	}
	return equipment.OK
}
func persistEquipmentDefaults(ctx context.Context, tx *sql.Tx, old, current equipment.State) equipment.Code {
	for class, id := range current.CredentialDefaults {
		if old.CredentialDefaults[class] == id {
			continue
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO radio_credential_defaults VALUES(?,?)
   ON CONFLICT(class) DO UPDATE SET credential_id=excluded.credential_id`, class, id)
		if err != nil {
			return equipment.Unavailable
		}
	}
	return equipment.OK
}
func persistEquipmentOperations(ctx context.Context, tx *sql.Tx, old, current []equipment.Operation) equipment.Code {
	previous := map[string]bool{}
	for _, op := range old {
		previous[op.ID] = true
	}
	for _, op := range current {
		if previous[op.ID] {
			continue
		}
		entity := op.Entity
		if op.BulkReceipt != nil || op.ImportReceipt != nil {
			var receipt any = op.BulkReceipt
			if op.ImportReceipt != nil {
				receipt = op.ImportReceipt
			}
			body, err := json.Marshal(receipt)
			if err != nil {
				return equipment.Unavailable
			}
			entity = string(body)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?) ON CONFLICT(id) DO NOTHING",
			op.ID, op.Stamp, entity); err != nil {
			return equipment.Unavailable
		}
	}
	return equipment.OK
}
func persistEquipmentDevices(ctx context.Context, tx *sql.Tx, old, current []equipment.Device) equipment.Code {
	previous := map[string][]byte{}
	previousDevices := map[string]equipment.Device{}
	for _, d := range old {
		b, err := json.Marshal(d)
		if err != nil {
			return equipment.Unavailable
		}
		previous[d.ID] = b
		previousDevices[d.ID] = d
	}
	for _, d := range current {
		b, err := json.Marshal(d)
		if err != nil {
			return equipment.Unavailable
		}
		if bytes.Equal(b, previous[d.ID]) {
			continue
		}
		if before, ok := previousDevices[d.ID]; ok {
			if err := preserveBlockReadings(ctx, tx, before, d); err != nil {
				return equipment.Unavailable
			}
		}
		if code := persistEquipmentDevice(ctx, tx, d); code != equipment.OK {
			return code
		}
	}
	return equipment.OK
}

type storedDeviceValues struct{ observation, credential, acceptance, attempt any }

func storedDeviceFields(d equipment.Device) (storedDeviceValues, equipment.Code) {
	var v storedDeviceValues
	if d.CredentialID != "" {
		v.credential = d.CredentialID
	}
	if d.Result != nil {
		b, err := json.Marshal(d.Result)
		if err != nil || len(b) > 48*1024 {
			return v, equipment.Limit
		}
		v.observation = string(b)
	}
	if d.TrustAcceptance != nil {
		b, err := json.Marshal(d.TrustAcceptance)
		if err != nil {
			return v, equipment.Unavailable
		}
		v.acceptance = string(b)
	}
	if d.LastAttempt != nil {
		b, err := json.Marshal(d.LastAttempt)
		if err != nil {
			return v, equipment.Unavailable
		}
		v.attempt = string(b)
	}
	return v, equipment.OK
}
func persistEquipmentDevice(ctx context.Context, tx *sql.Tx, d equipment.Device) equipment.Code {
	if d.Role == "" {
		d.Role = equipment.RoleOther
	}
	v, code := storedDeviceFields(d)
	if code != equipment.OK {
		return code
	}
	databaseRole := d.Role
	if d.Role == equipment.RolePTP || equipment.RoleWithoutDriver(d.Role) {
		databaseRole = equipment.RoleOther
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,
 fingerprint,trust_origin,observation,trust_acceptance,last_attempt,role,block) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(id) DO UPDATE SET name=excluded.name,address=excluded.address,port=excluded.port,
 transport=excluded.transport,credential_id=excluded.credential_id,revision=excluded.revision,active=excluded.active,
 fingerprint=excluded.fingerprint,trust_origin=excluded.trust_origin,observation=excluded.observation,
 trust_acceptance=excluded.trust_acceptance,last_attempt=excluded.last_attempt,role=excluded.role,block=excluded.block`,
		d.ID, d.Name, d.Address, d.Port, d.Transport, v.credential, d.Revision, d.Active, d.Fingerprint, d.TrustOrigin,
		v.observation, v.acceptance, v.attempt, databaseRole, d.Block)
	if err != nil {
		return equipment.Unavailable
	}
	return persistDeviceRole(ctx, tx, d)
}
func persistDeviceRole(ctx context.Context, tx *sql.Tx, d equipment.Device) equipment.Code {
	if d.Role == equipment.RolePTP {
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_radio_roles VALUES(?,?)
   ON CONFLICT(device_id) DO UPDATE SET role=excluded.role`, d.ID, d.Role); err != nil {
			return equipment.Unavailable
		}
	} else if _, err := tx.ExecContext(ctx, "DELETE FROM device_radio_roles WHERE device_id=?", d.ID); err != nil {
		return equipment.Unavailable
	}
	if equipment.RoleWithoutDriver(d.Role) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO device_inventory_roles VALUES(?,?)
   ON CONFLICT(device_id) DO UPDATE SET role=excluded.role`, d.ID, d.Role); err != nil {
			return equipment.Unavailable
		}
	} else if _, err := tx.ExecContext(ctx, "DELETE FROM device_inventory_roles WHERE device_id=?", d.ID); err != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}
