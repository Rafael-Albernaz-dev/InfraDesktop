package storage

import (
	"database/sql"
	_ "embed"
)

//go:embed company_013_radio_journey.sql
var companyRadioJourneySchema string

//go:embed company_014_inventory_block.sql
var companyInventoryBlockSchema string

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
