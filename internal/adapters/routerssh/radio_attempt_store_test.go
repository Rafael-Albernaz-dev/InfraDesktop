package routerssh_test

import (
	"context"
	e "infradesktop/internal/modules/equipment"
)

func (recoveryStore) UpdateEquipment(context.Context, string, func(*e.State) e.Code) e.Code {
	return e.OK
}
