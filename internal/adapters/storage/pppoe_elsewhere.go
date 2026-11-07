package storage

import (
	"context"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"time"
)

type concentratorReading struct {
	id, name, endpoint string
	o                  equipment.PPObservation
	current            bool
}

func ppReadingMatchesConnection(o *equipment.PPObservation, d equipment.Device, credential uint64) bool {
	return o.Provenance.Revision == d.Revision &&
		o.Provenance.Connection == equipment.PPConnection(d) &&
		o.Provenance.CredentialRevision == credential
}

func loadConcentratorReadings(ctx context.Context, q queryer, exclude string) ([]concentratorReading, error) {
	rows, err := q.QueryContext(ctx, `SELECT d.id,d.name,d.address,d.revision,d.transport,d.port,d.fingerprint,
		COALESCE(d.credential_id,''),COALESCE(k.revision,0),o.meta_json FROM devices d
		LEFT JOIN credentials k ON k.id=d.credential_id JOIN pppoe_current c ON c.device_id=d.id
		JOIN pppoe_observations o ON o.id=c.observation_id
		WHERE d.active=1 AND d.role='concentrator' AND c.scope=? AND d.id<>? ORDER BY d.name,d.id`, equipment.PPScope, exclude)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := []concentratorReading{}
	for rows.Next() {
		var r concentratorReading
		var d equipment.Device
		var credential uint64
		var raw string
		err = rows.Scan(&r.id, &r.name, &r.endpoint, &d.Revision, &d.Transport, &d.Port,
			&d.Fingerprint, &d.CredentialID, &credential, &raw)
		if err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &r.o); err != nil {
			return nil, err
		}
		d.ID, d.Address = r.id, r.endpoint
		r.current = ppReadingMatchesConnection(&r.o, d, credential)
		all = append(all, r)
	}
	return all, rows.Err()
}

func loadConcentratorRows(ctx context.Context, q queryer, observation string) (map[string][]equipment.PPRow, error) {
	rows, err := q.QueryContext(ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal", observation)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	by := map[string][]equipment.PPRow{}
	for rows.Next() {
		var raw string
		var row equipment.PPRow
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &row); err != nil {
			return nil, err
		}
		if row.Login != "" {
			by[row.Login] = append(by[row.Login], row)
		}
	}
	return by, rows.Err()
}

func onlineSeenOnOtherConcentrators(ctx context.Context, q queryer, device string) (map[string]string, error) {
	readings, err := loadConcentratorReadings(ctx, q, device)
	if err != nil {
		return nil, err
	}
	seen := map[string]string{}
	for _, r := range readings {
		if !r.current || r.o.Quality.Status != "complete" {
			continue
		}
		ended, err := time.Parse(time.RFC3339Nano, r.o.Provenance.Ended)
		if err != nil {
			continue
		}
		rows, err := loadConcentratorRows(ctx, q, r.o.ID)
		if err != nil {
			return nil, err
		}
		for login, sessions := range rows {
			for _, row := range sessions {
				previous, parseErr := time.Parse(time.RFC3339Nano, seen[login])
				if row.State == "ONLINE" && (parseErr != nil || ended.After(previous)) {
					seen[login] = r.o.Provenance.Ended
				}
			}
		}
	}
	return seen, nil
}
