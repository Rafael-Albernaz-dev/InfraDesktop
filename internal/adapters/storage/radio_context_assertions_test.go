package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func assertRadioSchemaAtomicAndAdditive(t *testing.T, j *journey) {
	t.Helper()
	var before, after string
	j.st.unit.QueryRow(("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations" +
		" WHERE version<=11 ORDER BY version)")).Scan(&before)
	dropRadioFixtureSchema(t, j)
	original := companyRadioSchema
	defer func() { companyRadioSchema = original }()
	companyRadioSchema = original + ";invalid SQL;"
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err == nil {
		t.Fatal("non-atomic invalid migration accepted")
	}
	var n int
	j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='radio_current'").Scan(&n)
	if n != 0 {
		t.Fatal("partial migration escaped")
	}
	companyRadioSchema = original
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err != nil {
		t.Fatal(err)
	}
	j.st.unit.QueryRow(("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations" +
		" WHERE version<=11 ORDER BY version)")).Scan(&after)
	if before != after {
		t.Fatal("historical checksums changed")
	}
}
func assertRadioContextAndPTP(t *testing.T, j *journey, target equipment.PPTarget, at time.Time,
	save func(time.Time, string, []equipment.RadioClient) equipment.Code) {
	t.Helper()
	ctx := context.Background()
	r, p, c := j.st.RadioCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || r == nil || !r.Compatible || !r.HadClients || p.Confirmed {
		t.Fatal("last good/provenance", r, p, c)
	}
	if c := j.st.ConfirmRadioFamily(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r.Family, 15, 20); c != equipment.OK {
		t.Fatal(c)
	}
	changeRadioFixtureRolePTP(t, j, target)
	st, c := j.st.ReadEquipment(ctx, companyX)
	if c != equipment.OK || st.Devices[0].Role != equipment.RolePTP {
		t.Fatal("PTP projection", c, st.Devices)
	}
	r, p, c = j.st.RadioCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || r == nil || r.Compatible || p.Confirmed {
		t.Fatal("old context confirms current", r, p, c)
	}
	if c := save(at.Add(time.Second), uuid.NewString(), nil); c != equipment.Revision {
		t.Fatal("stale publication", c)
	}
	if err := j.st.validateUnit(j.st.unit, companyX); err != nil {
		t.Fatal("reopen", err)
	}
}
func changeRadioFixtureRolePTP(t *testing.T, j *journey, target equipment.PPTarget) {
	t.Helper()
	ctx := context.Background()
	if c := j.st.UpdateEquipment(ctx, companyX, func(s *equipment.State) equipment.Code {
		for i := range s.Devices {
			if s.Devices[i].ID == target.Device.ID {
				s.Devices[i].Role = equipment.RolePTP
				s.Devices[i].Revision++
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal("PTP additive role", c)
	}
}
