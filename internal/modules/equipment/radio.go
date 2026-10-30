package equipment

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const RadioMaxClients = 512
const RadioMaxCPU = 90
const RadioPageSize = 8

type RadioMetric struct {
	Label       string   `json:"label,omitempty"`
	Name        string   `json:"name"`
	Value       float64  `json:"value"`
	Unit        string   `json:"unit"`
	Direction   string   `json:"direction"`
	Perspective string   `json:"perspective"`
	DeviceID    string   `json:"device_id"`
	Interface   string   `json:"interface"`
	At          string   `json:"observed_at"`
	Quality     string   `json:"quality"`
	Best        *float64 `json:"best,omitempty"`
	Worst       *float64 `json:"worst,omitempty"`
}
type RadioInterface struct {
	HadClients      bool          `json:"had_clients"`
	Name            string        `json:"name"`
	Master          string        `json:"master,omitempty"`
	BSSID           string        `json:"bssid"`
	Band            string        `json:"band,omitempty"`
	ConfiguredWidth string        `json:"configured_width,omitempty"`
	Channel         string        `json:"channel,omitempty"`
	Power           string        `json:"power,omitempty"`
	PowerMetrics    []RadioMetric `json:"power_metrics,omitempty"`
	Clients         *int          `json:"clients,omitempty"`
	Metrics         []RadioMetric `json:"metrics"`
}
type RadioClient struct {
	MAC          string        `json:"mac"`
	Name         string        `json:"name"`
	Interface    string        `json:"interface"`
	BSSID        string        `json:"bssid"`
	Uptime       string        `json:"wireless_uptime,omitempty"`
	LastActivity string        `json:"last_activity,omitempty"`
	Metrics      []RadioMetric `json:"metrics"`
}
type RadioReading struct {
	DeviceName         string           `json:"device_name"`
	Address            string           `json:"address"`
	PingRTT            *float64         `json:"ping_rtt_ms,omitempty"`
	Compatible         bool             `json:"compatible"`
	ID                 string           `json:"id"`
	DeviceID           string           `json:"device_id"`
	Family             string           `json:"family"`
	Version            string           `json:"version"`
	Perspective        string           `json:"perspective"`
	At                 string           `json:"observed_at"`
	Revision           uint64           `json:"revision"`
	CredentialRevision uint64           `json:"credential_revision"`
	Connection         string           `json:"connection"`
	Legacy             []string         `json:"ssh_legacy,omitempty"`
	Interfaces         []RadioInterface `json:"interfaces"`
	Clients            []RadioClient    `json:"clients"`
	Metrics            []RadioMetric    `json:"metrics"`
	WirelessUptime     string           `json:"wireless_uptime,omitempty"`
	DeviceUptime       string           `json:"device_uptime,omitempty"`
	APBSSID            string           `json:"ap_bssid,omitempty"`
	WirelessMAC        string           `json:"wireless_mac,omitempty"`
	CPU                []int            `json:"cpu_before_during_after,omitempty"`
	Seconds            float64          `json:"seconds"`
	CommandSeconds     float64          `json:"command_seconds"`
	HadClients         bool             `json:"had_clients"`
}
type RadioProfile struct {
	DeviceID           string `json:"device_id"`
	Family             string `json:"family"`
	Confirmed          bool   `json:"confirmed"`
	Revision           uint64 `json:"revision"`
	CredentialRevision uint64 `json:"credential_revision"`
	Connection         string `json:"connection"`
	Warning            int    `json:"warning"`
	Critical           int    `json:"critical"`
}
type RadioLink struct {
	Concentrator string `json:"concentrator_id"`
	Scope        string `json:"scope"`
	Login        string `json:"login"`
	CPE          string `json:"cpe_id"`
	AP           string `json:"ap_id"`
	MAC          string `json:"mac"`
	Interface    string `json:"interface"`
	BSSID        string `json:"bssid"`
	Evidence     string `json:"evidence"`
	Origin       string `json:"origin"`
	At           string `json:"confirmed_at"`
	ConfirmedNow bool   `json:"confirmed_now"`
}
type RadioInput struct {
	DeviceID      string           `json:"device_id,omitempty"`
	Revision      uint64           `json:"revision,omitempty"`
	Operation     string           `json:"operation_id,omitempty"`
	JobID         string           `json:"job_id,omitempty"`
	ObservationID string           `json:"observation_id,omitempty"`
	RowID         string           `json:"row_id,omitempty"`
	Family        string           `json:"family,omitempty"`
	Offset        int              `json:"offset,omitempty"`
	ReadingID     string           `json:"reading_id,omitempty"`
	Link          *RadioLinkChoice `json:"link,omitempty"`
	Warning       int              `json:"warning,omitempty"`
	Critical      int              `json:"critical,omitempty"`
	Mode          string           `json:"mode,omitempty"`
}
type RadioLinkChoice struct {
	CPE       string `json:"cpe_id"`
	AP        string `json:"ap_id"`
	MAC       string `json:"mac"`
	Interface string `json:"interface"`
	BSSID     string `json:"bssid"`
	Evidence  string `json:"evidence"`
	Origin    string `json:"origin"`
}
type RadioTiming struct {
	CommandSeconds float64 `json:"command_seconds"`
	PathSeconds    float64 `json:"path_seconds"`
}
type RadioCost struct {
	Measurements []RadioMeasurement `json:"measurements,omitempty"`
	PingBaseline float64            `json:"ping_baseline_ms"`
	PingLimit    float64            `json:"ping_limit_ms"`
	CPUBaseline  float64            `json:"cpu_baseline"`
	Samples      []float64          `json:"samples"`
	P95          float64            `json:"p95_seconds"`
	Interval     float64            `json:"interval_seconds"`
	Enabled      bool               `json:"enabled"`
	Reason       string             `json:"reason"`
	Context      string             `json:"-"`
	Load         []RadioLoad        `json:"load,omitempty"`
}
type RadioLoad struct {
	CPU          []int     `json:"cpu"`
	PingSent     int       `json:"ping_sent"`
	PingReceived int       `json:"ping_received"`
	PingRTT      []float64 `json:"ping_rtt_ms"`
	Started      time.Time `json:"started_at"`
	Ended        time.Time `json:"ended_at"`
}
type RadioLoadRemote interface {
	ObserveRadioLoad(context.Context, Device, string, string, func() Code, func(), <-chan struct{}) (RadioLoad, Code)
}
type RadioMeasurement struct {
	Seconds  float64     `json:"seconds"`
	AP       RadioTiming `json:"ap"`
	CPE      RadioTiming `json:"cpe"`
	Started  time.Time   `json:"started_at"`
	Ended    time.Time   `json:"ended_at"`
	Complete bool        `json:"complete"`
	Load     RadioLoad   `json:"load"`
}

func radioFinite(n float64) bool    { return !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 }
func radioSeconds(n float64) string { return strings.ReplaceAll(fmt.Sprintf("%.2f", n), ".", ",") }
func AssessRadioMeasurements(samples []RadioMeasurement) RadioCost {
	cost := RadioCost{Interval: 5, Reason: "Custo ainda não medido em dez leituras.", Measurements: append([]RadioMeasurement{}, samples...)}
	var times, bases, cpus []float64
	for _, sample := range samples {
		cost.Samples = append(cost.Samples, sample.Seconds)
		cost.Load = append(cost.Load, sample.Load)
		if radioFinite(sample.Seconds) && sample.Seconds > 0 {
			times = append(times, sample.Seconds)
		}
		if len(sample.Load.PingRTT) > 0 && radioFinite(sample.Load.PingRTT[0]) {
			bases = append(bases, sample.Load.PingRTT[0])
		}
		if len(sample.Load.CPU) > 0 && sample.Load.CPU[0] >= 0 && sample.Load.CPU[0] <= 100 {
			cpus = append(cpus, float64(sample.Load.CPU[0]))
		}
	}
	if len(times) > 0 {
		sort.Float64s(times)
		cost.P95 = times[int(math.Ceil(.95*float64(len(times))))-1]
		cost.Interval = math.Max(5, 3*cost.P95)
	}
	if len(samples) != 10 {
		return cost
	}
	cost.Reason = ""
	refuse := func(reason string) {
		if cost.Reason == "" {
			cost.Reason = reason
		}
	}
	for _, sample := range samples {
		if !sample.Complete {
			refuse("Amostra de leitura falhou; live desligado.")
		}
	}
	for _, side := range []struct {
		name string
		get  func(RadioMeasurement) RadioTiming
	}{
		{"AP", func(s RadioMeasurement) RadioTiming { return s.AP }}, {"CPE", func(s RadioMeasurement) RadioTiming { return s.CPE }},
	} {
		for _, sample := range samples {
			timing := side.get(sample)
			if timing.CommandSeconds > 3 {
				article := "do AP"
				if side.name == "CPE" {
					article = "da CPE"
				}
				refuse("Comando " + article + " levou " + radioSeconds(timing.CommandSeconds) + " s (limite 3 s); live desligado.")
			}
		}
	}
	if len(cpus) == 10 {
		sort.Float64s(cpus)
		cost.CPUBaseline = (cpus[4] + cpus[5]) / 2
	}
	for _, sample := range samples {
		for _, cpu := range sample.Load.CPU {
			if cpu <= 100 && cpu > RadioMaxCPU {
				refuse(fmt.Sprintf("CPU do AP chegou a %d%% (limite %d%%); live desligado.", cpu, RadioMaxCPU))
			}
		}
	}
	if len(bases) == 10 {
		sort.Float64s(bases)
		cost.PingBaseline = (bases[4] + bases[5]) / 2
		cost.PingLimit = 2 * cost.PingBaseline
		consecutive := 0
		for _, sample := range samples {
			if len(sample.Load.PingRTT) == 0 {
				continue
			}
			for _, rtt := range sample.Load.PingRTT[1:] {
				if radioFinite(rtt) && rtt > cost.PingLimit {
					consecutive++
				} else {
					consecutive = 0
				}
				if consecutive >= 2 {
					refuse("Ping degradou: " + radioSeconds(rtt) + " ms, duas latências consecutivas acima de " + radioSeconds(cost.PingLimit) + " ms (2× a mediana); live desligado.")
				}
			}
		}
	}
	for _, sample := range samples {
		load := sample.Load
		if load.PingSent > 0 && load.PingReceived >= 0 && load.PingReceived < load.PingSent {
			refuse(fmt.Sprintf("Ping perdido: %d de %d respostas; live desligado.", load.PingReceived, load.PingSent))
		}
	}
	for _, sample := range samples {
		load := sample.Load
		valid := radioFinite(sample.Seconds) && sample.Seconds > 0 && !load.Started.IsZero() && !load.Started.After(sample.Started) && !load.Ended.Before(sample.Ended) && len(load.CPU) >= 3 && load.PingSent >= 3 && load.PingReceived == load.PingSent && len(load.PingRTT) == load.PingReceived
		for _, timing := range []RadioTiming{sample.AP, sample.CPE} {
			valid = valid && radioFinite(timing.CommandSeconds) && timing.CommandSeconds > 0 && radioFinite(timing.PathSeconds) && timing.PathSeconds >= timing.CommandSeconds
		}
		for _, rtt := range load.PingRTT {
			valid = valid && radioFinite(rtt)
		}
		for _, cpu := range load.CPU {
			valid = valid && cpu >= 0 && cpu <= 100
		}
		if !valid {
			refuse("Carga incompleta: CPU, ping ou janela não cobrem a leitura; live desligado.")
		}
	}
	if cost.Reason == "" {
		cost.Enabled = true
		cost.Reason = "Custo medido em dez leituras; comandos AP/CPE até 3 s e CPU do AP até 90%."
	}
	return cost
}
func RadioLiveLimit(reading *RadioReading, cost RadioCost) bool {
	if reading == nil || !radioFinite(reading.CommandSeconds) || reading.CommandSeconds <= 0 || reading.CommandSeconds > 3 || reading.PingRTT == nil || !radioFinite(*reading.PingRTT) || *reading.PingRTT > cost.PingLimit || len(reading.CPU) < 3 {
		return true
	}
	for _, cpu := range reading.CPU {
		if cpu < 0 || cpu > RadioMaxCPU {
			return true
		}
	}
	return false
}

// A full path may exceed three seconds; only each established-session command
// is limited. The caller retains the two-consecutive-violations state machine.
func RadioLiveTickViolation(ap, cpe *RadioReading, cost RadioCost) bool {
	return RadioLiveLimit(ap, cost) || cpe == nil || !radioFinite(cpe.CommandSeconds) || cpe.CommandSeconds <= 0 || cpe.CommandSeconds > 3
}

type RadioView struct {
	Reading     *RadioReading `json:"reading,omitempty"`
	CPEReading  *RadioReading `json:"cpe_reading,omitempty"`
	Profile     *RadioProfile `json:"profile,omitempty"`
	Link        *RadioLink    `json:"link,omitempty"`
	Suggestions []RadioLink   `json:"suggestions,omitempty"`
	Job         *Job          `json:"job,omitempty"`
	APCode      Code          `json:"ap_code,omitempty"`
	CPECode     Code          `json:"cpe_code,omitempty"`
	Cost        *RadioCost    `json:"cost,omitempty"`
	Total       int           `json:"total"`
	More        bool          `json:"more"`
	NextOffset  int           `json:"next_offset"`
}
type RadioStore interface {
	RadioCurrent(context.Context, string, string) (*RadioReading, *RadioProfile, Code)
	SaveRadio(context.Context, string, Device, uint64, *RadioReading) Code
	ConfirmRadioFamily(context.Context, string, Device, uint64, string, int, int) Code
	RadioLinks(context.Context, string, string, string) ([]RadioLink, Code)
	SaveRadioLink(context.Context, string, RadioLink, string, PPInput) Code
	RadioAPReadings(context.Context, string) ([]RadioReading, Code)
}
type RadioRemote interface {
	ReadRadio(context.Context, Device, string, string, string, func() Code) (*RadioReading, Code)
}

func RadioFamily(f string) bool {
	return f == "routeros-wireless-v6" || f == "airos-v6" || f == "intelbras-wom" || f == "unsupported"
}
func RadioMAC(s string) (string, bool) {
	s = strings.ToUpper(s)
	if len(s) != 17 {
		return "", false
	}
	for i, c := range s {
		if i%3 == 2 {
			if c != ':' {
				return "", false
			}
		} else if !strings.ContainsRune("0123456789ABCDEF", c) {
			return "", false
		}
	}
	if s == "00:00:00:00:00:00" || s == "FF:FF:FF:FF:FF:FF" {
		return "", false
	}
	return s, true
}
func RadioLinkPresent(link RadioLink, reading *RadioReading) bool {
	if reading == nil || !reading.Compatible || reading.DeviceID != link.AP {
		return false
	}
	for _, c := range reading.Clients {
		if c.MAC == link.MAC && c.Interface == link.Interface && c.BSSID == link.BSSID {
			return true
		}
	}
	return false
}
func RadioCostGate(samples []float64, cpu [][]int) RadioCost {
	c := RadioCost{Samples: append([]float64{}, samples...), Interval: 5, Reason: "Custo ainda não medido no caminho completo."}
	if len(samples) != 10 || len(cpu) != 10 {
		return c
	}
	times := append([]float64{}, samples...)
	for i, seconds := range times {
		if math.IsNaN(seconds) || math.IsInf(seconds, 0) || seconds <= 0 || len(cpu[i]) < 3 {
			return c
		}
		for _, n := range cpu[i] {
			if n < 0 || n > 100 {
				return c
			}
		}
	}
	sort.Float64s(times)
	c.P95 = times[int(math.Ceil(.95*float64(len(times))))-1]
	c.Interval = math.Max(5, 3*c.P95)
	for i := range samples {
		for _, n := range cpu[i] {
			if n > RadioMaxCPU {
				c.Reason = "CPU do AP passou de 90%; use consulta pontual."
				return c
			}
		}
	}
	c.Enabled, c.Reason = true, "Custo medido em dez leituras."
	return c
}
func StampRadio(reading *RadioReading, device Device, credential uint64, at time.Time) {
	reading.DeviceName, reading.Address = device.Name, device.Address
	reading.Compatible = true
	reading.DeviceID, reading.Revision, reading.CredentialRevision, reading.Connection, reading.At = device.ID, device.Revision, credential, PPConnection(device), PPTime(at)
	stamp := func(metrics []RadioMetric, iface string) {
		for i := range metrics {
			m := &metrics[i]
			m.Perspective, m.DeviceID, m.At, m.Quality, m.Interface = reading.Perspective, device.ID, reading.At, "observed", iface
			if strings.HasPrefix(m.Name, "chain") {
				v := m.Value
				m.Best, m.Worst = &v, &v
			}
		}
	}
	stamp(reading.Metrics, "wireless")
	for i := range reading.Interfaces {
		stamp(reading.Interfaces[i].Metrics, reading.Interfaces[i].Name)
		stamp(reading.Interfaces[i].PowerMetrics, reading.Interfaces[i].Name)
	}
	for i := range reading.Clients {
		stamp(reading.Clients[i].Metrics, reading.Clients[i].Interface)
	}
}

// Extrema belong to this finite observation only; PHY rates never form a series.
func AccumulateRadioChains(previous, next *RadioReading) {
	if previous == nil || next == nil || previous.DeviceID != next.DeviceID || previous.Connection != next.Connection || previous.Revision != next.Revision || previous.CredentialRevision != next.CredentialRevision {
		return
	}
	merge := func(before, after []RadioMetric) {
		for i := range after {
			m := &after[i]
			if !strings.HasPrefix(m.Name, "chain") {
				continue
			}
			for _, old := range before {
				if old.Name == m.Name && old.Direction == m.Direction && old.Interface == m.Interface {
					best, worst := m.Value, m.Value
					if old.Best != nil {
						best = math.Max(best, *old.Best)
					}
					if old.Worst != nil {
						worst = math.Min(worst, *old.Worst)
					}
					m.Best, m.Worst = &best, &worst
				}
			}
		}
	}
	merge(previous.Metrics, next.Metrics)
	for i := range next.Clients {
		for _, old := range previous.Clients {
			if old.MAC == next.Clients[i].MAC && old.Interface == next.Clients[i].Interface && old.BSSID == next.Clients[i].BSSID {
				merge(old.Metrics, next.Clients[i].Metrics)
			}
		}
	}
}
