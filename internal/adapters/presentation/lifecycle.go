package presentation

import "sync"

// recoveryLifecycle serializes normal native close with recovery issuance.
// It does not protect against process termination, crashes or power loss.
type recoveryLifecycle struct {
	mu      sync.Mutex
	issuing int
	pending bool
	closing bool
}

func (l *recoveryLifecycle) beginEmission() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closing {
		return false
	}
	l.issuing++
	return true
}

func (l *recoveryLifecycle) endEmission() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.issuing--
}

func (l *recoveryLifecycle) setPending(pending bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.pending = pending
}

// PreventClose is wired by composition to Wails options.App.OnBeforeClose.
// A package function intentionally stays outside the App binding method set.
func PreventClose(a *App) bool {
	l := &a.lifecycle
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.issuing > 0 || l.pending {
		return true
	}
	// Once close wins, no late emission may commit a code into a closing UI.
	l.closing = true
	return false
}
