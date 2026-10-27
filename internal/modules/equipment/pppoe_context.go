package equipment

import "strings"

// PPContext contains stable, optional facts from a uniquely associated tunnel.
// Nil fields express unavailable; a successful empty PD lookup is distinct.
type PPContext struct {
	Association     string `json:"association"`
	Service         string `json:"service,omitempty"`
	Interface       string `json:"interface,omitempty"`
	VLAN            *int   `json:"vlan,omitempty"`
	IPv6            *bool  `json:"ipv6,omitempty"`
	DelegatedPrefix string `json:"delegated_prefix,omitempty"`
	PDStatus        string `json:"pd_status,omitempty"`
	Profile         string `json:"profile,omitempty"`
	SecretState     string `json:"secret_state,omitempty"`
	UploadLimit     string `json:"upload_limit,omitempty"`
	DownloadLimit   string `json:"download_limit,omitempty"`
}

// PPTunnel is a transient remote reading. Identity and counters never persist.
type PPTunnel struct {
	ID, Name, Login, Caller, Address string
	Uptime                           PPInterval
	Context                          PPContext
	RouterCPU                        *uint64
	RX, TX                           *uint64
	Sample, RouterUptime             PPInterval
}

func PPTunnelMatches(row PPRow, t PPTunnel, age PPInterval) bool {
	if row.State == "OFFLINE" || row.Login == "" || row.Caller == "" || row.Address == "" || row.Login != t.Login || !strings.EqualFold(row.Caller, t.Caller) || row.Address != t.Address {
		return false
	}
	return intervalAdvance(row.Uptime, t.Uptime, age)
}

// Matching is a one-to-one relation in both directions, not a name lookup.
func AssociatePPTunnels(rows []PPRow, tunnels []PPTunnel, age PPInterval) []int {
	matches := make([]int, len(rows))
	uses := make([]int, len(tunnels))
	for i, row := range rows {
		matches[i] = -1
		count := 0
		for n, tunnel := range tunnels {
			if PPTunnelMatches(row, tunnel, age) {
				count++
				matches[i] = n
				uses[n]++
			}
		}
		if count != 1 {
			matches[i] = -1
		}
	}
	for i, n := range matches {
		rows[i].Context = nil
		if n < 0 || uses[n] != 1 {
			matches[i] = -1
			continue
		}
		value := tunnels[n].Context
		value.Association = "associated"
		addresses := 0
		for _, row := range rows {
			if row.Address == rows[i].Address {
				addresses++
			}
		}
		if addresses != 1 {
			value.UploadLimit = ""
			value.DownloadLimit = ""
		}
		rows[i].Context = &value
	}
	return matches
}
