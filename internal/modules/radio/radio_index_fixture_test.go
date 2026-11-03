package radio

import "time"

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
		{"incomplete", []RadioIndexEntry{entry}, entry.MAC, 0, false, false, "incompleto"},
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
