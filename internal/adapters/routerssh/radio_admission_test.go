package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestRadioSessionAdmissionByFamilyRoleAndLink(t *testing.T) {
	families := []equipment.RadioFamily{equipment.RadioStandalone, equipment.RadioRouterOSWirelessV6,
		equipment.RadioAirOSV6, equipment.RadioIntelbrasWOM, equipment.RadioAuto, equipment.RadioUnsupported, "unknown"}
	roles := []equipment.Role{equipment.RoleAP, equipment.RoleCPE, equipment.RolePTP, equipment.RoleConcentrator, "unknown"}
	allowed := map[equipment.RadioFamily]map[equipment.Role][2]bool{
		equipment.RadioStandalone:         {equipment.RoleAP: {true, false}, equipment.RoleCPE: {true, false}, equipment.RolePTP: {true, false}},
		equipment.RadioRouterOSWirelessV6: {equipment.RoleAP: {true, true}, equipment.RoleCPE: {true, false}, equipment.RolePTP: {true, false}},
		equipment.RadioAirOSV6:            {equipment.RoleCPE: {true, true}},
		equipment.RadioIntelbrasWOM:       {equipment.RoleCPE: {true, true}},
		equipment.RadioAuto:               {equipment.RoleCPE: {true, true}},
	}
	for _, family := range families {
		for _, role := range roles {
			for i, link := range []*equipment.RadioLink{nil, {MAC: "AA:BB:CC:DD:EE:FF"}} {
				if got := admitsRadioSession(role, family, link); got != allowed[family][role][i] {
					t.Errorf("family=%s role=%s linked=%t admission=%t", family, role, link != nil, got)
				}
			}
		}
	}
}

func TestRadioSessionInvalidLinkStopsBeforeAuthorizationAndNetwork(t *testing.T) {
	calls := 0
	check := func() equipment.Code { calls++; return equipment.Cancelled }
	session, code := openRadioSession(context.Background(), equipment.Device{Role: equipment.RoleAP}, "", "",
		equipment.RadioRouterOSWirelessV6, &equipment.RadioLink{MAC: "invalid"}, check, false)
	if code != equipment.Invalid || session != nil || calls != 0 {
		t.Fatalf("session=%v code=%s checks=%d", session, code, calls)
	}
}
