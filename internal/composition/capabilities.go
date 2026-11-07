package composition

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"os"
)

func mountCapabilities(server *ipc.Server, service *access.Service, store *storage.Store) {
	provider := vault.Provider(vault.Systemd)
	if path := os.Getenv("INFRADESKTOP_DEV_VAULT_KEY"); path != "" && devVaultEnabled == "true" {
		provider = func() ([]byte, error) { return vault.ReadKey(path) }
	}
	ipc.AttachEquipment(server, composeEquipment(service, store, provider, composePulse(server, service, store)))
}
