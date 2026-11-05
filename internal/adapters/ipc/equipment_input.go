package ipc

import (
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

// EquipmentInput is operation-specific on the wire; presence allowlists below
// reject unrelated fields even when their values are zero.
type EquipmentInput struct {
	ID           string              `json:"id,omitempty"`
	Operation    string              `json:"operation_id,omitempty"`
	Revision     uint64              `json:"revision,omitempty"`
	Role         equipment.Role      `json:"role,omitempty"`
	Name         string              `json:"name,omitempty"`
	Address      string              `json:"address,omitempty"`
	Port         int                 `json:"port,omitempty"`
	Transport    equipment.Transport `json:"transport,omitempty"`
	CredentialID string              `json:"credential_id,omitempty"`
	Block        string              `json:"block,omitempty"`
	Active       bool                `json:"active,omitempty"`
	Inactive     bool                `json:"inactive,omitempty"`
	Search       string              `json:"search,omitempty"`
	Cursor       int                 `json:"cursor,omitempty"`
	Alias        string              `json:"alias,omitempty"`
	User         string              `json:"user,omitempty"`
	Secret       string              `json:"secret,omitempty"`
	Affected     int                 `json:"affected_links,omitempty"`
	JobID        string              `json:"job_id,omitempty"`
	Fingerprint  string              `json:"fingerprint,omitempty"`
}

func equipmentCommand(uid uint32, owner string, q Request) equipment.Command {
	e := q.Equipment
	return equipment.Command{
		Op: q.Op, UID: uid,
		Scope: access.Command{Session: q.Session, Owner: owner, Profile: q.Profile, Activation: q.Activation,
			ExpectedProfile: q.ExpectedProfile, ExpectedActivation: q.ExpectedActivation},
		ID: e.ID, Operation: e.Operation, Revision: e.Revision, Role: e.Role, Name: e.Name, Block: e.Block,
		Address: e.Address, Port: e.Port, Transport: e.Transport, CredentialID: e.CredentialID,
		Active: e.Active, Inactive: e.Inactive, Search: e.Search, Cursor: e.Cursor, Alias: e.Alias,
		User: e.User, Secret: e.Secret, Affected: e.Affected, JobID: e.JobID, Fingerprint: e.Fingerprint,
	}
}
