package equipment

// RoleOther keeps the historical generic equipment capability.
const RoleOther Role = "other"

func ValidInventoryRole(role Role) bool {
	switch role {
	case RoleConcentrator, RoleAP, RoleCPE, RolePTP, RoleOther,
		RoleEdge, RoleSwitch, RoleStation, RolePowerMonitor, RoleUPS,
		RoleRectifier, RoleSolarController, RoleBatteryMonitor:
		return true
	default:
		return false
	}
}

// RoleWithoutDriver identifies the new roles for which no read driver exists.
func RoleWithoutDriver(role Role) bool {
	switch role {
	case RoleEdge, RoleSwitch, RoleStation, RolePowerMonitor, RoleUPS,
		RoleRectifier, RoleSolarController, RoleBatteryMonitor:
		return true
	default:
		return false
	}
}
