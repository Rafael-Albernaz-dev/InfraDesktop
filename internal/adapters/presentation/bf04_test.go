package presentation

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"image"
	"image/png"
	"infradesktop/internal/adapters/ipc"
	"infradesktop/internal/adapters/security"
	"os"
	"path/filepath"
	"testing"
)

func TestBF04PreAuthCopy(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	r := app.Register("technical", "isolated BF04 password")
	if r.Code != "OK" {
		t.Fatal(r.Code)
	}
	var copied string
	app.clipboard = func(_ context.Context, value string) error { copied = value; return nil }
	if app.CopyRecovery() != "OK" || copied != r.Recovery {
		t.Fatal("registration code must be copyable before login")
	}
	if !PreventClose(app) {
		t.Fatal("copy must not acknowledge")
	}
}

func TestBF04SaveThreeFlowsAndPrivateFile(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	issued := app.Register("technical", "isolated BF04 password")
	for _, flow := range []string{"register", "renew", "recover"} {
		t.Run(flow, func(t *testing.T) {
			if flow == "renew" {
				if app.Login("technical", "isolated BF04 password", true).Code != "OK" {
					t.Fatal("login")
				}
				issued = app.RenewRecovery("isolated BF04 password", false)
			}
			if flow == "recover" {
				issued = app.Recover("technical", issued.Recovery, "isolated BF04 replacement")
			}
			if issued.Code != "OK" {
				t.Fatal(issued.Code)
			}
			path := filepath.Join(t.TempDir(), "recovery.txt")
			app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return "", nil }
			if app.SaveRecovery() != "CANCELLED" || !PreventClose(app) {
				t.Fatal("cancel acknowledged")
			}
			app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) {
				return path, errors.New("private path error")
			}
			if app.SaveRecovery() != "SAVE_UNAVAILABLE" {
				t.Fatal("picker failure")
			}
			app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) { return path, nil }
			if app.SaveRecovery() != "OK" {
				t.Fatal("save failed")
			}
			b, e := os.ReadFile(path)
			if e != nil || string(b) != issued.Recovery+"\n\nCódigo de recuperação InfraDesktop. Guarde offline em local seguro.\n" {
				t.Fatal("file differs from issued secret")
			}
			info, _ := os.Stat(path)
			if info.Mode().Perm() != 0600 {
				t.Fatal("public secret")
			}
			if app.SaveRecovery() != "SAVE_EXISTS" {
				t.Fatal("overwrite accepted")
			}
			var copied string
			app.clipboard = func(_ context.Context, s string) error { copied = s; return nil }
			if app.CopyRecovery() != "OK" || copied != issued.Recovery || !PreventClose(app) {
				t.Fatal("copy/save changed or acknowledged issuance")
			}
			if app.AckRecovery().Code != "OK" || app.CopyRecovery() != "INVALID_REQUEST" {
				t.Fatal("ack retained code")
			}
		})
	}
}
func TestBF04SaveDialogDoesNotLockIPCAndAckWins(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	app.Register("technical", "isolated BF04 password")
	chosen := make(chan struct{})
	release := make(chan struct{})
	path := filepath.Join(t.TempDir(), "code.txt")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) {
		close(chosen)
		<-release
		return path, nil
	}
	done := make(chan string, 1)
	go func() { done <- app.SaveRecovery() }()
	<-chosen
	if app.Status().Code != "OK" {
		t.Fatal("dialog held IPC mutex")
	}
	if app.SaveRecovery() != "BUSY" {
		t.Fatal("parallel picker allowed")
	}
	if app.AckRecovery().Code != "OK" {
		t.Fatal("ack")
	}
	close(release)
	if <-done != "PROFILE_CHANGED" {
		t.Fatal("late save accepted")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("late secret written")
	}
}
func TestBF04RecoveryDestinationSafety(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	os.WriteFile(target, []byte("preserve"), 0600)
	link := filepath.Join(dir, "link.txt")
	os.Symlink(target, link)
	if writeRecovery(link, "synthetic") != "SAVE_EXISTS" {
		t.Fatal("followed symlink")
	}
	alias := filepath.Join(dir, "alias")
	os.Symlink(dir, alias)
	if writeRecovery(filepath.Join(alias, "new.txt"), "synthetic") != "SAVE_UNAVAILABLE" {
		t.Fatal("followed ancestor symlink")
	}
	if writeRecovery(filepath.Join(dir, "no-extension"), "synthetic") != "SAVE_INVALID_DESTINATION" {
		t.Fatal("invalid extension")
	}
	b, _ := os.ReadFile(target)
	if string(b) != "preserve" {
		t.Fatal("overwrote target")
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".infradesktop-recovery-*"))
	if len(matches) != 0 {
		t.Fatal("temporary secret retained")
	}
}

func TestBF04PreAuthRevocationDuringPickerAndExplicitDiscard(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	first := app.Register("technical", "isolated BF04 password")
	path := filepath.Join(t.TempDir(), "old.txt")
	app.saveDialog = func(context.Context, runtime.SaveDialogOptions) (string, error) {
		// Real reset outside the presenting surface, including same-process owner.
		r := app.client.Call(context.Background(), ipc.Request{Op: "access.recover", Name: "technical", Recovery: first.Recovery, Password: "isolated BF04 replacement"})
		if r.Code != "OK" {
			t.Fatal(r.Code)
		}
		return path, nil
	}
	if app.SaveRecovery() != "UNAUTHENTICATED" {
		t.Fatal("revoked issuance saved")
	}
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("old secret written")
	}
	if !PreventClose(app) {
		t.Fatal("revocation itself acknowledged displayed code")
	}
	if app.AckRecovery().Code != "OK" || PreventClose(app) {
		t.Fatal("explicit discard after observed revocation must release native veto")
	}
}

func TestBF04MaximumBrandRealIPC(t *testing.T) {
	app := newRecoveryApp(t, security.Crypto{})
	app.Register("technical", "isolated BF04 password")
	app.AckRecovery()
	app.Login("technical", "isolated BF04 password", true)
	created := app.CreateCompany("Catalog X")
	if created.Code != "OK" {
		t.Fatal(created.Code)
	}
	id := created.Companies[0].ID
	selected := app.SelectCompany(id)
	if selected.Code != "OK" {
		t.Fatal(selected.Code)
	}
	var buf bytes.Buffer
	if e := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); e != nil {
		t.Fatal(e)
	}
	raw := append(buf.Bytes(), make([]byte, 2*1024*1024-buf.Len())...)
	logo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(raw)
	saved := app.SaveBrand(id, selected.Activation, "Visual X", logo, "rose")
	if saved.Code != "OK" || saved.Brand == nil || saved.Brand.Logo != logo || saved.Brand.Name != "Visual X" {
		t.Fatal("maximum real IPC save/response failed", saved.Code)
	}
	loaded := app.GetBrand(id, selected.Activation)
	if loaded.Code != "OK" || loaded.Brand == nil || *loaded.Brand != *saved.Brand {
		t.Fatal("maximum real IPC read failed", loaded.Code)
	}
	if status := app.Status(); status.Code != "OK" || status.Brand != nil || status.Companies[0].Name != "Catalog X" {
		t.Fatal("polling leaked brand or catalog changed")
	}
	oversized := "data:image/png;base64," + base64.StdEncoding.EncodeToString(append(raw, 0))
	if r := app.SaveBrand(id, selected.Activation, "Visual X", oversized, "rose"); r.Code != "BRAND_LOGO_INVALID" {
		t.Fatal("oversize accepted", r.Code)
	}
	other := app.CreateCompany("Catalog X")
	if other.Code != "OK" {
		t.Fatal(other.Code)
	}
	if app.SelectCompany(other.Companies[1].ID).Code != "OK" {
		t.Fatal("select Y")
	}
	if r := app.SaveBrand(id, selected.Activation, "Late X", logo, "rose"); r.Code != "PROFILE_CHANGED" {
		t.Fatal("stale X saved to Y", r.Code)
	}
}
