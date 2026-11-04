package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestRadioDeviceLinksUseStoredAssociationForBothEndpoints(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	observation := ppSynthetic(t, j, target, 1, "synthetic-linked-login")
	ap, cpe := uuid.NewString(), uuid.NewString()
	code := j.st.UpdateEquipment(ctx, companyX, func(state *equipment.State) equipment.Code {
		a, c := target.Device, target.Device
		a.ID, a.Role = ap, equipment.RoleAP
		c.ID, c.Role, c.Address = cpe, equipment.RoleCPE, observation.Rows[0].Address
		state.Devices = append(state.Devices, a, c)
		return equipment.OK
	})
	if code != equipment.OK {
		t.Fatal(code)
	}
	link := equipment.RadioLink{Concentrator: target.Device.ID, Scope: equipment.PPScope, Login: observation.Rows[0].Login,
		AP: ap, CPE: cpe, MAC: "02:00:00:00:00:01", Interface: "wlan1", BSSID: "02:00:00:00:00:02",
		Origin: "manual", Evidence: "synthetic association", At: "2026-11-04T12:00:00Z"}
	source := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: observation.ID, RowID: observation.Rows[0].ID}
	if c := j.st.SaveRadioLink(ctx, companyX, link, uuid.NewString(), source); c != equipment.OK {
		t.Fatal(c)
	}
	for _, id := range []string{ap, cpe} {
		links, c := j.st.RadioDeviceLinks(ctx, companyX, id)
		if c != equipment.OK || len(links) != 1 || links[0].CPE != cpe || links[0].Concentrator != target.Device.ID {
			t.Fatal("stored endpoint association lost", c, links)
		}
	}
	links, c := j.st.RadioDeviceLinks(ctx, companyX, uuid.NewString())
	if c != equipment.OK || len(links) != 0 {
		t.Fatal("unbound endpoint gained association", c, links)
	}
	if _, c := j.st.RadioDeviceLinks(ctx, uuid.NewString(), ap); c != equipment.Changed {
		t.Fatal("cross-company link escaped", c)
	}
}
