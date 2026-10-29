package equipment

import (
	"context"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// Measured 2048-row atomic replacement under race takes2.35s after prepared batches.
	PPPersistenceBudget          = 3 * time.Second
	PPScope                      = "pppoe-active-v1"
	PPMaxRows                    = 2048
	PPWireBytes                  = 448 * 1024
	PPObservationBytes           = 512 * 1024
	PPRowBytes                   = 2048
	PPReferenceDevice            = 8
	PPReferenceCompany           = 64
	PPCompanyBytes               = (MaxDevices + PPReferenceCompany) * PPObservationBytes
	ObservationMissing      Code = "OBSERVATION_UNAVAILABLE"
	ObservationIncompatible Code = "OBSERVATION_INCOMPATIBLE"
	CursorInvalid           Code = "CURSOR_INVALID"
)

type PPInterval struct {
	Min   int64 `json:"min"`
	Max   int64 `json:"max"`
	Known bool  `json:"known"`
}
type PPProvenance struct {
	Projection         string     `json:"projection"`
	Codec              string     `json:"codec"`
	Capabilities       string     `json:"capabilities"`
	RouterFinalUptime  PPInterval `json:"router_final_uptime"`
	Installation       string     `json:"installation"`
	Company            string     `json:"company"`
	DeviceID           string     `json:"device_id"`
	Revision           uint64     `json:"revision"`
	Connection         string     `json:"connection"`
	CredentialRevision uint64     `json:"credential_revision"`
	Family             string     `json:"family"`
	Version            string     `json:"version"`
	Epoch              string     `json:"epoch"`
	Started            string     `json:"started_at"`
	Ended              string     `json:"ended_at"`
	Sample             PPInterval `json:"sample"`
	RouterUptime       PPInterval `json:"router_uptime"`
}
type PPQuality struct {
	Status            string `json:"status"`
	Code              Code   `json:"code"`
	Reason            string `json:"reason,omitempty"`
	CountsKnown       bool   `json:"counts_known"`
	Eligible          int    `json:"eligible"`
	Returned          int    `json:"returned"`
	WireBytes         int    `json:"wire_bytes"`
	Terminated        bool   `json:"terminated"`
	MembershipChecked bool   `json:"membership_checked"`
}
type PPRow struct {
	Context     *PPContext `json:"context,omitempty"`
	State       string     `json:"state,omitempty"`
	AbsentStart string     `json:"absent_start,omitempty"`
	AbsentEnd   string     `json:"absent_end,omitempty"`
	ID          string     `json:"row_id"`
	RemoteID    string     `json:"remote_id"`
	SessionID   string     `json:"session_id"`
	Login       string     `json:"login"`
	Address     string     `json:"address"`
	Caller      string     `json:"caller_id"`
	Uptime      PPInterval `json:"uptime"`
	Sample      PPInterval `json:"sample"`
	Duplicates  int        `json:"duplicates"`
}
type PPObservation struct {
	// Some optional context was omitted by a wire, row, observation or company
	// budget. The complete base sessions remain authoritative, including absences.
	ContextOmitted bool         `json:"context_omitted,omitempty"`
	Online         int          `json:"online"`
	Offline        int          `json:"offline"`
	ID             string       `json:"observation_id"`
	DeviceID       string       `json:"device_id"`
	Scope          string       `json:"scope"`
	Provenance     PPProvenance `json:"provenance"`
	Quality        PPQuality    `json:"quality"`
	Count          int          `json:"count"`
	Bytes          int          `json:"bytes"`
	Rows           []PPRow      `json:"-"`
}
type PPAttempt struct {
	Operation         string    `json:"operation_id,omitempty"`
	Scope             string    `json:"scope,omitempty"`
	SourceObservation string    `json:"source_observation,omitempty"`
	SourceRow         string    `json:"source_row,omitempty"`
	Epoch             string    `json:"epoch"`
	JobID             string    `json:"job_id"`
	State             string    `json:"state"`
	Code              Code      `json:"code"`
	At                string    `json:"at"`
	Quality           PPQuality `json:"quality"`
	ObservationID     string    `json:"observation_id,omitempty"`
}
type PPReference struct {
	ID            string `json:"reference_id"`
	ObservationID string `json:"observation_id"`
	Created       string `json:"created_at"`
	Count         int    `json:"count"`
}
type PPQuery struct {
	Type       string `json:"type,omitempty"`
	Search     string `json:"search,omitempty"`
	Duplicates bool   `json:"duplicates_only,omitempty"`
	Sort       string `json:"sort,omitempty"`
	Desc       bool   `json:"desc,omitempty"`
	Category   string `json:"category,omitempty"`
}
type PPInput struct {
	MonitoringGeneration string   `json:"-"`
	RowIDs               []string `json:"row_ids,omitempty"`
	Scope                string   `json:"scope,omitempty"`
	DiagnosticID         string   `json:"diagnostic_id,omitempty"`
	WorkerEpoch          string   `json:"-"`
	SavedAt              string   `json:"-"`
	ExportPart           string   `json:"export_part,omitempty"`
	DeviceID             string   `json:"device_id,omitempty"`
	Revision             uint64   `json:"revision,omitempty"`
	Operation            string   `json:"operation_id,omitempty"`
	JobID                string   `json:"job_id,omitempty"`
	ObservationID        string   `json:"observation_id,omitempty"`
	RowID                string   `json:"row_id,omitempty"`
	ReferenceID          string   `json:"reference_id,omitempty"`
	BaseID               string   `json:"base_id,omitempty"`
	TargetID             string   `json:"target_id,omitempty"`
	Query                *PPQuery `json:"query,omitempty"`
	Cursor               string   `json:"cursor,omitempty"`
	ExportKey            string   `json:"export_key,omitempty"`
	Offset               int      `json:"-"`
}
type PPDeltaSession struct {
	Login   string     `json:"login"`
	Address string     `json:"address"`
	Caller  string     `json:"caller_id"`
	Uptime  PPInterval `json:"uptime"`
	State   string     `json:"state"`
}

func ppDeltaSession(r PPRow) *PPDeltaSession {
	state := r.State
	if state == "" {
		state = "ONLINE"
	}
	return &PPDeltaSession{r.Login, r.Address, r.Caller, r.Uptime, state}
}

type PPDelta struct {
	Base           *PPDeltaSession `json:"base,omitempty"`
	Target         *PPDeltaSession `json:"target,omitempty"`
	BaseRow        string          `json:"base_row,omitempty"`
	TargetRow      string          `json:"target_row,omitempty"`
	Category       string          `json:"category"`
	Reason         string          `json:"reason"`
	AddressChanged bool            `json:"address_changed"`
	CallerChanged  bool            `json:"caller_changed"`
}
type PPExportDelta struct {
	BaseRow        string `json:"base_row,omitempty"`
	TargetRow      string `json:"target_row,omitempty"`
	Category       string `json:"category"`
	Reason         string `json:"reason"`
	AddressChanged bool   `json:"address_changed"`
	CallerChanged  bool   `json:"caller_changed"`
}
type PPGroup struct {
	Peak    *int   `json:"peak_7d,omitempty"`
	Limited bool   `json:"history_limited,omitempty"`
	Service string `json:"service"`
	VLAN    *int   `json:"vlan,omitempty"`
	Count   int    `json:"count"`
}
type PPView struct {
	MonitoringGeneration string          `json:"-"`
	Read                 *PPReadResult   `json:"read,omitempty"`
	Groups               []PPGroup       `json:"groups,omitempty"`
	Diagnostic           *PDDiagnostic   `json:"diagnostic,omitempty"`
	DiagnosticRows       []PDRow         `json:"diagnostic_rows,omitempty"`
	ExportDiagnostics    []PDRow         `json:"export_diagnostics,omitempty"`
	ExportDeltas         []PPExportDelta `json:"export_deltas,omitempty"`
	Current              *PPObservation  `json:"current,omitempty"`
	Attempt              *PPAttempt      `json:"attempt,omitempty"`
	Job                  *Job            `json:"job,omitempty"`
	Rows                 []PPRow         `json:"rows,omitempty"`
	References           []PPReference   `json:"references,omitempty"`
	Reference            *PPReference    `json:"reference,omitempty"`
	Deltas               []PPDelta       `json:"deltas,omitempty"`
	Counts               map[string]int  `json:"counts,omitempty"`
	Total                int             `json:"total"`
	Next                 string          `json:"next_cursor,omitempty"`
	More                 bool            `json:"-"`
	NextOffset           int             `json:"-"`
	Reason               string          `json:"-"`
	Entity               string          `json:"-"`
	Effect               string          `json:"effect_state,omitempty"`
	ExportRows           []PPExportRow   `json:"export_rows,omitempty"`
}
type PPExportRow struct {
	State       string     `json:"state"`
	Uptime      PPInterval `json:"uptime"`
	Duplicates  int        `json:"duplicates"`
	Observation string     `json:"observation"`
	Row         string     `json:"row"`
	Login       string     `json:"login"`
	Address     string     `json:"address"`
	Caller      string     `json:"caller"`
}
type PPTarget struct {
	Installation, Company string
	Device                Device
	Credential            SealedCredential
	Wrapped               []byte
}
type PPRemote interface {
	CollectPPPoE(context.Context, Device, string, string, func() Code) (*PPObservation, Code)
}

// Store methods name product effects; no SQL or transaction types escape adapters.
type PPStore interface {
	ReadPPTarget(context.Context, string, string, bool) (PPTarget, Code)
	ReservePPCollection(context.Context, string, PPInput, string, string) (string, bool, Code)
	FinishPPCollection(context.Context, string, PPTarget, string, PPAttempt, *PPObservation) Code
	PPCurrent(context.Context, string, string) (PPView, Code)
	PPObservation(context.Context, string, string, string) (*PPObservation, Code)
	PPPage(context.Context, string, PPInput) (PPView, Code)
	PPSession(context.Context, string, PPInput) (PPView, Code)
	SavePPReference(context.Context, string, PPInput, string) (PPView, Code)
	PPReferences(context.Context, string, PPInput) (PPView, Code)
	DeletePPReference(context.Context, string, PPInput, string) (PPView, Code)
}

func PPConnection(d Device) string {
	return strings.Join([]string{d.Transport, d.Address, fmtPort(d.Port), d.Fingerprint, d.CredentialID}, "\x00")
}
func fmtPort(p int) string { return strconv.Itoa(p) }
func Endpoint(d Device) string {
	ip, e := netip.ParseAddr(d.Address)
	if e != nil {
		return ""
	}
	return d.Transport + "/" + ip.Unmap().String() + "/" + fmtPort(d.Port)
}
func PPValidQuery(q *PPQuery) bool {
	if q == nil {
		return true
	}
	return utf8.ValidString(q.Type) && len(q.Type) <= 64 && strings.IndexFunc(q.Type, func(r rune) bool { return r < 32 || r == 127 }) < 0 && utf8.ValidString(q.Search) && len(q.Search) <= 160 && (q.Sort == "" || q.Sort == "login" || q.Sort == "address" || q.Sort == "uptime" || q.Sort == "row") && (q.Category == "" || q.Category == "unchanged" || q.Category == "changed" || q.Category == "ambiguous" || q.Category == "new" || q.Category == "absent")
}
func PPTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
