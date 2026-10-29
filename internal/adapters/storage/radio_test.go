package storage

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"testing"
	"time"
)

func dropRadioFixtureSchema(t *testing.T, j *journey) {
	t.Helper()
	for _, table := range []string{"radio_links", "radio_current", "radio_profiles", "device_radio_roles"} {
		if _, err := j.st.unit.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.st.unit.Exec("DELETE FROM schema_migrations WHERE version=12"); err != nil {
		t.Fatal(err)
	}
}
func TestL07RadioMigrationCurrentContextAndPTP(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	var before, after string
	j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations WHERE version<=11 ORDER BY version)").Scan(&before)
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
	j.st.unit.QueryRow("SELECT group_concat(checksum,'|') FROM (SELECT checksum FROM schema_migrations WHERE version<=11 ORDER BY version)").Scan(&after)
	if before != after {
		t.Fatal("historical checksums changed")
	}
	at := time.Date(2026, 10, 29, 0, 0, 0, 0, time.UTC)
	save := func(stamp time.Time, id string, clients []equipment.RadioClient) equipment.Code {
		r := &equipment.RadioReading{ID: id, Family: "routeros-wireless-v6", Perspective: "AP", Clients: clients}
		equipment.StampRadio(r, target.Device, target.Credential.Metadata.Revision, stamp)
		return j.st.SaveRadio(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r)
	}
	if c := save(at, uuid.NewString(), []equipment.RadioClient{{MAC: "02:00:00:00:10:03"}}); c != equipment.OK {
		t.Fatal(c)
	}
	if c := save(at.Add(100*time.Millisecond), uuid.NewString(), nil); c != equipment.OK {
		t.Fatal("fractional RFC3339 ordering", c)
	}
	if c := save(at.Add(100100*time.Microsecond), uuid.NewString(), nil); c != equipment.OK {
		t.Fatal("variable precision ordering", c)
	}
	r, p, c := j.st.RadioCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || r == nil || !r.Compatible || !r.HadClients || p.Confirmed {
		t.Fatal("last good/provenance", r, p, c)
	}
	if c := j.st.ConfirmRadioFamily(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r.Family, 15, 20); c != equipment.OK {
		t.Fatal(c)
	}
	if c := j.st.UpdateEquipment(ctx, companyX, func(s *equipment.State) equipment.Code {
		for i := range s.Devices {
			if s.Devices[i].ID == target.Device.ID {
				s.Devices[i].Role = "ptp"
				s.Devices[i].Revision++
			}
		}
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal("PTP additive role", c)
	}
	st, c := j.st.ReadEquipment(ctx, companyX)
	if c != equipment.OK || st.Devices[0].Role != "ptp" {
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
func TestL07LinkReceiptReplayAndSourceReplacement(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	o := ppSynthetic(t, j, target, 1, "synthetic-radio")
	ap, cpe := uuid.NewString(), uuid.NewString()
	if c := j.st.UpdateEquipment(ctx, companyX, func(s *equipment.State) equipment.Code {
		a, d := target.Device, target.Device
		a.ID, a.Role = ap, "ap"
		d.ID, d.Role, d.Address = cpe, "cpe", o.Rows[0].Address
		s.Devices = append(s.Devices, a, d)
		return equipment.OK
	}); c != equipment.OK {
		t.Fatal(c)
	}
	l := equipment.RadioLink{Concentrator: target.Device.ID, Scope: equipment.PPScope, Login: o.Rows[0].Login, AP: ap, CPE: cpe, MAC: "02:00:00:00:10:03", Interface: "wlan1", BSSID: "02:00:00:00:10:05", Origin: "manual", Evidence: "synthetic confirmed association", At: "2026-10-29T00:00:00Z"}
	source := equipment.PPInput{DeviceID: target.Device.ID, ObservationID: o.ID, RowID: o.Rows[0].ID}
	operation := uuid.NewString()
	if c := j.st.SaveRadioLink(ctx, companyX, l, operation, source); c != equipment.OK {
		t.Fatal(c)
	}
	l.At = "2026-10-29T00:00:01Z"
	if c := j.st.SaveRadioLink(ctx, companyX, l, operation, source); c != equipment.OK {
		t.Fatal("same effect replay", c)
	}
	links, c := j.st.RadioLinks(ctx, companyX, target.Device.ID, l.Login)
	if c != equipment.OK || len(links) != 1 || links[0].At != "2026-10-29T00:00:00Z" {
		t.Fatal("replay changed confirmation", c, links)
	}
	ppSynthetic(t, j, target, 1, "synthetic-radio")
	if c := j.st.SaveRadioLink(ctx, companyX, l, uuid.NewString(), source); c != equipment.ObservationMissing {
		t.Fatal("retired source accepted", c)
	}
	links, c = j.st.RadioLinks(ctx, companyX, target.Device.ID, l.Login)
	if c != equipment.OK || len(links) != 1 {
		t.Fatal("stale source deleted durable link", c)
	}
}

func TestL07ZeroClientsHistoryIsPerInterfaceAndBSSID(t *testing.T) {
	j, target := ppStoreSetup(t)
	ctx := context.Background()
	at := time.Now().UTC()
	one, zero := 1, 0
	save := func(n int, master, virtual *int) {
		t.Helper()
		r := &equipment.RadioReading{ID: uuid.NewString(), Family: "routeros-wireless-v6", Perspective: "AP", Interfaces: []equipment.RadioInterface{{Name: "wlan1", BSSID: "02:00:00:00:10:05", Clients: master}, {Name: "virtual", Master: "wlan1", BSSID: "02:00:00:00:10:06", Clients: virtual}}}
		equipment.StampRadio(r, target.Device, target.Credential.Metadata.Revision, at.Add(time.Duration(n)*time.Second))
		if c := j.st.SaveRadio(ctx, companyX, target.Device, target.Credential.Metadata.Revision, r); c != equipment.OK {
			t.Fatal(c)
		}
	}
	save(0, &one, &zero)
	r, _, c := j.st.RadioCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || !r.Interfaces[0].HadClients || r.Interfaces[1].HadClients {
		t.Fatal("fresh virtual zero inherited master clients", r, c)
	}
	save(1, &zero, &one)
	save(2, &zero, &zero)
	r, _, c = j.st.RadioCurrent(ctx, companyX, target.Device.ID)
	if c != equipment.OK || !r.Interfaces[0].HadClients || !r.Interfaces[1].HadClients {
		t.Fatal("real 1 to 0 history lost", r, c)
	}
}
