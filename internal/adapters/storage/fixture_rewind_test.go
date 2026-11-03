package storage

import "testing"

// latestCompanySchema is the newest company migration the synthetic fixtures know.
const latestCompanySchema = 13

// dropRadioFixtureSchema rewinds the additive radio migrations 12 and 13 so the
// synthetic fixtures can rebuild an older unit; the PPPoE/monitoring rewind chain
// calls it first. A newer migration must be taught to the chain explicitly.
func dropRadioFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	var latest int
	if e := j.st.unit.QueryRow("SELECT max(version) FROM schema_migrations").Scan(&latest); e != nil || latest > latestCompanySchema {
		t.Fatal("company migration newer than the fixture rewind; extend the rewind chain", latest, e)
	}
	for _, table := range []string{"radio_credential_defaults", "radio_links", "radio_current", "radio_profiles", "device_radio_roles"} {
		if _, err := j.st.unit.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version>=12"); err != nil {
		t.Fatal(err)
	}
}

func dropPPDiagnosticFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	dropRadioFixtureSchema(t, j)
	for _, table := range []string{
		"device_context_samples", "device_equipment_snapshot", "device_equipment_events", "device_monitoring_points",
		"device_session_transitions", "device_monitoring_checkpoint", "device_monitoring_limits",
		"device_monitoring_generation", "device_monitoring_packages", "device_monitoring_package_status",
	} {
		if _, err := j.st.unit.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	v7 := "DROP TABLE equipment_read_receipt_migration; ALTER TABLE devices DROP COLUMN role; " +
		"DROP INDEX pppoe_rows_service_vlan; DELETE FROM schema_migrations WHERE version>=7"
	if _, e := j.st.unit.Exec(v7); e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{
		"pppoe_diagnostic_attempts", "pppoe_diagnostic_current", "pppoe_diagnostic_rows", "pppoe_diagnostic_observations",
	} {
		if _, err := j.st.unit.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version=6"); err != nil {
		t.Fatal(err)
	}
}
