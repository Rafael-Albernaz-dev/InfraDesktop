package presentation

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/security"
	"infradesktop/internal/adapters/storage"
	"infradesktop/internal/adapters/vault"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type modalIDs struct{}

func (modalIDs) ID() (string, equipment.Code) { return uuid.NewString(), equipment.OK }

// No Remote is provided: saving/validation must never attempt SSH.
func TestValidationModalBindingRealDaemon(t *testing.T) {
	root := t.TempDir()
	t.Logf("synthetic root: %s; cleanup: t.TempDir after server cancellation", root)
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	for _, n := range []string{"runtime", "state", "client"} {
		if e := os.Mkdir(filepath.Join(root, n), 0700); e != nil {
			t.Fatal(e)
		}
	}
	st, e := storage.Open(filepath.Join(root, "state"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	uid := uint32(os.Getuid())
	auth := access.New(st, security.Crypto{}, time.Now, uid, "modal", st)
	key := filepath.Join(root, "key")
	if e = os.WriteFile(key, bytes.Repeat([]byte{29}, 32), 0600); e != nil {
		t.Fatal(e)
	}
	provider := vault.New(func() ([]byte, error) { return vault.ReadKey(key) })
	svc := equipment.New(auth, st, provider, nil, modalIDs{}, time.Now)
	defer svc.Close()
	server, e := ipc.ListenAccess(filepath.Join(root, "runtime"), uid, auth, foundation.Status{Instance: "modal"}, nil)
	if e != nil {
		t.Fatal(e)
	}
	ipc.AttachEquipment(server, svc)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	defer func() {
		cancel()
		if e := <-done; e != nil {
			t.Error(e)
		}
	}()
	app := New(ipc.NewAccountClient(filepath.Join(root, "runtime"), filepath.Join(root, "client"), uid))
	const pw = "isolated modal account password"
	if app.Register("technical", pw).Code != "OK" || app.AckRecovery().Code != "OK" || app.Login("technical", pw, false).Code != "OK" {
		t.Fatal("account setup")
	}
	c := app.CreateCompany("Modal synthetic")
	if c.Code != "OK" {
		t.Fatal(c.Code)
	}
	selected := app.SelectCompany(c.Companies[0].ID)
	if selected.Code != "OK" {
		t.Fatal(selected.Code)
	}
	profile, activation := selected.Profile, selected.Activation
	base := DeviceInput{ID: uuid.NewString(), Operation: uuid.NewString(), Name: "Router", Address: "10.0.0.1", Port: 22, Transport: "ssh", User: "reader", Secret: "  synthetic password  "}
	for _, tc := range []struct {
		name string
		edit func(*DeviceInput)
		want string
	}{
		{"dotless", func(d *DeviceInput) { d.Address = "10001" }, "DEVICE_ADDRESS_INVALID"},
		{"host-port", func(d *DeviceInput) { d.Address = "10.0.0.1:8291" }, "DEVICE_ADDRESS_INVALID"},
		{"url", func(d *DeviceInput) { d.Address = "ssh://10.0.0.1" }, "DEVICE_ADDRESS_INVALID"},
		{"user-at", func(d *DeviceInput) { d.Address = "reader@10.0.0.1" }, "DEVICE_ADDRESS_INVALID"},
		{"unspecified", func(d *DeviceInput) { d.Address = "::" }, "DEVICE_ADDRESS_INVALID"},
		{"multicast", func(d *DeviceInput) { d.Address = "ff02::1" }, "DEVICE_ADDRESS_INVALID"},
		{"name", func(d *DeviceInput) { d.Name = "bad\nname" }, "DEVICE_NAME_INVALID"},
		{"user", func(d *DeviceInput) { d.User = "bad\nuser" }, "CREDENTIAL_USER_INVALID"},
		{"port", func(d *DeviceInput) { d.Port = 65536 }, "DEVICE_PORT_INVALID"},
		{"secret", func(d *DeviceInput) { d.Secret = "" }, "CREDENTIAL_SECRET_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			tc.edit(&d)
			r := app.SaveEquipment(profile, activation, d)
			if r.Code != tc.want {
				t.Fatalf("got %s want %s", r.Code, tc.want)
			}
			state, c := st.ReadEquipment(ctx, profile)
			if c != equipment.OK || len(state.Devices)+len(state.Credentials)+len(state.Operations) != 0 || len(state.WrappedKey) != 0 {
				t.Fatal("validation left partial data")
			}
		})
	}
	for _, address := range []string{"10.0.0.1", " 2001:0DB8:0:0:0:0:0:1 ", "::ffff:192.0.2.1"} {
		d := base
		d.ID = uuid.NewString()
		d.Operation = uuid.NewString()
		d.Address = address
		d.Name = " Router "
		d.User = " reader "
		r := app.SaveEquipment(profile, activation, d)
		if r.Code != "OK" {
			t.Fatalf("valid address rejected: %s", r.Code)
		}
		if app.SaveEquipment(profile, activation, d).Code != "OK" {
			t.Fatal("retry failed")
		}
	}
	state, cod := st.ReadEquipment(ctx, profile)
	if cod != equipment.OK || len(state.Devices) != 3 || len(state.Credentials) != 3 || len(state.Operations) != 3 {
		t.Fatal("atomic/idempotent enrollment failed")
	}
	if state.Devices[1].Address != "2001:db8::1" || state.Devices[0].Name != "Router" || state.Credentials[0].Metadata.User != "reader" {
		t.Fatal("identifier canonicalization failed")
	}
	for _, c := range state.Credentials {
		plain, cod := provider.Open(state.Installation, state.Company, c.Metadata.ID, c.Metadata.Revision, c.Envelope, state.WrappedKey)
		if cod != equipment.OK || plain != base.Secret {
			t.Fatal("password changed or envelope unreadable")
		}
	}
}
