// Package presentation exposes narrow use-case bindings, never generic IO or tokens.
package presentation

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"infradesktop/internal/adapters/ipc"
	"strings"
	"sync"
)

type Result struct {
	Code           string        `json:"code"`
	RequestID      string        `json:"request_id"`
	Instance       string        `json:"instance_id"`
	Activation     string        `json:"activation_id"`
	Profile        string        `json:"profile_id"`
	Heartbeat      int64         `json:"heartbeat"`
	Setup          bool          `json:"setup"`
	Authenticated  bool          `json:"authenticated"`
	Name           string        `json:"name"`
	Theme          string        `json:"theme"`
	Recovery       string        `json:"recovery"`
	Companies      []ipc.Company `json:"companies"`
	Brand          *ipc.Branding `json:"brand,omitempty"`
	ProductVersion string        `json:"product_version"`
}
type App struct {
	ppExportFS                                                      ppFileOps
	ppGeneration                                                    uint64
	ppExporting                                                     bool
	client                                                          *ipc.AccountClient
	mu                                                              sync.Mutex
	ctx                                                             context.Context
	lifecycle                                                       recoveryLifecycle
	accessRecoveryPending                                           bool
	renewalRecoveryPending                                          bool
	recovery, recoveryInstance, recoveryProfile, recoveryActivation string
	clipboard                                                       func(context.Context, string) error
	saveDialog                                                      func(context.Context, runtime.SaveDialogOptions) (string, error)
	saving                                                          bool
	recoveryGeneration                                              uint64
}

func New(c *ipc.AccountClient) *App {
	return &App{client: c, ctx: context.Background(), clipboard: runtime.ClipboardSetText, saveDialog: runtime.SaveFileDialog}
}
func AttachContext(a *App, ctx context.Context) { a.mu.Lock(); a.ctx = ctx; a.mu.Unlock() }
func (a *App) call(q ipc.Request) Result {
	if q.Op == "access.register" || q.Op == "access.recover" || q.Op == "access.renew-recovery" {
		if !a.lifecycle.beginEmission() {
			return Result{Code: "BUSY", Companies: []ipc.Company{}}
		}
		defer a.lifecycle.endEmission()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	defer a.syncRecoveryPending()
	switch q.Op {
	case "access.register", "access.login", "access.logout", "company.select", "company.create", "access.recover", "access.renew-recovery", "access.ack-recovery", "access.ack-issued":
		a.ppGeneration++
	}
	r := a.client.Call(a.ctx, q)
	if r.Code == "UNAUTHENTICATED" || r.Code == "PROFILE_CHANGED" {
		a.ppGeneration++
	}
	if !a.accessRecoveryPending && (r.Code == "UNAUTHENTICATED" || r.Code == "PROFILE_CHANGED" || (r.Data != nil && a.recovery != "" && (r.Data.Instance != a.recoveryInstance || !r.Data.Authenticated || r.Data.Profile != a.recoveryProfile || r.Data.Activation != a.recoveryActivation))) {
		a.recovery = ""
		a.recoveryGeneration++
	}
	if (q.Op == "access.renew-recovery" || q.Op == "access.register" || q.Op == "access.recover") && r.Code == "OK" && r.Data != nil {
		a.recovery = r.Data.Recovery
		a.renewalRecoveryPending = q.Op == "access.renew-recovery" && r.Data.Recovery != ""
		a.recoveryGeneration++
		a.recoveryInstance = r.Data.Instance
		a.recoveryProfile = r.Data.Profile
		a.recoveryActivation = r.Data.Activation
	}
	if q.Op == "access.ack-recovery" && r.Code == "OK" {
		a.recovery = ""
		a.renewalRecoveryPending = false
		a.recoveryGeneration++
	}
	if (q.Op == "access.register" || q.Op == "access.recover") && r.Code == "OK" && r.Data != nil {
		a.accessRecoveryPending = r.Data.Recovery != ""
		a.renewalRecoveryPending = false
	}
	if q.Op == "access.bootstrap" && (r.Code == "UNAUTHENTICATED" || (r.Code == "OK" && r.Data != nil && !r.Data.Authenticated)) {
		a.renewalRecoveryPending = false
	}
	v := Result{Code: string(r.Code), RequestID: r.RequestID, Companies: []ipc.Company{}}
	if d := r.Data; d != nil {
		v.Instance = d.Instance
		v.Activation = d.Activation
		v.Profile = d.Profile
		v.Heartbeat = d.Heartbeat
		v.Setup = d.Setup
		v.Authenticated = d.Authenticated
		v.Name = d.Name
		v.Theme = d.Theme
		v.Recovery = d.Recovery
		v.Companies = d.Companies
		v.Brand = d.Brand
		v.ProductVersion = d.ProductVersion
	}
	return v
}
func (a *App) Status() Result { return a.call(ipc.Request{Op: "access.bootstrap"}) }
func (a *App) Register(name, password string) Result {
	return a.call(ipc.Request{Op: "access.register", Name: name, Password: password})
}
func (a *App) Login(name, password string, remember bool) Result {
	return a.call(ipc.Request{Op: "access.login", Name: name, Password: password, Remember: remember})
}
func (a *App) Recover(name, code, password string) Result {
	return a.call(ipc.Request{Op: "access.recover", Name: name, Recovery: strings.ReplaceAll(code, "-", ""), Password: password})
}
func (a *App) Logout() Result { return a.call(ipc.Request{Op: "access.logout"}) }
func (a *App) CreateCompany(name string) Result {
	return a.call(ipc.Request{Op: "company.create", Name: name})
}
func (a *App) SelectCompany(id string) Result {
	return a.call(ipc.Request{Op: "company.select", Name: id})
}
func (a *App) SetTheme(theme string) Result {
	return a.call(ipc.Request{Op: "preferences.theme", Theme: theme})
}

func (a *App) RenewRecovery(password string, replacePending bool) Result {
	return a.call(ipc.Request{Op: "access.renew-recovery", Password: password, Replace: replacePending})
}
func (a *App) AckRecovery() Result {
	a.mu.Lock()
	if a.accessRecoveryPending {
		r := a.client.Call(a.ctx, ipc.Request{Op: "access.ack-issued", Recovery: a.recovery})
		if r.Code != "OK" && r.Code != "UNAUTHENTICATED" && r.Code != "PROFILE_CHANGED" {
			a.mu.Unlock()
			return Result{Code: string(r.Code), Companies: []ipc.Company{}}
		}
		a.accessRecoveryPending = false
		a.recovery = ""
		a.recoveryGeneration++
		a.syncRecoveryPending()
		a.mu.Unlock()
		return Result{Code: "OK", Companies: []ipc.Company{}}
	}
	a.mu.Unlock()
	return a.call(ipc.Request{Op: "access.ack-recovery"})
}

// Caller holds mu; the native callback never waits on the IPC/client mutex.
func (a *App) syncRecoveryPending() {
	// Clipboard cache invalidation (including a transport restart) alone is
	// not an acknowledgement of the code still displayed by the UI.
	a.lifecycle.setPending(a.accessRecoveryPending || a.renewalRecoveryPending)
}

// CopyRecovery cannot accept arbitrary clipboard text or read clipboard contents.
func (a *App) CopyRecovery() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if code := a.checkRecovery(); code != "OK" {
		return code
	}
	if e := a.clipboard(a.ctx, a.recovery); e != nil {
		return "CLIPBOARD_UNAVAILABLE"
	}
	return "OK"
}

// Caller holds mu. Pre-auth receipt grants no authenticated account access.
func (a *App) checkRecovery() string {
	if a.recovery == "" {
		return "INVALID_REQUEST"
	}
	op := "access.snapshot"
	if a.accessRecoveryPending {
		op = "access.check-issued"
	}
	q := ipc.Request{Op: op}
	if a.accessRecoveryPending {
		q.Recovery = a.recovery
	}
	r := a.client.Call(a.ctx, q)
	if r.Code != "OK" {
		if r.Code == "UNAUTHENTICATED" || r.Code == "PROFILE_CHANGED" {
			a.recovery = ""
			a.recoveryGeneration++
		}
		return string(r.Code)
	}
	d := r.Data
	if d == nil || d.Instance != a.recoveryInstance || (!a.accessRecoveryPending && (d.Profile != a.recoveryProfile || d.Activation != a.recoveryActivation)) {
		a.recovery = ""
		a.recoveryGeneration++
		return "PROFILE_CHANGED"
	}
	return "OK"
}
func (a *App) GetBrand(profile, activation string) Result {
	return a.call(ipc.Request{Op: "company.brand-get", ExpectedProfile: profile, ExpectedActivation: activation})
}
func (a *App) SaveBrand(profile, activation, name, logo, accent string) Result {
	return a.call(ipc.Request{Op: "company.brand-set", ExpectedProfile: profile, ExpectedActivation: activation, Brand: &ipc.Branding{Name: name, Logo: logo, Accent: accent}})
}
