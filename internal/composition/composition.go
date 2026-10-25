// Package composition connects concrete adapters outside the capability core.
package composition

import (
	"github.com/google/uuid"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/presentation"
	"infradesktop/internal/adapters/routerssh"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"time"
)

func RuntimePath() string {
	if path := os.Getenv("INFRADESKTOP_RUNTIME"); path != "" {
		return path
	}
	u, e := user.Current()
	if e != nil || !regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`).MatchString(u.Username) {
		return "/invalid-infradesktop-runtime"
	}
	return filepath.Join("/run", "infradesktop-"+u.Username)
}
func Desktop() *presentation.App {
	return presentation.New(ipc.NewAccountClient(RuntimePath(), ClientStatePath(), uint32(os.Getuid())))
}
func StatePath() string {
	if p := os.Getenv("INFRADESKTOP_STATE"); p != "" {
		return p
	}
	u, e := user.Current()
	if e != nil || !regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`).MatchString(u.Username) {
		return "/invalid-infradesktop-state"
	}
	return filepath.Join("/var/lib", "infradesktop-"+u.Username)
}
func ClientStatePath() string {
	if p := os.Getenv("INFRADESKTOP_CLIENT_STATE"); p != "" {
		return p
	}
	home, e := os.UserHomeDir()
	if e != nil {
		return "/invalid-client-state"
	}
	return filepath.Join(home, ".local/state/infradesktop")
}

// devVaultEnabled is true only in the separate, uninstalled test daemon.
var devVaultEnabled = "false"

type equipmentIDs struct{}

func (equipmentIDs) ID() (string, equipment.Code) {
	id, e := uuid.NewRandom()
	if e != nil {
		return "", equipment.Unavailable
	}
	return id.String(), equipment.OK
}
func Daemon(dir string) (*ipc.Server, error) {
	instance, e := ipc.RandomID()
	if e != nil {
		return nil, e
	}
	activation, e := ipc.RandomID()
	if e != nil {
		return nil, e
	}
	uid := uint32(os.Getuid())
	store, e := storage.Open(StatePath())
	if e != nil {
		return nil, e
	}
	service := access.New(store, security.Crypto{}, time.Now, uid, activation, store)
	server, e := ipc.ListenAccess(dir, uid, service, foundation.Status{Instance: instance, Activation: activation}, store.Close)
	if e != nil {
		store.Close()
	}
	if e == nil {
		provider := vault.Provider(vault.Systemd)
		if path := os.Getenv("INFRADESKTOP_DEV_VAULT_KEY"); path != "" && devVaultEnabled == "true" {
			provider = func() ([]byte, error) { return vault.ReadKey(path) }
		}
		queries := equipment.New(service, store, vault.New(provider), routerssh.Adapter{}, equipmentIDs{}, time.Now)
		ipc.AttachEquipment(server, queries)
	}
	return server, e
}
