package storage

import (
	"context"
	"reflect"
	"testing"

	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
)

// Synthetic rewind removes migration 14 before older migration fixtures are reconstructed.
func dropInventoryBlockFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	if _, err := j.st.unit.Exec(`DROP TABLE device_inventory_roles; ALTER TABLE devices DROP COLUMN block;
  DELETE FROM schema_migrations WHERE version=14`); err != nil {
		t.Fatal(err)
	}
}
func TestInventoryBlockMigrationPreservesStateAndRollsBack(t *testing.T) {
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
	original := companyInventoryBlockSchema
	defer func() { companyInventoryBlockSchema = original }()
	companyInventoryBlockSchema = original + "; invalid SQL;"
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err == nil {
		t.Fatal("invalid migration accepted")
	}
	var n int
	if err := j.st.unit.QueryRow("SELECT count(*) FROM pragma_table_info('devices') WHERE name='block'").Scan(&n); err != nil || n != 0 {
		t.Fatal("block DDL escaped rollback", n, err)
	}
	if err := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='device_inventory_roles'").Scan(&n); err != nil || n != 0 {
		t.Fatal("role DDL escaped rollback", n, err)
	}
	if err := j.st.unit.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&n); err != nil || n != 13 {
		t.Fatal("ledger advanced after rollback", n, err)
	}
	companyInventoryBlockSchema = original
	if err := j.st.validateUnitIdentity(j.st.unit, companyX); err != nil {
		t.Fatal(err)
	}
	after, c := j.st.ReadEquipment(context.Background(), companyX)
	if c != equipment.OK || !reflect.DeepEqual(before, after) {
		t.Fatal("legacy state changed during migration", c)
	}
	if err := j.st.unit.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&n); err != nil || n != 14 {
		t.Fatal("schema 14 missing", n, err)
	}
}
