package storage

import (
	"context"
	"database/sql"
	"encoding/json"

	"infradesktop/internal/modules/equipment"
)

func loadEquipmentCredentials(ctx context.Context, q queryer, st *equipment.State) error {
	err := q.QueryRowContext(ctx, "SELECT envelope FROM vault_key WHERE singleton=1").Scan(&st.WrappedKey)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	rows, err := q.QueryContext(ctx, "SELECT id,alias,username,revision,envelope FROM credentials ORDER BY rowid")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c equipment.SealedCredential
		if err = rows.Scan(&c.Metadata.ID, &c.Metadata.Alias, &c.Metadata.User, &c.Metadata.Revision, &c.Envelope); err != nil {
			return err
		}
		st.Credentials = append(st.Credentials, c)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	rows.Close()
	defaults, err := q.QueryContext(ctx, "SELECT class,credential_id FROM radio_credential_defaults")
	if err != nil {
		return err
	}
	defer defaults.Close()
	st.CredentialDefaults = map[string]string{}
	for defaults.Next() {
		var class, id string
		if err = defaults.Scan(&class, &id); err != nil {
			return err
		}
		st.CredentialDefaults[class] = id
	}
	return defaults.Err()
}

const equipmentDeviceQuery = `SELECT d.id,d.name,d.address,d.port,d.transport,COALESCE(d.credential_id,''),
 d.revision,d.active,d.fingerprint,d.trust_origin,d.observation,d.trust_acceptance,d.last_attempt,
 COALESCE(extra.role,radio.role,d.role),d.block,h.health_json,h.stamp,o.meta_json,r.body
 FROM devices d
 LEFT JOIN device_inventory_roles extra ON extra.device_id=d.id
 LEFT JOIN device_radio_roles radio ON radio.device_id=d.id
 LEFT JOIN device_equipment_snapshot h ON h.device_id=d.id AND h.revision=d.revision
 LEFT JOIN pppoe_current current ON current.device_id=d.id
 LEFT JOIN pppoe_observations o ON o.id=current.observation_id
 LEFT JOIN radio_current r ON r.device_id=d.id
 ORDER BY d.rowid`

type deviceJSONFields struct {
	observation, acceptance, attempt, health, healthAt, pppoe, radio sql.NullString
}

func scanEquipmentDevice(rows *sql.Rows) (equipment.Device, error) {
	var d equipment.Device
	var raw deviceJSONFields
	err := rows.Scan(&d.ID, &d.Name, &d.Address, &d.Port, &d.Transport, &d.CredentialID, &d.Revision, &d.Active,
		&d.Fingerprint, &d.TrustOrigin, &raw.observation, &raw.acceptance, &raw.attempt, &d.Role, &d.Block,
		&raw.health, &raw.healthAt, &raw.pppoe, &raw.radio)
	if err != nil {
		return d, err
	}
	if err = decodeDeviceJSON(&d, raw); err != nil {
		return d, err
	}
	return d, nil
}
func decodeDeviceJSON(d *equipment.Device, raw deviceJSONFields) error {
	if err := decodeRadioSummary(d, raw.radio); err != nil {
		return err
	}
	if raw.observation.Valid {
		d.Result = &equipment.Observation{}
		if err := json.Unmarshal([]byte(raw.observation.String), d.Result); err != nil {
			return err
		}
	}
	if raw.acceptance.Valid {
		d.TrustAcceptance = &equipment.TrustAcceptance{}
		if err := json.Unmarshal([]byte(raw.acceptance.String), d.TrustAcceptance); err != nil {
			return err
		}
	}
	if raw.attempt.Valid {
		d.LastAttempt = &equipment.Attempt{}
		if err := json.Unmarshal([]byte(raw.attempt.String), d.LastAttempt); err != nil {
			return err
		}
	}
	if raw.health.Valid {
		d.Monitoring = &equipment.DeviceHealth{}
		if err := json.Unmarshal([]byte(raw.health.String), d.Monitoring); err != nil {
			return err
		}
		d.MonitoringAt = raw.healthAt.String
	}
	if raw.pppoe.Valid && d.Role == equipment.RoleConcentrator {
		var o equipment.PPObservation
		if err := json.Unmarshal([]byte(raw.pppoe.String), &o); err != nil {
			return err
		}
		online := o.Online
		if online == 0 && o.Offline == 0 {
			online = o.Count
		}
		d.PPPoE = &equipment.PPSummary{Online: online, Offline: o.Offline, At: o.Provenance.Ended}
	}
	return nil
}
func loadEquipmentDevices(ctx context.Context, q queryer, st *equipment.State) error {
	rows, err := q.QueryContext(ctx, equipmentDeviceQuery)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		d, err := scanEquipmentDevice(rows)
		if err != nil {
			return err
		}
		st.Devices = append(st.Devices, d)
	}
	return rows.Err()
}
func loadEquipmentOperations(ctx context.Context, q queryer, st *equipment.State) error {
	rows, err := q.QueryContext(ctx, "SELECT id,stamp,entity FROM equipment_operations ORDER BY rowid")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var op equipment.Operation
		if err = rows.Scan(&op.ID, &op.Stamp, &op.Entity); err != nil {
			return err
		}
		st.Operations = append(st.Operations, op)
	}
	return rows.Err()
}
