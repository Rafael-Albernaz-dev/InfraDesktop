package equipment

import (
	"context"
	"strings"
	"time"
)

// RadioFamily is the driver identity proved by a reading, never a caller-selected guess.
type RadioFamily string

// Role identifies the operational capability of an inventory device.
type Role string

const (
	RoleAP                  Role        = "ap"
	RoleCPE                 Role        = "cpe"
	RolePTP                 Role        = "ptp"
	RoleConcentrator        Role        = "concentrator"
	RadioAuto               RadioFamily = "auto"
	RadioStandalone         RadioFamily = "auto-avulso"
	RadioUnsupported        RadioFamily = "unsupported"
	RadioRouterOSWirelessV6 RadioFamily = "routeros-wireless-v6"
	RadioAirOSV6            RadioFamily = "airos-v6"
	RadioIntelbrasWOM       RadioFamily = "intelbras-wom"
)

// RadioMode selects the bounded radio use case.
type RadioMode string

const (
	RadioOnce      RadioMode = "once"
	RadioMeasure   RadioMode = "measure"
	RadioLive      RadioMode = "live"
	RadioModeIndex RadioMode = "index"
)

// Transport is the inventory connection protocol, independent of its adapter.
type Transport string

const (
	TransportSSH    Transport = "ssh"
	RadioMaxClients           = 512
	RadioMaxCPU               = 90
	RadioPageSize             = 8
	RadioIndexTTL             = 5 * time.Minute
)

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
	CPE          string            `json:"cpe_id,omitempty"`
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

type RadioTickReceipt struct {
	Failure *RadioFailure `json:"failure,omitempty"`
	At      string        `json:"observed_at"`
	CPU     []int         `json:"cpu"`
	AP      float64       `json:"ap_command_seconds"`
	CPE     float64       `json:"cpe_command_seconds"`
	ICMP    []float64     `json:"icmp_rtt_ms"`
	Reason  string        `json:"reason"`
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

type RadioIndexEntry struct {
	AP        string   `json:"ap_id"`
	MAC       string   `json:"mac"`
	Interface string   `json:"interface"`
	BSSID     string   `json:"bssid"`
	LastIP    string   `json:"last_ip,omitempty"`
	Signal    *float64 `json:"signal_dbm,omitempty"`
}

type RadioIndex struct {
	Total    int               `json:"total"`
	Offset   int               `json:"offset"`
	More     bool              `json:"more"`
	Next     int               `json:"next_offset"`
	Labels   map[string]string `json:"labels,omitempty"`
	At       string            `json:"observed_at"`
	Complete bool              `json:"complete"`
	Entries  []RadioIndexEntry `json:"-"`
	Targets  map[string]Code   `json:"targets"`
	Context  string            `json:"-"`
}

type RadioProposal struct {
	APName          string          `json:"ap_name"`
	APAddress       string          `json:"ap_address"`
	CredentialAlias string          `json:"credential_alias"`
	Choice          RadioLinkChoice `json:"choice"`
	Address         string          `json:"address"`
	Family          RadioFamily     `json:"family"`
	CredentialID    string          `json:"credential_id"`
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
	ReadRadio(
		context.Context, Device, string, string, RadioFamily, func() Code,
	) (*RadioReading, Code)
}

type RadioIndexRemote interface {
	ReadRadioIndex(context.Context, Device, string, string, func() Code) ([]RadioIndexEntry, Code)
}

func IsRadioFamily(f RadioFamily) bool {
	return f == RadioRouterOSWirelessV6 ||
		f == RadioAirOSV6 ||
		f == RadioIntelbrasWOM ||
		f == RadioUnsupported
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
