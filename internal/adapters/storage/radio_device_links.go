package storage

import (
	"context"
	"encoding/json"
	"infradesktop/internal/modules/equipment"
)

func (s *Store) RadioDeviceLinks(ctx context.Context, company, device string) ([]equipment.RadioLink, equipment.Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.unit == nil || s.unitID != company {
		return nil, equipment.Changed
	}
	if s.checkActive() != nil {
		return nil, equipment.Unavailable
	}
	const query = "SELECT body FROM radio_links WHERE json_extract(body,'$.ap_id')=? OR json_extract(body,'$.cpe_id')=? " +
		"ORDER BY concentrator_id,scope,login LIMIT ?"
	rows, err := s.unit.QueryContext(ctx, query, device, device, equipment.RadioMaxClients+1)
	if err != nil {
		return nil, equipment.Unavailable
	}
	defer rows.Close()
	out := []equipment.RadioLink{}
	for rows.Next() {
		var raw string
		var link equipment.RadioLink
		if rows.Scan(&raw) != nil || json.Unmarshal([]byte(raw), &link) != nil {
			return nil, equipment.Unavailable
		}
		out = append(out, link)
	}
	if rows.Err() != nil {
		return nil, equipment.Unavailable
	}
	if len(out) > equipment.RadioMaxClients {
		return nil, equipment.Limit
	}
	return out, equipment.OK
}
