package composition

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

func composeLiveEquipment(server *ipc.Server, service *access.Service, store *storage.Store, provider vault.Provider) *equipment.Service {
	ping := composePulse(server, service, store)
	reader := composeLiveReader(server, liveInventory{service, store}, vault.New(provider), ping)
	return composeEquipment(service, store, provider, ping, reader)
}
