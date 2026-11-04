package inventory

import (
	"context"
	"time"
)

// RecordAttempt keeps a finished connection outcome durable for the automatic
// refresh gate. A credential swapped during the job never inherits the outcome.
func (s *Service) RecordAttempt(ctx context.Context, profile string, device Device, jobID string, code Code) {
	persistCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	at := s.Now().UTC().Format(time.RFC3339Nano)
	_ = s.Store.UpdateEquipment(persistCtx, profile, func(state *State) Code {
		target := DeviceByID(state, device.ID)
		if target != nil && target.CredentialID == device.CredentialID {
			target.LastAttempt = &Attempt{JobID: jobID, State: jobStateForCode(code), Code: code, At: at}
		}
		return OK
	})
}
