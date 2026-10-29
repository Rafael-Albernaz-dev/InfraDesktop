package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"strconv"
	"time"
)

func saveEquipmentInsights(ctx context.Context, tx *sql.Tx, d equipment.Device, credential uint64, result *equipment.PPReadResult, h *equipment.DeviceHealth, at time.Time) error {
	stamp := equipment.PPTime(at)
	var revision, oldCredential uint64
	var connection, oldStamp, rawHealth, rawPorts string
	e := tx.QueryRowContext(ctx, "SELECT revision,credential_revision,connection,stamp,health_json,ports_json FROM device_equipment_snapshot WHERE device_id=?", d.ID).Scan(&revision, &oldCredential, &connection, &oldStamp, &rawHealth, &rawPorts)
	if e != nil && e != sql.ErrNoRows {
		return e
	}
	old := map[string]*equipment.DevicePort{}
	var previous equipment.DeviceHealth
	if e == nil {
		before, e := time.Parse(time.RFC3339Nano, oldStamp)
		if e != nil {
			return e
		}
		if !at.After(before) {
			return nil
		}
		if revision == d.Revision && oldCredential == credential && connection == equipment.PPConnection(d) {
			if e = json.Unmarshal([]byte(rawHealth), &previous); e != nil {
				return e
			}
			var rows []equipment.PPReadRow
			if e = json.Unmarshal([]byte(rawPorts), &rows); e != nil {
				return e
			}
			for _, r := range rows {
				old[r.Name] = r.Port
			}
		}
	}
	save := func(kind, name string, row equipment.PPReadRow) error {
		body, e := json.Marshal(row)
		if e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO device_equipment_events VALUES(?,?,?,?,?,?)", d.ID, stamp, at.Unix(), kind, name, string(body))
		return e
	}
	if equipment.DetectedDeviceReboot(&previous, h) {
		if e = save("reboot", "", equipment.PPReadRow{Kind: "reboot", At: stamp, SourceAt: oldStamp}); e != nil {
			return e
		}
	}
	ports := []equipment.PPReadRow{}
	if result.Availability["ports"] == "complete" {
		for _, r := range result.Rows {
			if r.Kind != "port" || r.Port == nil {
				continue
			}
			r.At = stamp
			ports = append(ports, r)
			before := old[r.Name]
			counterIncrease := false
			if before != nil && before.LinkDowns != nil && r.Port.LinkDowns != nil {
				a, ea := strconv.ParseUint(*before.LinkDowns, 10, 64)
				b, eb := strconv.ParseUint(*r.Port.LinkDowns, 10, 64)
				counterIncrease = ea == nil && eb == nil && b > a
			}
			if equipment.PortNegotiationChanged(before, r.Port) || counterIncrease {
				event := r
				event.Kind = "port_change"
				event.BeforePort = before
				event.SourceAt = oldStamp
				if e = save("port_change", r.Name, event); e != nil {
					return e
				}
			}
		}
	}
	body, e := json.Marshal(h)
	if e != nil {
		return e
	}
	portBody, e := json.Marshal(ports)
	if e != nil {
		return e
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO device_equipment_snapshot VALUES(?,?,?,?,?,?,?,?) ON CONFLICT(device_id) DO UPDATE SET revision=excluded.revision,credential_revision=excluded.credential_revision,connection=excluded.connection,at=excluded.at,stamp=excluded.stamp,health_json=excluded.health_json,ports_json=excluded.ports_json`, d.ID, d.Revision, credential, equipment.PPConnection(d), at.Unix(), stamp, string(body), string(portBody))
	return e
}

func readEquipmentInsights(ctx context.Context, db *sql.DB, device, scope string, now time.Time) ([]equipment.PPReadRow, error) {
	rows := []equipment.PPReadRow{}
	if scope == "port-history" {
		q, e := db.QueryContext(ctx, "SELECT row_json FROM device_equipment_events WHERE device_id=? AND kind='port_change' AND at>=? ORDER BY at DESC,stamp DESC,name LIMIT 2049", device, now.Add(-30*24*time.Hour).Unix())
		if e != nil {
			return nil, e
		}
		defer q.Close()
		for q.Next() {
			var raw string
			var r equipment.PPReadRow
			if e = q.Scan(&raw); e != nil {
				return nil, e
			}
			if e = json.Unmarshal([]byte(raw), &r); e != nil {
				return nil, e
			}
			rows = append(rows, r)
		}
		return rows, q.Err()
	}
	var raw, ports, stamp string
	e := db.QueryRowContext(ctx, "SELECT s.health_json,s.ports_json,s.stamp FROM device_equipment_snapshot s JOIN devices d ON d.id=s.device_id WHERE s.device_id=? AND s.revision=d.revision", device).Scan(&raw, &ports, &stamp)
	if e == sql.ErrNoRows {
		return rows, nil
	}
	if e != nil {
		return nil, e
	}
	var h equipment.DeviceHealth
	if e = json.Unmarshal([]byte(raw), &h); e != nil {
		return nil, e
	}
	var portRows []equipment.PPReadRow
	if e = json.Unmarshal([]byte(ports), &portRows); e != nil {
		return nil, e
	}
	var seven, thirty uint64
	if e = db.QueryRowContext(ctx, "SELECT COALESCE(sum(CASE WHEN at>=? THEN 1 ELSE 0 END),0),count(*) FROM device_equipment_events WHERE device_id=? AND kind='reboot' AND at>=?", now.Add(-7*24*time.Hour).Unix(), device, now.Add(-30*24*time.Hour).Unix()).Scan(&seven, &thirty); e != nil {
		return nil, e
	}
	rows = append(rows, equipment.PPReadRow{Kind: "resource", Health: &h, At: stamp}, equipment.PPReadRow{Kind: "restarts", Used: &seven, Size: &thirty, At: stamp})
	rows = append(rows, portRows...)
	return rows, nil
}
