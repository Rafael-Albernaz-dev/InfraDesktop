package radio

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

type radioIndexMatchCase struct {
	name               string
	entries            []RadioIndexEntry
	caller             string
	age                time.Duration
	complete, proposal bool
	reason             string
}

func radioIndexMatchCases(entry RadioIndexEntry) []radioIndexMatchCase {
	return []radioIndexMatchCase{
		{"unique", []RadioIndexEntry{entry}, entry.MAC, 0, true, true, "inequívoco"},
		{"bridge last-ip", []RadioIndexEntry{entry}, "AA:BB:CC:DD:EE:10", 0, true, true, "inequívoco"},
		{"two APs",
			[]RadioIndexEntry{entry,
				{AP: radioLiveUUID(),
					MAC:       entry.MAC,
					Interface: "wlan2",
					BSSID:     "AA:BB:CC:DD:EE:01"}},
			entry.MAC,
			0,
			true,
			false,
			"mais de um"},

		{"empty", nil, entry.MAC, 0, true, false, "Sem match"},
		{"old", []RadioIndexEntry{entry}, entry.MAC, RadioIndexTTL + time.Second, true, false, "antigo"},
		{"incomplete", []RadioIndexEntry{entry}, entry.MAC, 0, false, true, "inequívoco"},
		{"bridge without last-ip",
			[]RadioIndexEntry{{AP: entry.AP,
				MAC:       entry.MAC,
				Interface: entry.Interface,
				BSSID:     entry.BSSID}},
			"AA:BB:CC:DD:EE:10",
			0,
			true,
			false,
			"Sem match"},
	}
}

func TestRadioIndexFailureWarningBoundsNamesAndPreservesPagedProposal(t *testing.T) {
	index := RadioIndex{Targets: map[string]Code{}, Labels: map[string]string{}}
	for i := 0; i < MaxDevices; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		index.Targets[id] = RadioCodeConnectFailed
		index.Labels[id] = strings.Repeat("界", 80) + " (2001:db8:ffff:ffff:ffff:ffff:ffff:ffff)"
	}
	warning := radioIndexFailureWarning(index)
	if !strings.Contains(warning, fmt.Sprintf("%d AP não respondeu", MaxDevices)) ||
		!strings.Contains(warning, "demais APs nas páginas do índice") {
		t.Fatal("failure count or navigation to remaining names lost")
	}
	for offset := 0; offset < MaxDevices; offset += RadioPageSize {
		view, code := pageRadioIndex(RadioView{Index: &index, Proposal: &RadioProposal{}, MatchReason: warning}, offset)
		if code != OK || view.Proposal == nil || strings.Count(view.MatchReason, string(RadioCodeConnectFailed)) != RadioPageSize {
			t.Fatalf("paged proposal or bounded failure names lost: code=%s", code)
		}
	}
}
