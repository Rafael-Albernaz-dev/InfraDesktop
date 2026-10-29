package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"infradesktop/internal/modules/equipment"
	"sort"
	"time"
)

func groupContextKey(p equipment.PPProvenance) string {
	raw, _ := json.Marshal([]any{p.Installation, p.Company, p.DeviceID, p.Connection, p.Revision, p.CredentialRevision, p.Family, p.Version})
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}
func knownOnlineGroups(o *equipment.PPObservation) (map[string]equipment.PPGroup, bool) {
	groups := map[string]equipment.PPGroup{}
	if o == nil || o.ContextOmitted || o.Quality.Status != "complete" {
		return groups, false
	}
	for _, r := range o.Rows {
		if r.State == "OFFLINE" {
			continue
		}
		c := r.Context
		if c == nil || c.Association != "associated" || c.Service == "" || c.VLAN == nil {
			return groups, false
		}
		key := fmt.Sprintf("%d:%s", *c.VLAN, c.Service)
		g := groups[key]
		g.Service = c.Service
		g.VLAN = c.VLAN
		g.Count++
		groups[key] = g
	}
	return groups, true
}
func recordContextGroups(ctx context.Context, tx *sql.Tx, o *equipment.PPObservation) error {
	groups, known := knownOnlineGroups(o)
	if !known {
		return nil
	}
	at, e := time.Parse(time.RFC3339Nano, o.Provenance.Ended)
	if e != nil {
		return e
	}
	key := groupContextKey(o.Provenance)
	for _, g := range groups {
		if _, e = tx.ExecContext(ctx, "INSERT OR IGNORE INTO device_context_samples VALUES(?,?,?,?,?,?,?)", o.DeviceID, key, o.ID, at.Unix(), g.Service, *g.VLAN, g.Count); e != nil {
			return e
		}
	}
	return nil
}
func contextPeakGroups(ctx context.Context, db *sql.DB, o *equipment.PPObservation, now time.Time) ([]equipment.PPGroup, bool, error) {
	current, known := knownOnlineGroups(o)
	if !known {
		return nil, false, nil
	}
	q, e := db.QueryContext(ctx, "SELECT service,vlan,max(sessions) FROM device_context_samples WHERE device_id=? AND context=? AND at>=? AND at<=? GROUP BY service,vlan ORDER BY service,vlan", o.DeviceID, groupContextKey(o.Provenance), now.Add(-7*24*time.Hour).Unix(), now.Unix())
	if e != nil {
		return nil, false, e
	}
	defer q.Close()
	for q.Next() {
		var service string
		var vlan, peak int
		if e = q.Scan(&service, &vlan, &peak); e != nil {
			return nil, false, e
		}
		key := fmt.Sprintf("%d:%s", vlan, service)
		g := current[key]
		g.Service = service
		v := vlan
		g.VLAN = &v
		p := peak
		g.Peak = &p
		current[key] = g
	}
	if e = q.Err(); e != nil {
		return nil, false, e
	}
	var limited int64
	e = db.QueryRowContext(ctx, "SELECT pruned_at FROM device_monitoring_limits WHERE singleton=1").Scan(&limited)
	if e != nil && e != sql.ErrNoRows {
		return nil, false, e
	}
	out := []equipment.PPGroup{}
	for _, g := range current {
		g.Limited = limited > 0
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Service == out[j].Service {
			return *out[i].VLAN < *out[j].VLAN
		}
		return out[i].Service < out[j].Service
	})
	return out, true, nil
}
