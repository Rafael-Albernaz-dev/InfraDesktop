package radio

import (
	"context"
	"infradesktop/internal/modules/access"
	"testing"
	"time"
)

type radioStoreWithoutPPTargets struct {
	Store
	RadioStore
}

func (radioStoreWithoutPPTargets) RadioCurrent(context.Context, string, string) (*RadioReading, *RadioProfile, Code) {
	return nil, nil, OK
}

func TestRadioFamilyConfirmationWithoutTargetPortReturnsUnavailable(t *testing.T) {
	service := NewWithInventory(radioLiveAuth{}, radioStoreWithoutPPTargets{}, radioLiveVault{}, &radioLiveRemote{}, radioLiveIDs{}, time.Now)
	defer service.Close()
	_,
		code := service.HandleRadio(context.Background(),
		Command{Op: "radio.confirm-family",
			Scope: access.Command{Profile: "synthetic",
				ExpectedProfile:    "synthetic",
				Activation:         "active",
				ExpectedActivation: "active"}},
		RadioInput{DeviceID: radioLiveUUID()})
	if code != Unavailable {
		t.Fatalf("missing target capability returned %s", code)
	}
}
