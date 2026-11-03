package composition

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/monitoring"
	"infradesktop/internal/modules/pppoe"
	"infradesktop/internal/modules/radio"
	"os"
	"time"
)

func mountCapabilities(server *ipc.Server, service *access.Service, store *storage.Store) {
	provider := vault.Provider(vault.Systemd)
	if path := os.Getenv("INFRADESKTOP_DEV_VAULT_KEY"); path != "" && devVaultEnabled == "true" {
		provider = func() ([]byte, error) { return vault.ReadKey(path) }
	}
	mon := monitoring.New(service, store)
	inv := inventory.New(service, store, vault.New(provider), routerssh.Adapter{}, equipmentIDs{}, time.Now)
	pp := pppoe.New(inv, mon)
	rad := radio.New(inv)
	ipc.AttachEquipment(server, &equipment.Service{Handler: radio.NewComposite(inv, pp, rad, mon)})
}
