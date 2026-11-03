package ipc

import (
	"github.com/google/uuid"
	"infradesktop/internal/modules/equipment"
	"strings"
	"testing"
	"time"
)

func prepareUnsupportedRadioLink(t *testing.T, f *unsupportedIPCScenario) equipment.RadioInput {
	t.Helper()
	v := f.finish(t, equipment.RadioInput{DeviceID: f.ap,
		Revision:  2,
		Family:    equipment.RadioRouterOSWirelessV6,
		Mode:      equipment.RadioOnce,
		Operation: uuid.NewString()})
	if v.Job.Code != equipment.OK {
		t.Fatal(v.Job)
	}
	f.call(t, "radio.confirm-family",
		equipment.RadioInput{DeviceID: f.ap,
			Revision: 2,
			Family:   equipment.RadioRouterOSWirelessV6,
			Warning:  15,
			Critical: 20})
	choice := &equipment.RadioLinkChoice{AP: f.ap,
		CPE:       f.cpe,
		MAC:       "02:00:00:00:10:01",
		Interface: "wlan1",
		BSSID:     "02:00:00:00:10:05",
		Evidence:  "synthetic MAC association",
		Origin:    "manual"}
	input := equipment.RadioInput{DeviceID: f.journey.device,
		ObservationID: f.input.ObservationID,
		RowID:         f.input.RowID,
		Revision:      2,
		Mode:          equipment.RadioMeasure,
		Operation:     uuid.NewString()}
	f.call(t, "radio.link-confirm",
		equipment.RadioInput{DeviceID: f.journey.device,
			ObservationID: f.input.ObservationID,
			RowID:         f.input.RowID,
			Operation:     uuid.NewString(),
			Link:          choice})
	return input
}
func assertUnknownCPERequiresIdentity(t *testing.T, f *unsupportedIPCScenario, input equipment.RadioInput) {
	t.Helper()
	unknown := f.finish(t, input)
	if unknown.Job.Code != equipment.RadioCodeFamilyConfirmationRequired {
		t.Errorf("unidentified CPE was classified unsupported: state=%s code=%s", unknown.Job.State, unknown.Job.Code)
	}
}
func identifyUnsupportedCPE(t *testing.T, f *unsupportedIPCScenario) int32 {
	t.Helper()
	cpeBefore := f.cpePeer.Auth.Load()
	identified := f.finish(t, equipment.RadioInput{DeviceID: f.cpe,
		Revision:  2,
		Family:    equipment.RadioAirOSV6,
		Mode:      equipment.RadioOnce,
		Operation: uuid.NewString()})
	saved := f.call(t, "radio.current", equipment.RadioInput{DeviceID: f.cpe})
	if identified.Job.Code != equipment.RadioCodeUnsupported ||
		saved.Reading == nil ||
		saved.Reading.Family != equipment.RadioUnsupported ||
		saved.Reading.Version != "WA.v8.7.0-synthetic" ||
		!saved.Reading.Compatible ||
		saved.Profile == nil ||
		saved.Profile.Family != equipment.RadioUnsupported {
		t.Fatalf("observed incompatibility was not preserved through SSH/store/IPC: job=%+v saved=%+v", identified.Job, saved)
	}
	if f.cpePeer.Auth.Load() != cpeBefore+1 {
		t.Fatal("identification retried")
	}
	return cpeBefore
}
func measureUnsupportedAPOnly(t *testing.T, f *unsupportedIPCScenario, input equipment.RadioInput) equipment.RadioInput {
	t.Helper()
	input.Operation = uuid.NewString()
	measured := f.finish(t, input)
	if measured.Job.Code != equipment.OK ||
		measured.Cost == nil ||
		!measured.Cost.Enabled ||
		!measured.Cost.APOnly ||
		len(measured.Cost.Series) != 10 ||
		!strings.Contains(measured.CPENote,
			"WA.v8.7.0-synthetic") ||
		!strings.Contains(measured.CPENote,
			"dados vistos pelo AP") {
		t.Fatalf("observed unsupported CPE blocked AP-only measurement: job=%+v cost=%+v note=%s", measured.Job, measured.Cost, measured.CPENote)
	}
	return input
}
func assertUnsupportedAPOnlyLive(t *testing.T, f *unsupportedIPCScenario, input equipment.RadioInput, cpeBefore int32) {
	t.Helper()
	input.Mode = equipment.RadioLive
	input.Operation = uuid.NewString()
	live := f.call(t, "radio.start", input)
	until := time.Now().Add(5 * time.Second)
	for time.Now().Before(until) {
		live = f.call(t, "radio.status", equipment.RadioInput{JobID: live.Job.ID})
		if live.Job.State != "RUNNING" {
			t.Fatal("AP-only live aborted", live.Job)
		}
		if live.Cost != nil && len(live.Cost.Series) >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if live.Cost == nil || len(live.Cost.Series) < 3 || live.CPEReading != nil || live.Reading == nil {
		t.Fatal("AP-only live did not publish AP ticks")
	}
	f.call(t, "radio.cancel", equipment.RadioInput{JobID: live.Job.ID})
	if f.cpePeer.Auth.Load() != cpeBefore+1 {
		t.Fatal("measurement/live contacted unsupported CPE")
	}
	t.Logf(("real IPC/SQLite: unknown rejected; unsupported version persisted; ten AP ticks" +
		"; AP-only live >=3 ticks; CPE auth after identification=%d"),
		f.cpePeer.Auth.Load()-cpeBefore-1)
}
