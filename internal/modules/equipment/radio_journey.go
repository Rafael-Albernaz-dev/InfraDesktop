package equipment

import (
	"context"
	"sort"
	"strconv"
	"time"
)

const RadioIndexTTL = 5 * time.Minute

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
type RadioIndexRemote interface {
	ReadRadioIndex(context.Context, Device, string, string, func() Code) ([]RadioIndexEntry, Code)
}

func radioIndexKey(q Command) string { return q.Scope.Profile + "|" + q.Scope.Activation }
func radioIndexContext(st State) string {
	out := ""
	for _, d := range st.Devices {
		if d.Active && d.Role == RoleAP {
			out += d.ID + "|" + PPConnection(d) + "|" + strconv.FormatUint(d.Revision, 10)
			if c := credential(&st, d.CredentialID); c != nil {
				out += "|" + strconv.FormatUint(c.Metadata.Revision, 10)
			}
		}
	}
	return out
}

// Bound public metadata as well as hidden registration entries. A large inventory
// must still expose refused APs without exceeding the IPC frame budget.
func pageRadioIndex(v RadioView, offset int) (RadioView, Code) {
	if v.Index == nil {
		return v, OK
	}
	keys := make([]string, 0, len(v.Index.Targets))
	for id := range v.Index.Targets {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	if offset < 0 || offset > len(keys) {
		return RadioView{}, CursorInvalid
	}
	index := *v.Index
	index.Targets = map[string]Code{}
	index.Labels = map[string]string{}
	end := min(offset+RadioPageSize, len(keys))
	index.Total, index.Offset, index.Next, index.More = len(keys), offset, end, end < len(keys)
	for _, id := range keys[offset:end] {
		index.Targets[id] = v.Index.Targets[id]
		index.Labels[id] = v.Index.Labels[id]
	}
	v.Index = &index
	return v, OK
}
