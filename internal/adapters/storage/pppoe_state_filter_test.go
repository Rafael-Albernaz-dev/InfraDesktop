package storage

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"testing"
)

func stateFilterFixture(t *testing.T) (*journey, equipment.PPTarget, *equipment.PPObservation) {
	j, target := ppStoreSetup(t)
	observation := ppSynthetic(t, j, target, 35, "synthetic-state")
	for _, row := range observation.Rows[:17] {
		_, err := j.st.unit.Exec("UPDATE pppoe_rows SET row_json=json_set(row_json,'$.state','OFFLINE') "+
			"WHERE observation_id=? AND row_id=?", observation.ID, row.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	return j, target, observation
}

func TestPPPoEStateFilterPrecedesPagination(t *testing.T) {
	j, target, observation := stateFilterFixture(t)
	for _, tc := range []struct {
		state string
		total int
	}{{"ONLINE", 18}, {"OFFLINE", 17}} {
		in := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: observation.ID,
			Query: &equipment.PPQuery{State: tc.state, Sort: "state"}}
		first, c := j.st.PPPage(context.Background(), companyX, in)
		if c != equipment.OK || first.Total != tc.total || len(first.Rows) != 16 || !first.More {
			t.Fatalf("filter not applied before page: %s %+v", c, first)
		}
		for _, row := range first.Rows {
			if row.State != tc.state {
				t.Fatal("other state escaped filter", row.State)
			}
		}
		in.Offset = 16
		last, c := j.st.PPPage(context.Background(), companyX, in)
		if c != equipment.OK || last.Total != tc.total || len(last.Rows) != tc.total-16 || last.More {
			t.Fatal("filtered second page", c, last)
		}
	}
}

func TestPPPoEStateOrderingAndInvalidFilter(t *testing.T) {
	j, target, observation := stateFilterFixture(t)
	in := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: observation.ID,
		Query: &equipment.PPQuery{Sort: "state"}}
	first, c := j.st.PPPage(context.Background(), companyX, in)
	if c != equipment.OK || first.Total != 35 || first.Rows[0].State != "OFFLINE" {
		t.Fatal("state ascending", c, first)
	}
	in.Query.Desc = true
	first, c = j.st.PPPage(context.Background(), companyX, in)
	if c != equipment.OK || first.Rows[0].State != "ONLINE" {
		t.Fatal("state descending", c, first)
	}
	in.Query.State = "invalid"
	if _, c = j.st.PPPage(context.Background(), companyX, in); c != equipment.Invalid {
		t.Fatal("invalid state accepted", c)
	}
}
