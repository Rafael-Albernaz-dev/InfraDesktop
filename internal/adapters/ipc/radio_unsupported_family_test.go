package ipc

import "testing"

// Real IPC/service/SQLite/adapter path, including reopening through persisted reads.
func TestRadioObservedUnsupportedThroughIPC(t *testing.T) {
	f := newUnsupportedIPCScenario(t)
	input := prepareUnsupportedRadioLink(t, f)
	assertUnknownCPERequiresIdentity(t, f, input)
	cpeBefore := identifyUnsupportedCPE(t, f)
	input = measureUnsupportedAPOnly(t, f, input)
	assertUnsupportedAPOnlyLive(t, f, input, cpeBefore)
}
