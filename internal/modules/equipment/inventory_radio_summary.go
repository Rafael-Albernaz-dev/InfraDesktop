package equipment

// RadioSummary is the bounded, local-only projection used by the inventory.
// Missing measurements stay absent; individual client identities never appear.
type RadioSummary struct {
	At          string   `json:"observed_at"`
	Version     string   `json:"version,omitempty"`
	Model       string   `json:"model,omitempty"`
	Uptime      string   `json:"uptime,omitempty"`
	CPU         *int     `json:"cpu,omitempty"`
	Memory      *float64 `json:"memory,omitempty"`
	Voltage     *float64 `json:"voltage,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	Clients     *int     `json:"clients,omitempty"`
}

func ProjectRadioSummary(r RadioReading) *RadioSummary {
	p := &RadioSummary{At: r.At, Version: r.Version, Model: r.Model, Uptime: r.DeviceUptime}
	if len(r.CPU) != 0 {
		cpu := r.CPU[len(r.CPU)-1]
		p.CPU = &cpu
	}
	if r.Perspective == "AP" {
		clients := len(r.Clients)
		p.Clients = &clients
	}
	for _, m := range r.Metrics {
		if m.Quality != "observed" {
			continue
		}
		value := m.Value
		switch m.Name {
		case "memory":
			p.Memory = &value
		case "voltage":
			p.Voltage = &value
		case "temperature":
			p.Temperature = &value
		}
	}
	return p
}
