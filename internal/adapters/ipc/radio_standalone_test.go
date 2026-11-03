package ipc

import (
	"infradesktop/internal/modules/equipment"
	"testing"
)

func TestStandaloneIPCTofuSemSessao(t *testing.T) {
	j := standaloneStationJourney(t)
	id, cred := createStandalonePTPTarget(t, j)
	job := startStandaloneRadioOnce(t, j, id, 1)
	v := awaitStandaloneRadioJob(t, j, job)
	if v == nil || v.Job.Code != equipment.OK || v.Reading == nil || v.Reading.Model != "RBLHG-5HPnD" || v.Reading.Mode != "station-bridge" {
		t.Fatalf("view=%+v", v)
	}
	assertStandaloneStoredReading(t, j, id)
	assertStandaloneTOFUReceipt(t, j, id, cred)
}
func TestStandaloneDesconhecidoPreservaLeituraPerfilEVinculo(t *testing.T) {
	j, observation, row := unknownRadioSourceJourney(t)
	peer := unknownRadioIdentityPeer(t)
	defer peer.Close()
	ap, id := enrollPreservedRadioTargets(t, j, peer)
	target, saved := saveKnownStandaloneRadio(t, j, id)
	before := savePreservedRadioLink(t, j, ap, id, observation, row)
	queryUnknownStandaloneRadio(t, j, id, target.Device.Revision)
	assertStandaloneContextPreserved(t, j, id, saved, before, row.Login)
}
