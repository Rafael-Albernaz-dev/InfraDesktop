package storage

import "testing"

func TestInventoryColumnsGlobalMigrationRollbackAndUpgrade(t *testing.T) {
	j := newJourney(t)
	var name string
	if err := j.st.db.QueryRow("SELECT name FROM account WHERE id=1").Scan(&name); err != nil {
		t.Fatal(err)
	}
	if _, err := j.st.db.Exec(`DROP TABLE account_inventory_preferences;
		DELETE FROM schema_migrations WHERE version=3;
		CREATE TRIGGER reject_column_migration BEFORE INSERT ON schema_migrations
		WHEN NEW.version=3 BEGIN SELECT RAISE(ABORT,'injected migration failure'); END`); err != nil {
		t.Fatal(err)
	}
	tx, err := j.st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = migrateGlobal(tx); err == nil {
		t.Fatal("expected conflicting migration to fail")
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var version int
	var after string
	if err = j.st.db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 2 {
		t.Fatal("migration did not roll back", version, err)
	}
	if err = j.st.db.QueryRow("SELECT name FROM account WHERE id=1").Scan(&after); err != nil || after != name {
		t.Fatal("account changed on rollback", err)
	}
	var tables int
	err = j.st.db.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='account_inventory_preferences'").Scan(&tables)
	if err != nil || tables != 0 {
		t.Fatal("partially created preference table survived rollback", tables, err)
	}
	verifyColumnsMigrationUpgrade(t, j)
}

func verifyColumnsMigrationUpgrade(t *testing.T, j *journey) {
	t.Helper()
	var version int
	var err error
	if _, err = j.st.db.Exec("DROP TRIGGER reject_column_migration"); err != nil {
		t.Fatal(err)
	}
	tx, err := j.st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = migrateGlobal(tx); err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = j.st.db.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&version); err != nil || version != 3 {
		t.Fatal("upgrade failed", version, err)
	}
}
