package storage

import (
	"context"
	"database/sql"
	"infradesktop/internal/modules/equipment"
	"sort"
	"time"
)

func crossConcentratorRows(ctx context.Context, db *sql.DB, selected string, now time.Time) ([]equipment.PPReadRow, error) {
	all, e := loadConcentratorReadings(ctx, db, "")
	if e != nil {
		return nil, e
	}
	var base *concentratorReading
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
	before, e := loadConcentratorRows(ctx, db, base.o.ID)
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
		after, e := loadConcentratorRows(ctx, db, peer.o.ID)
		if e != nil {
			return nil, e
		}
		out = appendConcentratorDuplicates(out, base, peer, before, after, keys)
		if len(out) > equipment.PPMaxRows {
			return out, nil
		}
	}
	return out, nil
}

func appendConcentratorDuplicates(
	out []equipment.PPReadRow, base, peer *concentratorReading,
	before, after map[string][]equipment.PPRow, keys []string,
) []equipment.PPReadRow {
	for _, login := range keys {
		for _, a := range before[login] {
			if a.State == "OFFLINE" {
				continue
			}
			for _, b := range after[login] {
				if b.State == "OFFLINE" {
					continue
				}
				duplicate := &equipment.PPCrossDuplicate{A: concentratorPeerSession(base, a), B: concentratorPeerSession(peer, b)}
				out = append(out, equipment.PPReadRow{Kind: "duplicate", Login: login, Duplicate: duplicate})
				if len(out) > equipment.PPMaxRows {
					return out
				}
			}
		}
	}
	return out
}

func concentratorPeerSession(d *concentratorReading, r equipment.PPRow) equipment.PPPeerSession {
	return equipment.PPPeerSession{
		DeviceID: d.id, Name: d.name, Endpoint: d.endpoint, Address: r.Address,
		Caller: r.Caller, Uptime: r.Uptime, At: d.o.Provenance.Ended,
	}
}
