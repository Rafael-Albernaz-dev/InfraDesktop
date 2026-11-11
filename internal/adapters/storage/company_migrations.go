package storage

import (
	"database/sql"
	"fmt"
)

// Company migrations, in version order. Their bytes are checksummed into
// schema_migrations, so the files never change once released.
var (
	companySchema                  = companyMigration("001_schema.sql")
	companyBrandSchema             = companyMigration("002_brand.sql")
	companyEquipmentSchema         = companyMigration("003_equipment.sql")
	companyOperationalSchema       = companyMigration("004_operational.sql")
	companyPPPoESchema             = companyMigration("005_pppoe.sql")
	companyPPDiagnosticSchema      = companyMigration("006_pppoe_diagnostics.sql")
	companySessionsOfflineSchema   = companyMigration("007_sessions_offline.sql")
	companySessionContextSchema    = companyMigration("008_session_context.sql")
	companyDeviceMonitoringSchema  = companyMigration("009_device_monitoring.sql")
	companyEquipmentInsightsSchema = companyMigration("010_equipment_insights.sql")
	companyContextPeaksSchema      = companyMigration("011_context_peaks.sql")
	companyRadioSchema             = companyMigration("012_radio.sql")
	companyRadioJourneySchema      = companyMigration("013_radio_journey.sql")
	companyInventoryBlockSchema    = companyMigration("014_inventory_block.sql")
)

// companyMigration reads one company migration from the single migrations embed.
func companyMigration(name string) string {
	b, err := migrations.ReadFile("migrations/company/" + name)
	if err != nil {
		panic(fmt.Sprintf("company migration %s missing from embed: %v", name, err))
	}
	return string(b)
}

func companySchemas() []string {
	return []string{companySchema, companyBrandSchema, companyEquipmentSchema, companyOperationalSchema,
		companyPPPoESchema, companyPPDiagnosticSchema, companySessionsOfflineSchema, companySessionContextSchema,
		companyDeviceMonitoringSchema, companyEquipmentInsightsSchema, companyContextPeaksSchema,
		companyRadioSchema, companyRadioJourneySchema, companyInventoryBlockSchema}
}

func applyCompanyMigrations(db *sql.DB, current int, schemas []string) error {
	for version := current + 1; version <= len(schemas); version++ {
		if err := applyCompanyMigration(db, version, schemas[version-1]); err != nil {
			return err
		}
	}
	return nil
}

func applyCompanyMigration(db *sql.DB, version int, source string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(source); err == nil && version == 7 {
		err = migrateReadReceipts(tx)
	}
	if err == nil {
		_, err = tx.Exec("INSERT INTO schema_migrations VALUES(?,?)", version, checksum([]byte(source)))
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
