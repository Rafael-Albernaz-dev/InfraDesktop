package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
)

func encodeRadioLinkEffect(link equipment.RadioLink) ([]byte, string, equipment.Code) {
	body, err := json.Marshal(link)
	if err != nil || len(body) > 4096 {
		return nil, "", equipment.Invalid
	}
	identity := link
	identity.At, identity.ConfirmedNow = "", false
	canonical, err := json.Marshal(identity)
	if err != nil {
		return nil, "", equipment.Invalid
	}
	return body, "radio-link:" + string(canonical), equipment.OK
}

func readRadioLinkSource(ctx context.Context, tx *sql.Tx,
	link equipment.RadioLink, source equipment.PPInput) (equipment.PPRow, equipment.Code) {
	var body string
	const query = "SELECT r.row_json FROM pppoe_current c JOIN pppoe_rows r ON r.observation_id=c.observation_id " +
		"WHERE c.device_id=? AND c.observation_id=? AND r.row_id=?"
	if tx.QueryRowContext(ctx, query, link.Concentrator, source.ObservationID, source.RowID).Scan(&body) != nil {
		return equipment.PPRow{}, equipment.ObservationMissing
	}
	var row equipment.PPRow
	if json.Unmarshal([]byte(body), &row) != nil || row.Login != link.Login || row.State == "OFFLINE" {
		return equipment.PPRow{}, equipment.ObservationIncompatible
	}
	return row, equipment.OK
}

func enrollRadioLinkCPE(ctx context.Context, tx *sql.Tx, link equipment.RadioLink, row equipment.PPRow) equipment.Code {
	device := link.NewCPE
	if device == nil {
		return equipment.OK
	}
	if equipment.ValidateDevice(*device) != equipment.OK || device.ID != link.CPE ||
		device.Role != equipment.RoleCPE || device.Address != row.Address || device.Fingerprint != "" {
		return equipment.Invalid
	}
	var count int
	if tx.QueryRowContext(ctx, "SELECT count(*) FROM devices").Scan(&count) != nil || count >= equipment.MaxDevices {
		return equipment.Limit
	}
	var selected string
	const defaultCredential = "SELECT credential_id FROM radio_credential_defaults WHERE class='cpe'"
	if tx.QueryRowContext(ctx, defaultCredential).Scan(&selected) != nil || selected != device.CredentialID {
		return equipment.Revision
	}
	const insert = "INSERT INTO devices(id,name,address,port,transport,credential_id,revision,active,fingerprint,trust_origin,role) " +
		"VALUES(?,?,?,?,?,?,1,1,'','','cpe')"
	if _, err := tx.ExecContext(ctx, insert, device.ID, device.Name, device.Address,
		device.Port, device.Transport, device.CredentialID); err != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}

func validateStoredRadioLinkEndpoints(ctx context.Context, tx *sql.Tx,
	link equipment.RadioLink, row equipment.PPRow) equipment.Code {
	var address string
	const query = "SELECT address FROM devices WHERE id=? AND role='cpe'"
	if tx.QueryRowContext(ctx, query, link.CPE).Scan(&address) != nil || address != row.Address {
		return equipment.ObservationIncompatible
	}
	for _, id := range []string{link.Concentrator, link.CPE, link.AP} {
		var count int
		if tx.QueryRowContext(ctx, "SELECT count(*) FROM devices WHERE id=? AND active=1", id).Scan(&count) != nil || count != 1 {
			return equipment.Revision
		}
	}
	return equipment.OK
}

func writeRadioLinkEffect(ctx context.Context, tx *sql.Tx, link equipment.RadioLink,
	operation, stamp string, body []byte) equipment.Code {
	const insert = "INSERT INTO radio_links VALUES(?,?,?,?) ON CONFLICT(concentrator_id,scope,login) DO UPDATE SET body=excluded.body"
	if _, err := tx.ExecContext(ctx, insert, link.Concentrator, link.Scope, link.Login, string(body)); err != nil {
		return equipment.Unavailable
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO equipment_operations VALUES(?,?,?)", operation, stamp, link.AP); err != nil {
		return equipment.Unavailable
	}
	return equipment.OK
}
