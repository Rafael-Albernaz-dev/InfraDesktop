package radio

import (
	"context"
	"testing"
)

func (radioLiveStore) UpdateEquipment(context.Context, string, func(*State) Code) Code { return OK }

func TestRefusedRadioJobPersistsDeviceAttempt(t *testing.T) {
	baseRemote := &radioLiveRemote{}
	s, q, _, base := radioHarness(t, baseRemote)
	d := base.devices[base.link.CPE]
	d.CredentialID = radioLiveUUID()
	store := &avulsoStore{radioLiveStore: base, state: State{Devices: []Device{d}}}
	s.Store = store
	s.Remote = &avulsoFailureRemote{radioLiveRemote: baseRemote, code: RadioCodeRemoteAuthFailed}
	in := RadioInput{DeviceID: d.ID, Revision: d.Revision, Operation: radioLiveUUID(), Mode: RadioOnce, Family: RadioAirOSV6}
	v, c := s.HandleRadio(context.Background(), q, in)
	if c != OK {
		t.Fatal(c)
	}
	s.WG.Wait()
	got := store.state.Devices[0].LastAttempt
	if got == nil || got.JobID != v.Job.ID || got.Code != RadioCodeRemoteAuthFailed || got.State != "FAILED" || got.At == "" {
		t.Fatalf("attempt not durable: %+v", got)
	}
}

func TestRadioAttemptOutcome(t *testing.T) {
	for _, tc := range []struct {
		name          string
		overall, side Code
		single, known bool
		want          Code
	}{
		{"refused alone", RadioCodeRemoteAuthFailed, RadioCodeRemoteAuthFailed, true, true, RadioCodeRemoteAuthFailed},
		{"success replaces refusal", OK, OK, true, true, OK},
		{"window end proves connection", RadioCodeWindowEnded, OK, false, true, OK},
		{"cancelled says nothing", Cancelled, RadioCodeRemoteAuthFailed, true, false, ""},
		{"untouched side keeps attempt", RadioCodeRemoteAuthFailed, OK, false, false, ""},
		{"own side failure wins", OK, RadioCodeConnectFailed, false, true, RadioCodeConnectFailed},
		{"no driver is not a failed login", OK, RadioCodeUnsupported, false, true, OK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, known := radioAttemptOutcome(tc.overall, tc.side, tc.single)
			if got != tc.want || known != tc.known {
				t.Fatal(got, known)
			}
		})
	}
}
