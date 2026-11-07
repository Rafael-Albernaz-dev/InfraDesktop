// Package pulse owns credential-free reachability observations in memory.
package pulse

import (
	"context"
	"time"
)

type Target struct{ ID, Address string }
type Outcome string

const (
	Reply            Outcome = "reply"
	NoReply          Outcome = "no_reply"
	ProbeUnavailable Outcome = "probe_unavailable"
)

type State string

const (
	Unknown State = ""
	Online  State = "ONLINE"
	Offline State = "OFFLINE"
)

type Network string

const (
	NetworkOK    Network = "ok"
	NoResponse   Network = "no_response"
	ProbeBlocked Network = "probe_blocked"
)

type DeviceStatus struct {
	Address   string    `json:"-"`
	State     State     `json:"state"`
	Since     time.Time `json:"since"`
	LastReply time.Time `json:"last_reply"`
	CheckedAt time.Time `json:"checked_at"`
}
type Snapshot struct {
	Devices map[string]DeviceStatus
	Network Network
}
type Inventory interface {
	Targets(context.Context) (string, []Target, bool)
}
type Prober interface {
	Probe(context.Context, string) Outcome
}
