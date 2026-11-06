package storage

import (
	"context"
	"database/sql"
	"reflect"

	"infradesktop/internal/modules/equipment"
)

// A block is presentation metadata; only an otherwise identical device can inherit a saved reading.
func onlyBlockChanged(before, after equipment.Device) bool {
	if before.Revision+1 != after.Revision || before.Block == after.Block {
		return false
	}
	before.Block, after.Block = "", ""
	before.Revision, after.Revision = 0, 0
	before.Monitoring, after.Monitoring = nil, nil
	before.MonitoringAt, after.MonitoringAt = "", ""
	before.RadioSummary, after.RadioSummary = nil, nil
	before.PPPoE, after.PPPoE = nil, nil
	return reflect.DeepEqual(before, after)
}

func preserveBlockReadings(ctx context.Context, tx *sql.Tx, before, after equipment.Device) error {
	if !onlyBlockChanged(before, after) {
		return nil
	}
	query := "UPDATE device_equipment_snapshot SET revision=? WHERE device_id=? AND revision=?"
	if _, err := tx.ExecContext(ctx, query, after.Revision, after.ID, before.Revision); err != nil {
		return err
	}
	const radio = `UPDATE radio_current SET body=json_set(body,'$.revision',?),
        payload_bytes=length(CAST(json_set(body,'$.revision',?) AS BLOB))
        WHERE device_id=? AND json_extract(body,'$.revision')=?`
	if _, err := tx.ExecContext(ctx, radio, after.Revision, after.Revision, after.ID, before.Revision); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE radio_profiles SET body=json_set(body,'$.revision',?)
        WHERE device_id=? AND json_extract(body,'$.revision')=?`, after.Revision, after.ID, before.Revision)
	return err
}
