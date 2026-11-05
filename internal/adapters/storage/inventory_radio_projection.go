package storage

import (
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
)

func decodeRadioSummary(d *equipment.Device, raw sql.NullString) error {
	if !raw.Valid {
		return nil
	}
	var reading equipment.RadioReading
	if err := json.Unmarshal([]byte(raw.String), &reading); err != nil {
		return err
	}
	d.RadioSummary = equipment.ProjectRadioSummary(reading)
	return nil
}
