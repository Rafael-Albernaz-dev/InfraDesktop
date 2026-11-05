package storage

import (
	"context"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"reflect"
	"testing"
)

func TestRadioMigration13PreservesFixtureAndRollsBack(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	setupEquipment(t, j, s, 2222)
	before, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	dropInventoryBlockFixtureSchema(t, j)
	if _, err := j.st.unit.Exec("DROP TABLE radio_credential_defaults; DELETE FROM schema_migrations WHERE version=13"); err != nil {
		t.Fatal(err)
	}
	original := companyRadioJourneySchema
	defer func() { companyRadioJourneySchema = original }()
	companyRadioJourneySchema = original + ";invalid SQL;"
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err == nil {
		t.Fatal("invalid migration accepted")
	}
	var n int
	if err := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='radio_credential_defaults'").Scan(&n); err != nil || n != 0 {
		t.Fatal("partial migration escaped", err, n)
	}
	companyRadioJourneySchema = original
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err != nil {
		t.Fatal(err)
	}
	after, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK {
		t.Fatal(c)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("migration changed credentials/vault/inventory/operations")
	}
	if err := j.st.unit.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&n); err != nil || n != 14 {
		t.Fatal("schema14 missing", n, err)
	}
}
