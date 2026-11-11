package inventory

import (
	"context"
	"infradesktop/internal/modules/foundation"
	"time"
)

// RecordAttempt keeps a finished connection outcome durable for the automatic
// refresh gate. A credential swapped during the job never inherits the outcome.
func (s *Service) RecordAttempt(ctx context.Context, profile string, device Device, jobID string, code Code) {
	at := s.Now().UTC().Format(time.RFC3339Nano)
	// Each write try has 2 s (stretched only under -race); a slow disk retries it.
	// ignored: the attempt is advisory; a lost write only delays the refresh gate.
	_ = s.PersistWithRetry(2*time.Second*foundation.RaceScale, func(persistCtx context.Context) Code {
		return s.Store.UpdateEquipment(persistCtx, profile, func(state *State) Code {
			target := DeviceByID(state, device.ID)
			if target != nil && target.CredentialID == device.CredentialID {
				target.LastAttempt = &Attempt{JobID: jobID, State: jobStateForCode(code), Code: code, At: at}
			}
			return OK
		})
	})
}
