package equipment

import (
	"context"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type PPContext struct {
	ProfileUploadLimit   string `json:"profile_upload_limit,omitempty"`
	ProfileDownloadLimit string `json:"profile_download_limit,omitempty"`
	Association          string `json:"association"`
	Service              string `json:"service,omitempty"`
	Interface            string `json:"interface,omitempty"`
	VLAN                 *int   `json:"vlan,omitempty"`
	IPv6                 *bool  `json:"ipv6,omitempty"`
	DelegatedPrefix      string `json:"delegated_prefix,omitempty"`
	PDStatus             string `json:"pd_status,omitempty"`
	Profile              string `json:"profile,omitempty"`
	SecretState          string `json:"secret_state,omitempty"`
	UploadLimit          string `json:"upload_limit,omitempty"`
	DownloadLimit        string `json:"download_limit,omitempty"`
}

type PPTunnel struct {
	ID, Name, Login, Caller, Address string
	Uptime                           PPInterval
	Context                          PPContext
	RouterCPU                        *uint64
	RX, TX                           *uint64
	Sample, RouterUptime             PPInterval
}

type PPRate struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

type PPTrafficPoint struct {
	T   int64    `json:"t"`
	D   *float64 `json:"d,omitempty"`
	U   *float64 `json:"u,omitempty"`
	Gap bool     `json:"gap,omitempty"`
}

type PPTraffic struct {
	Points          []PPTrafficPoint `json:"points"`
	DownloadBytes   string           `json:"download_bytes"`
	UploadBytes     string           `json:"upload_bytes"`
	CoveredNS       int64            `json:"covered_ns"`
	CoveredMinNS    int64            `json:"covered_min_ns"`
	Gaps            int              `json:"gaps"`
	Overflow        bool             `json:"overflow,omitempty"`
	AverageDownload *PPRate          `json:"average_download,omitempty"`
	AverageUpload   *PPRate          `json:"average_upload,omitempty"`
	PeakDownload    *PPRate          `json:"peak_download,omitempty"`
	PeakUpload      *PPRate          `json:"peak_upload,omitempty"`
}

type TrafficLoadSafety struct {
	HealthDuration         time.Duration
	CPU                    *uint64
	PingSent, PingReceived int
	TimedOut               bool
}

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

type DevicePort struct {
	Running    *bool   `json:"running,omitempty"`
	SpeedBPS   *uint64 `json:"speed_bps,omitempty"`
	FullDuplex *bool   `json:"full_duplex,omitempty"`
	LinkDowns  *string `json:"link_downs,omitempty"`
}

type DeviceHealth struct {
	FastTrack   *uint64    `json:"fasttrack_rules,omitempty"`
	Started     string     `json:"started_at"`
	At          string     `json:"at"`
	CPU         *uint64    `json:"cpu,omitempty"`
	FreeMemory  *uint64    `json:"free_memory,omitempty"`
	TotalMemory *uint64    `json:"total_memory,omitempty"`
	Uptime      PPInterval `json:"uptime"`
	Version     string     `json:"version,omitempty"`
	Temperature *float64   `json:"temperature,omitempty"`
	ClockOffset PPInterval `json:"clock_offset"`
	NTPEnabled  *bool      `json:"ntp_enabled,omitempty"`
	NTPStatus   string     `json:"ntp_status,omitempty"`
	ErrorCount  *uint64    `json:"error_count,omitempty"`
}

type DeviceMonitoringPoint struct {
	At         string        `json:"at"`
	Resolution int           `json:"resolution"`
	SessionAt  string        `json:"session_at,omitempty"`
	OnlineMin  *int          `json:"online_min,omitempty"`
	OnlineMax  *int          `json:"online_max,omitempty"`
	Online     *int          `json:"online,omitempty"`
	Health     *DeviceHealth `json:"health,omitempty"`
	HealthAt   string        `json:"health_at,omitempty"`
	Gap        bool          `json:"gap"`
	Drop       bool          `json:"drop"`
	Samples    int           `json:"samples"`
}

type DeviceMonitoringStore interface {
	SaveDeviceMonitoring(context.Context, string, Device, uint64, *PPReadResult) Code
	DeviceMonitoring(context.Context, string, PPInput) (PPView, Code)
}

type PPSecretEvidence struct {
	Disabled   *bool   `json:"disabled,omitempty"`
	Service    *string `json:"service,omitempty"`
	Profile    *string `json:"profile,omitempty"`
	LastOut    *string `json:"last_logged_out,omitempty"`
	LastCaller *string `json:"last_caller_id,omitempty"`
	LastReason *string `json:"last_disconnect_reason,omitempty"`
	Comment    *string `json:"comment,omitempty"`
}

type PPReadRow struct {
	Duplicate  *PPCrossDuplicate      `json:"duplicate,omitempty"`
	Port       *DevicePort            `json:"port,omitempty"`
	BeforePort *DevicePort            `json:"before_port,omitempty"`
	Traffic    *PPTraffic             `json:"traffic,omitempty"`
	SourceAt   string                 `json:"source_at,omitempty"`
	Health     *DeviceHealth          `json:"health,omitempty"`
	Point      *DeviceMonitoringPoint `json:"point,omitempty"`
	Value      string                 `json:"value,omitempty"`
	LinkDowns  *string                `json:"link_downs,omitempty"`
	Secret     *PPSecretEvidence      `json:"secret,omitempty"`
	RowID      string                 `json:"row_id,omitempty"`
	Kind       string                 `json:"kind"`
	Name       string                 `json:"name,omitempty"`
	Login      string                 `json:"login,omitempty"`
	TX         *PPRate                `json:"tx,omitempty"`
	RX         *PPRate                `json:"rx,omitempty"`
	Running    *bool                  `json:"running,omitempty"`
	Disabled   *bool                  `json:"disabled,omitempty"`
	Download   *PPRate                `json:"download,omitempty"`
	Upload     *PPRate                `json:"upload,omitempty"`
	Uptime     PPInterval             `json:"uptime"`
	At         string                 `json:"at,omitempty"`
	Event      string                 `json:"event,omitempty"`
	Used       *uint64                `json:"used,omitempty"`
	Size       *uint64                `json:"size,omitempty"`
	RXErrors   *string                `json:"rx_errors,omitempty"`
	TXErrors   *string                `json:"tx_errors,omitempty"`
	RXDrops    *string                `json:"rx_drops,omitempty"`
	TXDrops    *string                `json:"tx_drops,omitempty"`
}

type PPReadCost struct {
	Handshake  *int64  `json:"ssh_handshake_ns,omitempty"`
	Collector  *int64  `json:"collector_ns,omitempty"`
	Commands   int     `json:"ssh_commands"`
	Bytes      int     `json:"ssh_bytes"`
	FirstCount int     `json:"first_tunnels"`
	LastCount  int     `json:"last_tunnels"`
	CPUStart   *uint64 `json:"cpu_start,omitempty"`
	CPUEnd     *uint64 `json:"cpu_end,omitempty"`
}

type PPReadResult struct {
	FailureReason     string            `json:"-"`
	VersionDivergence *bool             `json:"version_divergence,omitempty"`
	PackageDivergence *bool             `json:"package_divergence,omitempty"`
	ComparedDevices   int               `json:"compared_devices,omitempty"`
	Availability      map[string]string `json:"availability,omitempty"`
	PayloadBytes      int               `json:"-"`
	MaxRowBytes       int               `json:"-"`
	MetadataBytes     int               `json:"-"`
	PoolsAvailable    *bool             `json:"pools_available,omitempty"`
	UplinksAvailable  *bool             `json:"uplinks_available,omitempty"`
	PoolTotal         int               `json:"pool_total"`
	UplinkTotal       int               `json:"uplink_total"`
	Version           string            `json:"remote_version,omitempty"`
	Cost              PPReadCost        `json:"cost"`
	Scope             string            `json:"scope"`
	Started           string            `json:"started_at"`
	Ended             string            `json:"ended_at"`
	Duration          int64             `json:"duration_ns"`
	Rows              []PPReadRow       `json:"rows"`
}

type PPReadTarget struct {
	Observation PPObservation
	Rows        []PPRow
	Age         PPInterval
	Prepared    time.Time
}

type PPReadRequest struct {
	Device              Device
	User, Secret, Scope string
	Target              *PPReadTarget
	Publish             func(*PPReadResult) Code
	Check               func() Code
}

type PPReadRemote interface {
	ReadPP(
		context.Context, PPReadRequest,
	) (*PPReadResult, Code)
}

func PPConnection(d Device) string {
	parts := []string{string(d.Transport), d.Address, fmtPort(d.Port), d.Fingerprint, d.CredentialID}
	return strings.Join(parts, "\x00")
}

func fmtPort(p int) string { return strconv.Itoa(p) }

func Endpoint(d Device) string {
	ip, e := netip.ParseAddr(d.Address)
	if e != nil {
		return ""
	}
	return string(d.Transport) + "/" + ip.Unmap().String() + "/" + fmtPort(d.Port)
}

func PPValidQuery(q *PPQuery) bool {
	if q == nil {
		return true
	}
	if !utf8.ValidString(q.Type) || len(q.Type) > 64 || !utf8.ValidString(q.Search) || len(q.Search) > 160 {
		return false
	}
	if strings.IndexFunc(q.Type, func(r rune) bool { return r < 32 || r == 127 }) >= 0 {
		return false
	}
	return PPValidSort(q.Sort) && validPPCategory(q.Category) && PPValidState(q.State)
}

func validPPCategory(cat string) bool {
	return cat == "" || cat == "unchanged" || cat == "changed" ||
		cat == "ambiguous" || cat == "new" || cat == "absent"
}

func PPTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func PPCompatible(a, b *PPObservation) bool {
	if a == nil || b == nil || a.DeviceID != b.DeviceID || a.Scope != PPScope || b.Scope != PPScope {
		return false
	}
	if a.Quality.Status != "complete" || b.Quality.Status != "complete" {
		return false
	}
	return sameProvenanceContext(a.Provenance, b.Provenance)
}

func sameProvenanceContext(p, q PPProvenance) bool {
	return p.Installation == q.Installation &&
		p.Company == q.Company &&
		p.Connection == q.Connection &&
		p.CredentialRevision == q.CredentialRevision &&
		p.Family == q.Family &&
		p.Version == q.Version &&
		p.Projection != "" && p.Projection == q.Projection &&
		p.Codec != "" && p.Codec == q.Codec &&
		p.Capabilities != "" && p.Capabilities == q.Capabilities
}

func intervalAdvance(a, b, elapsed PPInterval) bool {
	if !a.Known || !b.Known || !elapsed.Known {
		return false
	}
	if a.Min < 0 || b.Min < 0 || a.Max < a.Min || b.Max < b.Min || elapsed.Min < 0 || elapsed.Max < elapsed.Min {
		return false
	}
	return b.Min > elapsed.Max && b.Min-a.Max <= elapsed.Max && b.Max-a.Min >= elapsed.Min
}

func PPPingBounds(
	target *PPPingTarget, elapsed time.Duration,
) (router, session, age PPInterval, ok bool) {
	if target == nil || target.Row.State == "OFFLINE" || elapsed < 0 {
		return
	}
	age = target.Age
	advance := elapsed.Nanoseconds()
	if !age.Known || age.Min < 0 || age.Max < age.Min ||
		age.Max > math.MaxInt64-advance-int64(QueryBudget) {
		return
	}
	age.Min += advance
	age.Max += advance + int64(QueryBudget)
	var valid bool
	router, valid = advancePingInterval(target.Observation.Provenance.RouterUptime, age)
	if !valid {
		return
	}
	session, ok = advancePingInterval(target.Row.Uptime, age)
	return
}

func advancePingInterval(old, age PPInterval) (PPInterval, bool) {
	if !old.Known || old.Min < 0 || old.Max < old.Min ||
		old.Max > math.MaxInt64-age.Max-int64(time.Second) {
		return PPInterval{}, false
	}
	return PPInterval{Known: true, Min: old.Min + age.Min, Max: old.Max + age.Max}, true
}
