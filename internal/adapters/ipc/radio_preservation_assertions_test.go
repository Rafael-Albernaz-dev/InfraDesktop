package ipc

import (
	"context"
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"infradesktop/internal/modules/foundation"
	"testing"
	"time"
)

func saveKnownStandaloneRadio(t *testing.T, j *ppJourney, id string) (equipment.PPTarget, *equipment.RadioReading) {
	t.Helper()
	ctx := context.Background()
	target, c := j.store.ReadPPTarget(ctx, j.client.profile, id, false)
	if c != equipment.OK {
		t.Fatal(c)
	}
	saved := &equipment.RadioReading{ID: uuid.NewString(),
		Family:      equipment.RadioIntelbrasWOM,
		Version:     "WOM / iw (firmware não exposto)",
		Perspective: "CPE",
		Interfaces:  []equipment.RadioInterface{},
		Clients:     []equipment.RadioClient{},
		Metrics:     []equipment.RadioMetric{}}
	equipment.StampRadio(saved, target.Device, target.Credential.Metadata.Revision, time.Now().Add(-time.Minute))
	if c = j.store.SaveRadio(ctx, j.client.profile, target.Device, target.Credential.Metadata.Revision, saved); c != equipment.OK {
		t.Fatal(c)
	}
	if c = j.store.ConfirmRadioFamily(ctx,
		j.client.profile,
		target.Device,
		target.Credential.Metadata.Revision,
		equipment.RadioIntelbrasWOM,
		15,
		20); c != equipment.OK {
		t.Fatal(c)
	}
	return target, saved
}
func savePreservedRadioLink(t *testing.T, j *ppJourney, ap, id, observation string, row equipment.PPRow) []equipment.RadioLink {
	t.Helper()
	ctx := context.Background()
	link := equipment.RadioLink{Concentrator: j.device,
		Scope:     equipment.PPScope,
		Login:     row.Login,
		AP:        ap,
		CPE:       id,
		MAC:       "02:00:00:00:20:01",
		Interface: "wlan1",
		BSSID:     "02:00:00:00:20:02",
		Origin:    "manual",
		Evidence:  "vínculo sintético",
		At:        "2026-11-02T00:00:00Z"}
	if c := j.store.SaveRadioLink(ctx,
		j.client.profile,
		link,
		uuid.NewString(),
		equipment.PPInput{DeviceID: j.device,
			ObservationID: observation,
			RowID:         row.ID}); c != equipment.OK {
		t.Fatal(c)
	}
	before, c := j.store.RadioLinks(ctx, j.client.profile, j.device, row.Login)
	if c != equipment.OK || len(before) != 1 {
		t.Fatal("vínculo inicial", c, before)
	}
	return before
}
func queryUnknownStandaloneRadio(t *testing.T, j *ppJourney, id string, revision uint64) {
	t.Helper()
	ctx := context.Background()
	start := j.client.Call(ctx,
		Request{Op: "radio.start",
			ExpectedProfile:    j.client.profile,
			ExpectedActivation: j.client.activation,
			Radio: &equipment.RadioInput{DeviceID: id,
				Revision:  revision,
				Operation: uuid.NewString(),
				Mode:      equipment.RadioOnce,
				Family:    equipment.RadioStandalone}})
	if start.Code != foundation.OK {
		t.Fatal(start.Code)
	}
	v := awaitStandaloneRadioJob(t, j, start.Data.Radio.Job.ID)
	if v == nil || v.Job.Code != equipment.RadioCodeUnsupported || v.Reading == nil || v.Reading.Family != equipment.RadioUnsupported {
		t.Fatalf("resultado não explícito %+v", v)
	}
}
func assertStandaloneContextPreserved(t *testing.T, j *ppJourney, id string, saved *equipment.RadioReading,
	before []equipment.RadioLink, login string) {
	t.Helper()
	ctx := context.Background()
	old, profile, c := j.store.RadioCurrent(ctx, j.client.profile, id)
	if c != equipment.OK || old == nil || old.ID != saved.ID || profile.Family != equipment.RadioIntelbrasWOM || !profile.Confirmed {
		t.Fatalf("perfil ou leitura perdidos: %s %+v %+v", c, old, profile)
	}
	after, c := j.store.RadioLinks(ctx, j.client.profile, j.device, login)
	if c != equipment.OK || len(after) != 1 || after[0].CPE != id || after[0].MAC != before[0].MAC {
		t.Fatalf("vínculo alterado: %s %+v", c, after)
	}
}
