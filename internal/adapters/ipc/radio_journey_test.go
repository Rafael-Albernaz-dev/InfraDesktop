package ipc

import (
	"encoding/json"
	"fmt"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
)

func TestRadioIndexAndConfirmThroughIPC(t *testing.T) {
	s := newRadioIndexIPCFixture(t)
	s.verifyInitialProposal()
	s.verifyDefaultRace()
	confirmed := s.confirmAndVerifyReplay()
	s.verifyAtomicLink(confirmed)
	s.verifyRefusal()
}

func TestRadioIndexPublicPageFitsFrame(t *testing.T) {
	index := equipment.RadioIndex{Total: equipment.MaxDevices, Targets: map[string]equipment.Code{}, Labels: map[string]string{}}
	for i := 0; i < equipment.RadioPageSize; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i)
		index.Targets[id] = equipment.RadioCodeRemoteAuthFailed
		index.Labels[id] = strings.Repeat("界", 80) + " (2001:db8:ffff:ffff:ffff:ffff:ffff:ffff)"
	}
	raw,
		err := json.Marshal(equipment.RadioView{Index: &index,
		Proposal: &equipment.RadioProposal{APName: strings.Repeat("界",
			80),
			APAddress: "2001:db8:ffff:ffff:ffff:ffff:ffff:ffff",
			CredentialAlias: strings.Repeat("界",
				80)}})
	if err != nil || len(raw) > 48*1024 {
		t.Fatal("public index page exceeds IPC frame", len(raw), err)
	}
}
