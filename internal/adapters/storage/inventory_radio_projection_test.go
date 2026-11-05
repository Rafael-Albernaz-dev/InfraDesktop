package storage

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
)

func TestInventoryProjectsSavedRadioForAPAndCPE(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	devices := []equipment.Device{}
	for _, role := range []equipment.Role{equipment.RoleAP, equipment.RoleCPE} {
		d := target.Device
		d.ID, d.Role, d.Name = uuid.NewString(), role, string(role)
		devices = append(devices, d)
	}
	if c := j.st.UpdateEquipment(ctx, companyX, func(s *equipment.State) equipment.Code {
		s.Devices = append(s.Devices, devices...)
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	saveSyntheticInventoryRadios(t, j, devices, target.Credential.Metadata.Revision)
	state, c := j.st.ReadEquipment(ctx, companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	for _, d := range devices {
		var got *equipment.Device
		for i := range state.Devices {
			if state.Devices[i].ID == d.ID {
				got = &state.Devices[i]
			}
		}
		raw, _ := json.Marshal(got)
		var projected struct {
			Radio struct {
				Version, Model string
				CPU            *int   `json:"cpu"`
				At             string `json:"observed_at"`
			} `json:"radio_summary"`
		}
		if err := json.Unmarshal(raw, &projected); err != nil || projected.Radio.Version != "6.49" ||
			projected.Radio.Model != "synthetic" || projected.Radio.CPU == nil ||
			*projected.Radio.CPU != 7 || projected.Radio.At == "" {
			t.Fatalf("%s radio projection absent: %s (%v)", d.Role, raw, err)
		}
	}
	verifyRadioModelFilter(t, j)
}

func verifyRadioModelFilter(t *testing.T, j *journey) {
	t.Helper()
	s := newEquipment(t, j)
	q := equipmentScope(j)
	q.Op, q.Model = "inventory.list", "synthetic"
	got := equipmentCall(t, s, q, equipment.OK)
	if len(got.Devices) != 2 || len(got.Models) != 1 || got.Models[0] != "synthetic" {
		t.Fatal("radio model missing from selector and filter", got.Models, len(got.Devices))
	}
}

func saveSyntheticInventoryRadios(t *testing.T, j *journey, devices []equipment.Device, credentialRevision uint64) {
	t.Helper()
	ctx := context.Background()
	for _, d := range devices {
		perspective := "CPE"
		if d.Role == equipment.RoleAP {
			perspective = "AP"
		}
		r := &equipment.RadioReading{ID: uuid.NewString(), Family: equipment.RadioRouterOSWirelessV6,
			Version: "6.49", Model: "synthetic", DeviceUptime: "1d", CPU: []int{7},
			Perspective: perspective, Clients: []equipment.RadioClient{{MAC: "02:00:00:00:00:01"}},
			Metrics: []equipment.RadioMetric{{Name: "voltage", Value: 24, Quality: "observed"}}}
		equipment.StampRadio(r, d, credentialRevision, time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC))
		if c := j.st.SaveRadio(ctx, companyX, d, credentialRevision, r); c != equipment.OK {
			t.Fatal(c)
		}
	}
}
