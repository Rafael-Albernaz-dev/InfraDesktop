package storage

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"infradesktop/internal/modules/access"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/testsupport/sshfixture"
)

func setSyntheticBlock(t *testing.T, q *equipment.Command, value string) { t.Helper(); q.Block = value }
func readSyntheticBlock(t *testing.T, d *equipment.Device) string        { t.Helper(); return d.Block }
func TestNewRolesAreSavedWithoutStartingRemoteReads(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	peer, err := sshfixture.Start(sshfixture.Config{Family: "v6"})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	_, base := setupEquipment(t, j, s, peer.Port())
	for _, role := range []string{"edge", "switch", "station", "power-monitor", "ups", "rectifier", "solar-controller", "battery-monitor"} {
		t.Run(role, func(t *testing.T) {
			q := base
			q.ID = uuid.NewString()
			q.Operation = uuid.NewString()
			q.Role = equipment.Role(role)
			created := equipmentCall(t, s, q, equipment.OK)
			if created.Device == nil || string(created.Device.Role) != role {
				t.Fatal("role lost on create")
			}
			get := equipmentScope(j)
			get.Op = "inventory.get"
			get.ID = q.ID
			if got := equipmentCall(t, s, get, equipment.OK).Device; got == nil || string(got.Role) != role {
				t.Fatal("role lost on read")
			}
			for _, op := range []string{"trust.inspect", "device.query-start"} {
				read := equipmentScope(j)
				read.Op = op
				read.ID = q.ID
				read.Revision = 1
				read.Operation = uuid.NewString()
				if _, c := s.Handle(context.Background(), read); c != equipment.RadioCodeUnsupported {
					t.Fatalf("%s got %s", op, c)
				}
			}
			if peer.Handshakes.Load() != 0 || peer.Auth.Load() != 0 {
				t.Fatal("unsupported role contacted the peer")
			}
		})
	}
}
func TestDeviceBlockIsValidatedPersistedAndIsolated(t *testing.T) {
	j := newJourney(t)
	j.create(companyX, "X", access.OK)
	j.create(companyY, "Y", access.OK)
	j.selectCompany(companyX, access.OK)
	s := newEquipment(t, j)
	_, base := setupEquipment(t, j, s, 2222)
	q := base
	q.ID = uuid.NewString()
	q.Operation = uuid.NewString()
	setSyntheticBlock(t, &q, "Torre Norte")
	got := equipmentCall(t, s, q, equipment.OK).Device
	if got == nil || readSyntheticBlock(t, got) != "Torre Norte" {
		t.Fatal("created block missing")
	}
	update := q
	update.Op = "inventory.update"
	update.Operation = uuid.NewString()
	update.Revision = 1
	setSyntheticBlock(t, &update, "Área Sul")
	equipmentCall(t, s, update, equipment.OK)
	get := equipmentScope(j)
	get.Op = "inventory.get"
	get.ID = q.ID
	if got = equipmentCall(t, s, get, equipment.OK).Device; got == nil || readSyntheticBlock(t, got) != "Área Sul" {
		t.Fatal("updated block missing")
	}
	got = verifyBlockBoundaries(t, s, update, got)
	old := get
	j.selectCompany(companyY, access.OK)
	equipmentCall(t, s, old, equipment.Changed)
	foreign := equipmentScope(j)
	foreign.Op = "inventory.get"
	foreign.ID = q.ID
	equipmentCall(t, s, foreign, equipment.Invalid)
	j.selectCompany(companyX, access.OK)
	get = equipmentScope(j)
	get.Op = "inventory.get"
	get.ID = q.ID
	if got = equipmentCall(t, s, get, equipment.OK).Device; got == nil || readSyntheticBlock(t, got) != strings.Repeat("á", 80) {
		t.Fatal("company change lost block")
	}
}
func verifyBlockBoundaries(t *testing.T, s *equipment.Service, update equipment.Command, got *equipment.Device) *equipment.Device {
	t.Helper()
	for _, valid := range []string{"", strings.Repeat("á", 80)} {
		candidate := update
		candidate.Operation = uuid.NewString()
		candidate.Revision = got.Revision
		setSyntheticBlock(t, &candidate, valid)
		got = equipmentCall(t, s, candidate, equipment.OK).Device
		if got == nil || readSyntheticBlock(t, got) != valid {
			t.Fatal("valid block was not retained")
		}
	}
	for _, invalid := range []string{strings.Repeat("a", 81), "bad\nblock", string([]byte{0xff})} {
		candidate := update
		candidate.Operation = uuid.NewString()
		candidate.Revision = got.Revision
		setSyntheticBlock(t, &candidate, invalid)
		if _, c := s.Handle(context.Background(), candidate); c == equipment.OK {
			t.Fatal("invalid block accepted")
		}
	}
	return got
}
