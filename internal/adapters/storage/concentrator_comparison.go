package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
	"sort"
	"time"
)

func crossConcentratorRows(ctx context.Context, db *sql.DB, selected string, now time.Time) ([]equipment.PPReadRow, error) {
	type reading struct {
		id, name, endpoint string
		o                  equipment.PPObservation
		current            bool
	}
	q, e := db.QueryContext(ctx, `SELECT d.id,d.name,d.address,d.revision,d.transport,d.port,d.fingerprint,COALESCE(d.credential_id,''),COALESCE(k.revision,0),o.meta_json FROM devices d LEFT JOIN credentials k ON k.id=d.credential_id JOIN pppoe_current c ON c.device_id=d.id JOIN pppoe_observations o ON o.id=c.observation_id WHERE d.active=1 AND d.role='concentrator' AND c.scope=? ORDER BY d.name,d.id`, equipment.PPScope)
	if e != nil {
		return nil, e
	}
	all := []reading{}
	var base *reading
	for q.Next() {
		var r reading
		var raw string
		var d equipment.Device
		var credential uint64
		if e = q.Scan(&r.id, &r.name, &r.endpoint, &d.Revision, &d.Transport, &d.Port, &d.Fingerprint, &d.CredentialID, &credential, &raw); e != nil {
			q.Close()
			return nil, e
		}
		if e = json.Unmarshal([]byte(raw), &r.o); e != nil {
			q.Close()
			return nil, e
		}
		d.ID = r.id
		d.Address = r.endpoint
		r.current = r.o.Provenance.Revision == d.Revision && r.o.Provenance.Connection == equipment.PPConnection(d) && r.o.Provenance.CredentialRevision == credential
		all = append(all, r)
	}
	e = q.Err()
	q.Close()
	if e != nil {
		return nil, e
	}
	for i := range all {
		if all[i].id == selected {
			base = &all[i]
		}
	}
	if base == nil {
		return []equipment.PPReadRow{{Kind: "availability", Name: "Comparação entre concentradores", Value: "não comparável (leitura indisponível)"}}, nil
	}
	if !base.current {
		return []equipment.PPReadRow{{Kind: "availability", Name: "Comparação entre concentradores", Value: "não comparável (contexto alterado)"}}, nil
	}
	load := func(r *reading) (map[string][]equipment.PPRow, error) {
		q, e := db.QueryContext(ctx, "SELECT row_json FROM pppoe_rows WHERE observation_id=? ORDER BY ordinal", r.o.ID)
		if e != nil {
			return nil, e
		}
		defer q.Close()
		by := map[string][]equipment.PPRow{}
		for q.Next() {
			var raw string
			var row equipment.PPRow
			if e = q.Scan(&raw); e != nil {
				return nil, e
			}
			if e = json.Unmarshal([]byte(raw), &row); e != nil {
				return nil, e
			}
			if row.State != "OFFLINE" && row.Login != "" {
				by[row.Login] = append(by[row.Login], row)
			}
		}
		return by, q.Err()
	}
	before, e := load(base)
	if e != nil {
		return nil, e
	}
	out := []equipment.PPReadRow{}
	keys := []string{}
	for login := range before {
		keys = append(keys, login)
	}
	sort.Strings(keys)
	for i := range all {
		peer := &all[i]
		if peer.id == selected {
			continue
		}
		if !peer.current {
			out = append(out, equipment.PPReadRow{Kind: "availability", Name: peer.name, Value: "não comparável (contexto alterado)"})
			continue
		}
		if !equipment.ComparableConcentratorReadings(&base.o, &peer.o, now) {
			out = append(out, equipment.PPReadRow{Kind: "availability", Name: peer.name, Value: "não comparável (leituras distantes)"})
			continue
		}
		after, e := load(peer)
		if e != nil {
			return nil, e
		}
		for _, login := range keys {
			for _, a := range before[login] {
				for _, b := range after[login] {
					view := func(d *reading, r equipment.PPRow) equipment.PPPeerSession {
						return equipment.PPPeerSession{DeviceID: d.id, Name: d.name, Endpoint: d.endpoint, Address: r.Address, Caller: r.Caller, Uptime: r.Uptime, At: d.o.Provenance.Ended}
					}
					out = append(out, equipment.PPReadRow{Kind: "duplicate", Login: login, Duplicate: &equipment.PPCrossDuplicate{A: view(base, a), B: view(peer, b)}})
					if len(out) > equipment.PPMaxRows {
						return out, nil
					}
				}
			}
		}
	}
	return out, nil
}
