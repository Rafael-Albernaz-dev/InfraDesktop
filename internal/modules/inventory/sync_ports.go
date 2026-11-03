// Package inventory owns equipment, sectors, credentials, trust pins, and finite device queries.
package inventory

import "context"

// Sector identifies an operational sector with an opaque UUID and revision (ARQ-002 R5).
type Sector struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Revision uint64 `json:"revision"`
	Active   bool   `json:"active"`
}

// InventorySync is the optional domain port for synchronizing device and sector metadata (ARQ-002 R1/R5).
// Credentials and secrets never cross this port (ARQ-002 R3).
type InventorySync interface {
	SyncDevice(ctx context.Context, company string, device Device) Code
	SyncSector(ctx context.Context, company string, sector Sector) Code
}

// ObservationSink is the optional domain port for publishing sanitized device observations (ARQ-002 R1/R6).
type ObservationSink interface {
	PublishObservation(ctx context.Context, company, deviceID string, observation Observation) Code
}

// NullInventorySync is the default local-first no-op adapter when no remote sync is configured.
type NullInventorySync struct{}

func (NullInventorySync) SyncDevice(context.Context, string, Device) Code { return OK }
func (NullInventorySync) SyncSector(context.Context, string, Sector) Code { return OK }

// NullObservationSink is the default local-first no-op adapter when no remote sink is configured.
type NullObservationSink struct{}

func (NullObservationSink) PublishObservation(context.Context, string, string, Observation) Code {
	return OK
}
