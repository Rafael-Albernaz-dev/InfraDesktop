package equipment

import (
	"context"
	"sync"
	"sync/atomic"
)

type avulsoStore struct {
	radioLiveStore
	mu    sync.Mutex
	state State
}

func (s *avulsoStore) ReadEquipment(context.Context, string) (State, Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := s.state
	copy.Devices = append([]Device(nil), s.state.Devices...)
	copy.Operations = append([]Operation(nil), s.state.Operations...)
	return copy, OK
}
func (s *avulsoStore) UpdateEquipment(_ context.Context, _ string, f func(*State) Code) Code {
	s.mu.Lock()
	defer s.mu.Unlock()
	return f(&s.state)
}
func (s *avulsoStore) ReadPPTarget(_ context.Context, _ string, id string, _ bool) (PPTarget, Code) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := device(&s.state, id)
	if d == nil {
		return PPTarget{}, Invalid
	}
	if d.CredentialID == "" {
		return PPTarget{Device: *d}, MissingCredential
	}
	return PPTarget{Device: *d, Credential: SealedCredential{Metadata: Credential{ID: d.CredentialID, Revision: 1}}}, OK
}

type avulsoRemote struct {
	*radioLiveRemote
	inspections atomic.Int32
}

func (r *avulsoRemote) Inspect(_ context.Context, d Device) (string, Code) {
	r.inspections.Add(1)
	return "SHA256:synthetic", OK
}
