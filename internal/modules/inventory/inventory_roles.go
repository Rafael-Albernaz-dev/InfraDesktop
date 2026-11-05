package inventory

import "infradesktop/internal/modules/equipment"

func roleWithoutDriver(role Role) bool { return equipment.RoleWithoutDriver(role) }
