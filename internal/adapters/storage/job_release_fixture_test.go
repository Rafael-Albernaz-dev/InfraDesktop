package storage

import (
	"time"

	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/jobprobe"
)

func init() {
	equipment.DefaultFactory = func(a equipment.Authorizer, s equipment.Store, v equipment.Vault,
		r equipment.Remote, ids equipment.IDs, now func() time.Time) equipment.Handler {
		return jobprobe.New(a, s, v, r, ids, now).Handler
	}
}

func fixtureJobReleased(service *equipment.Service, id string) bool {
	return jobprobe.Released(service, id)
}
