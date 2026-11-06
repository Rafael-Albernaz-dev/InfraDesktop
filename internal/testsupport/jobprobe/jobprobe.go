// Package jobprobe exposes job release only to synthetic test fixtures.
package jobprobe

import (
	"time"

	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/monitoring"
	"infradesktop/internal/modules/pppoe"
	"infradesktop/internal/modules/radio"
)

type tracked struct {
	equipment.Handler
	inv *inventory.Service
}

func New(auth equipment.Authorizer, store equipment.Store, vault equipment.Vault, remote equipment.Remote,
	ids equipment.IDs, now func() time.Time) *equipment.Service {
	inv := inventory.New(auth, store, vault, remote, ids, now)
	mon := monitoring.New(auth, store)
	handler := radio.NewComposite(inv, pppoe.New(inv, mon), radio.New(inv), mon)
	return &equipment.Service{Handler: &tracked{Handler: handler, inv: inv}}
}

func Released(service *equipment.Service, id string) bool {
	fixture, ok := service.Handler.(*tracked)
	if !ok {
		return false
	}
	fixture.inv.Mu.Lock()
	defer fixture.inv.Mu.Unlock()
	job := fixture.inv.Jobs[id]
	return job != nil && job.Released
}

func WaitIdle(t interface {
	Helper()
	Fatal(...any)
}, service *equipment.Service) {
	t.Helper()
	fixture, ok := service.Handler.(*tracked)
	if !ok {
		t.Fatal("job fixture unavailable")
		return
	}
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		fixture.inv.Mu.Lock()
		idle := true
		for _, job := range fixture.inv.Jobs {
			if !job.Released {
				idle = false
				break
			}
		}
		fixture.inv.Mu.Unlock()
		if idle {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("jobs not released")
}
