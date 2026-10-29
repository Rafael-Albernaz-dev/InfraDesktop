package equipment

import "time"

type PPPeerSession struct {
	DeviceID string     `json:"device_id"`
	Name     string     `json:"name"`
	Endpoint string     `json:"endpoint"`
	Address  string     `json:"address"`
	Caller   string     `json:"caller_id"`
	Uptime   PPInterval `json:"uptime"`
	At       string     `json:"at"`
}
type PPCrossDuplicate struct {
	A PPPeerSession `json:"a"`
	B PPPeerSession `json:"b"`
}

func ComparableConcentratorReadings(a, b *PPObservation, now time.Time) bool {
	if a == nil || b == nil || a.Quality.Status != "complete" || b.Quality.Status != "complete" {
		return false
	}
	a0, e0 := time.Parse(time.RFC3339Nano, a.Provenance.Started)
	a1, e1 := time.Parse(time.RFC3339Nano, a.Provenance.Ended)
	b0, e2 := time.Parse(time.RFC3339Nano, b.Provenance.Started)
	b1, e3 := time.Parse(time.RFC3339Nano, b.Provenance.Ended)
	return e0 == nil && e1 == nil && e2 == nil && e3 == nil && !a1.Before(a0) && !b1.Before(b0) && !a1.After(now) && !b1.After(now) && now.Sub(a1) <= 15*time.Minute && now.Sub(b1) <= 15*time.Minute && !a1.Before(b0) && !b1.Before(a0)
}
