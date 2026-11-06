package composition

import (
	"time"

	"infradesktop/internal/adapters/importfile"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/inventory"
	"infradesktop/internal/modules/monitoring"
	"infradesktop/internal/modules/pppoe"
	"infradesktop/internal/modules/radio"
)

func composeEquipment(service *access.Service, store *storage.Store, provider vault.Provider) *equipment.Service {
	mon := monitoring.New(service, store)
	inv := inventory.New(service, store, vault.New(provider), routerssh.Adapter{}, equipmentIDs{}, time.Now)
	inv.Parser, inv.Digester = importfile.Parser{}, importfile.Parser{}
	return &equipment.Service{Handler: radio.NewComposite(inv, pppoe.New(inv, mon), radio.New(inv), mon)}
}
