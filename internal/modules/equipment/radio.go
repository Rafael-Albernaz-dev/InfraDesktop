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
	Counters     map[string]string `json:"counters,omitempty"`
	MAC          string            `json:"mac"`
	Name         string            `json:"name"`
	Interface    string            `json:"interface"`
	BSSID        string            `json:"bssid"`
	Uptime       string            `json:"wireless_uptime,omitempty"`
	LastActivity string            `json:"last_activity,omitempty"`
	Metrics      []RadioMetric     `json:"metrics"`
}
type RadioFailure struct {
	Reason string `json:"reason"`
	Bytes  int    `json:"bytes"`
}

type RadioReading struct {
	Failure            *RadioFailure     `json:"failure,omitempty"`
	Model              string            `json:"model,omitempty"`
	Manufacturer       string            `json:"manufacturer,omitempty"`
	DriverReason       string            `json:"driver_reason,omitempty"`
	Mode               string            `json:"mode,omitempty"`
	WirelessProtocol   string            `json:"wireless_protocol,omitempty"`
	Scope              string            `json:"scope,omitempty"`
	Counters           map[string]string `json:"counters,omitempty"`
	DeviceName         string            `json:"device_name"`
	Address            string            `json:"address"`
	PingRTT            *float64          `json:"ping_rtt_ms,omitempty"`
	Compatible         bool              `json:"compatible"`
	ID                 string            `json:"id"`
	DeviceID           string            `json:"device_id"`
	Family             RadioFamily       `json:"family"`
	Version            string            `json:"version"`
	Perspective        string            `json:"perspective"`
	At                 string            `json:"observed_at"`
	Revision           uint64            `json:"revision"`
	CredentialRevision uint64            `json:"credential_revision"`
	Connection         string            `json:"connection"`
	Legacy             []string          `json:"ssh_legacy,omitempty"`
	Negotiated         []string          `json:"ssh_negotiated,omitempty"`
	Interfaces         []RadioInterface  `json:"interfaces"`
	Clients            []RadioClient     `json:"clients"`
	Metrics            []RadioMetric     `json:"metrics"`
	WirelessUptime     string            `json:"wireless_uptime,omitempty"`
	DeviceUptime       string            `json:"device_uptime,omitempty"`
	APBSSID            string            `json:"ap_bssid,omitempty"`
	WirelessMAC        string            `json:"wireless_mac,omitempty"`
	CPU                []int             `json:"cpu_before_during_after,omitempty"`
	Seconds            float64           `json:"seconds"`
	CommandSeconds     float64           `json:"command_seconds"`
	HadClients         bool              `json:"had_clients"`
}
type RadioProfile struct {
	DeviceID           string      `json:"device_id"`
	Family             RadioFamily `json:"family"`
	Confirmed          bool        `json:"confirmed"`
	Revision           uint64      `json:"revision"`
	CredentialRevision uint64      `json:"credential_revision"`
	Connection         string      `json:"connection"`
	Warning            int         `json:"warning"`
	Critical           int         `json:"critical"`
}
type RadioLink struct {
	NewCPE       *Device `json:"-"`
	Concentrator string  `json:"concentrator_id"`
	Scope        string  `json:"scope"`
	Login        string  `json:"login"`
	CPE          string  `json:"cpe_id"`
	AP           string  `json:"ap_id"`
	MAC          string  `json:"mac"`
	Interface    string  `json:"interface"`
	BSSID        string  `json:"bssid"`
	Evidence     string  `json:"evidence"`
	Origin       string  `json:"origin"`
	At           string  `json:"confirmed_at"`
	ConfirmedNow bool    `json:"confirmed_now"`
}
type RadioInput struct {
	DeviceID      string           `json:"device_id,omitempty"`
	Revision      uint64           `json:"revision,omitempty"`
	Operation     string           `json:"operation_id,omitempty"`
	JobID         string           `json:"job_id,omitempty"`
	ObservationID string           `json:"observation_id,omitempty"`
	RowID         string           `json:"row_id,omitempty"`
	Family        RadioFamily      `json:"family,omitempty"`
	Offset        int              `json:"offset,omitempty"`
	ReadingID     string           `json:"reading_id,omitempty"`
	Link          *RadioLinkChoice `json:"link,omitempty"`
	Warning       int              `json:"warning,omitempty"`
	Critical      int              `json:"critical,omitempty"`
	Mode          RadioMode        `json:"mode,omitempty"`
}
type RadioLinkChoice struct {
	CredentialID string `json:"credential_id,omitempty"`
	CPE          string `json:"cpe_id"`
	AP           string `json:"ap_id"`
	MAC          string `json:"mac"`
	Interface    string `json:"interface"`
	BSSID        string `json:"bssid"`
	Evidence     string `json:"evidence"`
	Origin       string `json:"origin"`
}
type RadioTiming struct {
	CommandSeconds float64 `json:"command_seconds"`
	PathSeconds    float64 `json:"path_seconds"`
}
type RadioCost struct {
	APOnly       bool               `json:"ap_only,omitempty"`
	Series       []RadioTickReceipt `json:"series,omitempty"`
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
type RadioMeasurement struct {
	APOnly   bool        `json:"ap_only,omitempty"`
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
func RadioLiveLimit(reading *RadioReading, cost RadioCost) bool {
	if reading == nil ||
		!radioFinite(reading.CommandSeconds) ||
		reading.CommandSeconds <= 0 ||
		reading.CommandSeconds > 3 ||
		reading.PingRTT == nil ||
		!radioFinite(*reading.PingRTT) ||
		*reading.PingRTT > cost.PingLimit ||
		len(reading.CPU) < 1 {
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
	Index       *RadioIndex    `json:"index,omitempty"`
	Proposal    *RadioProposal `json:"proposal,omitempty"`
	MatchReason string         `json:"match_reason,omitempty"`
	CPENote     string         `json:"cpe_note,omitempty"`
	Reading     *RadioReading  `json:"reading,omitempty"`
	CPEReading  *RadioReading  `json:"cpe_reading,omitempty"`
	Profile     *RadioProfile  `json:"profile,omitempty"`
	Link        *RadioLink     `json:"link,omitempty"`
	Suggestions []RadioLink    `json:"suggestions,omitempty"`
	Job         *Job           `json:"job,omitempty"`
	APCode      Code           `json:"ap_code,omitempty"`
	CPECode     Code           `json:"cpe_code,omitempty"`
	Cost        *RadioCost     `json:"cost,omitempty"`
	Total       int            `json:"total"`
	More        bool           `json:"more"`
	NextOffset  int            `json:"next_offset"`
}
type RadioStore interface {
	RadioCurrent(context.Context, string, string) (*RadioReading, *RadioProfile, Code)
	SaveRadio(context.Context, string, Device, uint64, *RadioReading) Code
	ConfirmRadioFamily(context.Context, string, Device, uint64, RadioFamily, int, int) Code
	RadioLinks(context.Context, string, string, string) ([]RadioLink, Code)
	SaveRadioLink(context.Context, string, RadioLink, string, PPInput) Code
	RadioAPReadings(context.Context, string) ([]RadioReading, Code)
}
type RadioRemote interface {
	ReadRadio(context.Context, Device, string, string, RadioFamily, func() Code) (*RadioReading, Code)
}

func IsRadioFamily(f RadioFamily) bool {
	return f == RadioRouterOSWirelessV6 || f == RadioAirOSV6 || f == RadioIntelbrasWOM || f == RadioUnsupported
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
	reading.DeviceID,
		reading.Revision,
		reading.CredentialRevision,
		reading.Connection,
		reading.At = device.ID,
		device.Revision,
		credential,
		PPConnection(device),
		PPTime(at)
	stamp := func(metrics []RadioMetric, iface string) {
		for i := range metrics {
			m := &metrics[i]
			m.Perspective, m.DeviceID, m.Interface = reading.Perspective, device.ID, iface
			if m.Quality != "initial" || m.At == "" {
				m.At = reading.At
			}
			if m.Quality == "" {
				m.Quality = "observed"
			}
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
	if previous == nil ||
		next == nil ||
		previous.DeviceID != next.DeviceID ||
		previous.Connection != next.Connection ||
		previous.Revision != next.Revision ||
		previous.CredentialRevision != next.CredentialRevision {
		return
	}
	mergeRadioChainExtrema(previous.Metrics, next.Metrics)
	for i := range next.Clients {
		for _, old := range previous.Clients {
			if old.MAC == next.Clients[i].MAC && old.Interface == next.Clients[i].Interface && old.BSSID == next.Clients[i].BSSID {
				mergeRadioChainExtrema(old.Metrics, next.Clients[i].Metrics)
			}
		}
	}
}

func mergeRadioChainExtrema(before, after []RadioMetric) {
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
