package routerssh

import "infradesktop/internal/modules/equipment"

// Admission precedes both the network connection and any credential callback.
func admitsRadioSession(role equipment.Role, family equipment.RadioFamily, link *equipment.RadioLink) bool {
	if family == equipment.RadioRouterOSWirelessV6 && link != nil {
		if _, ok := equipment.RadioMAC(link.MAC); !ok {
			return false
		}
	}

	if family == equipment.RadioStandalone {
		if link != nil {
			return false
		}
		return role == equipment.RoleAP || role == equipment.RoleCPE || role == equipment.RolePTP
	}
	if family == equipment.RadioUnsupported {
		return false
	}
	if !equipment.IsRadioFamily(family) && family != equipment.RadioAuto {
		return false
	}
	if family == equipment.RadioRouterOSWirelessV6 {
		if role == equipment.RoleAP {
			return true
		}
		if link != nil {
			return false
		}
		return role == equipment.RoleCPE || role == equipment.RolePTP
	}
	return role == equipment.RoleCPE
}
