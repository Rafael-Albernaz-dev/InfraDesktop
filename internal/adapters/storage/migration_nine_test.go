package storage

import "testing"

const firstEightChecksums = "SELECT group_concat(checksum,'|') FROM schema_migrations WHERE version<=8 ORDER BY version"

func TestL06T2MigrationNineAtomicAndFirstEightUnchanged(t *testing.T) {
	j, _ := ppStoreSetup(t)
	original := companyDeviceMonitoringSchema
	defer func() { companyDeviceMonitoringSchema = original }()
	var before string
	if e := j.st.unit.QueryRow(firstEightChecksums).Scan(&before); e != nil {
		t.Fatal(e)
	}
	dropRadioFixtureSchema(t, j)
	for _, table := range []string{
		"device_context_samples", "device_equipment_snapshot", "device_equipment_events", "device_monitoring_points",
		"device_session_transitions", "device_monitoring_checkpoint", "device_monitoring_limits",
		"device_monitoring_generation", "device_monitoring_packages", "device_monitoring_package_status",
	} {
		if _, e := j.st.unit.Exec("DROP TABLE " + table); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version>=9"); e != nil {
		t.Fatal(e)
	}
	companyDeviceMonitoringSchema = original + "; invalid SQL;"
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e == nil {
		t.Fatal("bad migration accepted")
	}
	var n int
	if e := j.st.unit.QueryRow("SELECT count(*) FROM sqlite_master WHERE name='device_monitoring_points'").Scan(&n); e != nil || n != 0 {
		t.Fatal("partial migration escaped transaction", n, e)
	}
	companyDeviceMonitoringSchema = original
	if e := j.st.validateUnitIdentity(j.st.unit, companyX); e != nil {
		t.Fatal(e)
	}
	var after string
	if e := j.st.unit.QueryRow(firstEightChecksums).Scan(&after); e != nil || before != after {
		t.Fatal("applied migration changed", e)
	}
}
