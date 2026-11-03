package equipment

import (
	"context"
	"strings"
	"time"
)

// Inspection sends no password; the pin transaction precedes credential access.
func (s *Service) pinRadioTOFU(ctx context.Context, q Command, d Device) (Device, Code) {
	if d.Fingerprint != "" {
		return d, OK
	}
	if d.Role != RoleAP && d.Role != RoleCPE && d.Role != RolePTP {
		return d, TrustRequired
	}
	guard := func() Code { return s.guardRadioTOFUTarget(ctx, q, d) }
	if code := guard(); code != OK {
		return d, code
	}
	fingerprint, code := s.inspectRadioTOFU(ctx, d, guard)
	if code != OK {
		return d, code
	}
	return s.recordRadioTOFU(ctx, q, d, fingerprint)
}

func sameRadioTarget(current *Device, expected Device) bool {
	return current != nil && current.Active && current.Revision == expected.Revision &&
		PPConnection(*current) == PPConnection(expected)
}

func (s *Service) guardRadioTOFUTarget(ctx context.Context, q Command, d Device) Code {
	return s.authorized(ctx, q, func(uint64) Code {
		state, code := s.store.ReadEquipment(ctx, q.Scope.Profile)
		if code != OK {
			return code
		}
		if !sameRadioTarget(device(&state, d.ID), d) {
			return Revision
		}
		return OK
	})
}

func (s *Service) inspectRadioTOFU(ctx context.Context, d Device, guard func() Code) (string, Code) {
	inspectCtx, cancel := context.WithTimeout(ctx, QueryBudget)
	done, exited := make(chan struct{}), make(chan struct{})
	authFailure := make(chan Code, 1)
	go watchRadioTOFU(inspectCtx, guard, cancel, done, exited, authFailure)
	fingerprint, code := s.remote.Inspect(inspectCtx, d)
	close(done)
	<-exited
	cancel()
	select {
	case reason := <-authFailure:
		return "", reason
	default:
	}
	return fingerprint, code
}

func watchRadioTOFU(ctx context.Context, guard func() Code, cancel context.CancelFunc,
	done, exited chan struct{}, authFailure chan Code) {
	defer close(exited)
	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if code := guard(); code != OK {
				authFailure <- code
				cancel()
				return
			}
		}
	}
}

func (s *Service) recordRadioTOFU(ctx context.Context, q Command, d Device, fingerprint string) (Device, Code) {
	code := s.authorized(ctx, q, func(uint64) Code {
		return s.store.UpdateEquipment(ctx, q.Scope.Profile, func(state *State) Code {
			current := device(state, d.ID)
			if !sameRadioTarget(current, d) {
				return Revision
			}
			if current.Fingerprint != "" {
				return TrustChanged
			}
			if !strings.HasPrefix(fingerprint, "SHA256:") {
				return Invalid
			}
			if len(state.Operations) >= MaxOperations {
				return Limit
			}
			operation, code := s.ids.ID()
			if code != OK {
				return code
			}
			current.Fingerprint = fingerprint
			current.TrustOrigin = "automatic-tofu"
			current.TrustAcceptance = &TrustAcceptance{Mode: "automatic-tofu", At: PPTime(s.now()), UID: q.UID,
				Company: q.Scope.Profile, DeviceID: d.ID, Address: d.Address, Port: d.Port,
				Revision: d.Revision, Fingerprint: fingerprint}
			current.Revision++
			state.Operations = append(state.Operations, Operation{operation, "automatic-tofu:" + fingerprint, d.ID})
			d = *current
			return OK
		})
	})
	return d, code
}
