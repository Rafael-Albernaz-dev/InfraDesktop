package radio

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type indexViewStore struct {
	radioLiveStore
	st State
}

func (s indexViewStore) ReadEquipment(context.Context, string) (State, Code) { return s.st, OK }
func TestRadioIndexMatchDecisions(t *testing.T) {
	s, q, _, base := radioHarness(t, &radioLiveRemote{})
	st := State{Devices: []Device{base.devices[base.link.AP],
		base.devices[base.link.CPE]},
		CredentialDefaults: map[string]string{string(RoleCPE): "chosen"}}
	s.Store = indexViewStore{base, st}
	entry := RadioIndexEntry{AP: base.link.AP, MAC: base.link.MAC, Interface: base.link.Interface, BSSID: base.link.BSSID, LastIP: "192.0.2.2"}
	row := PPRow{Caller: base.link.MAC, Address: "192.0.2.2"}
	cases := radioIndexMatchCases(entry)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s.radioIndexes = map[string]RadioIndex{radioIndexKey(q): {At: PPTime(s.Now().Add(-tc.age)),
				Complete: tc.complete,
				Entries:  tc.entries,
				Context:  radioIndexContext(st)}}
			r := row
			r.Caller = tc.caller
			v := s.radioIndexView(q, &r, st)
			if (v.Proposal != nil) != tc.proposal || !strings.Contains(v.MatchReason, tc.reason) {
				t.Fatalf("proposal=%+v reason=%s", v.Proposal, v.MatchReason)
			}
			if tc.proposal && (v.Proposal.Choice.MAC != entry.MAC || v.Proposal.Choice.BSSID != entry.BSSID) {
				t.Fatal("wrong identity")
			}
		})
	}
	s.radioIndexes = map[string]RadioIndex{radioIndexKey(q): {At: PPTime(s.Now()),
		Complete: true,
		Entries:  []RadioIndexEntry{entry},
		Context:  radioIndexContext(st)}}
	st.Devices[0].Revision++
	if v := s.radioIndexView(q, &row, st); v.Proposal != nil || !strings.Contains(v.MatchReason, "inventário alterado") {
		t.Fatal("stale context accepted")
	}
}

func TestRadioIndexMetadataFitsIPCAndPagesAllTargets(t *testing.T) {
	index := RadioIndex{Targets: map[string]Code{}, Labels: map[string]string{}}
	for i := 0; i < MaxDevices; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		index.Targets[id] = RadioCodeRemoteAuthFailed
		index.Labels[id] = strings.Repeat("界", 80) + " (2001:db8:ffff:ffff:ffff:ffff:ffff:ffff)"
	}
	seen := map[string]bool{}
	for offset := 0; offset < MaxDevices; offset += RadioPageSize {
		v, c := pageRadioIndex(RadioView{Index: &index}, offset)
		if c != OK || len(v.Index.Targets) > RadioPageSize {
			t.Fatal("bad page", c)
		}
		for id := range v.Index.Targets {
			if seen[id] {
				t.Fatal("duplicate page target")
			}
			seen[id] = true
		}
	}
	if len(seen) != MaxDevices {
		t.Fatal("targets lost", len(seen))
	}
}
