// Package composition connects concrete adapters outside the capability core.
package composition

import (
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/presentation"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/modules/access"
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
	return server, e
}
