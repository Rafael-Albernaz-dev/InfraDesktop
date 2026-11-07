package access

import (
	"context"
	"testing"
	"time"
)

func TestInternalActiveProfileNeedsAccountAndRegisteredCompanyWithoutSession(t *testing.T) {
	store := &sessionStore{}
	service := New(store, &sessionCrypto{}, time.Now, 1000, "activation")
	cases := []struct {
		state State
		want  string
	}{
		{State{}, ""},
		{State{Account: &Account{Profile: "p"}}, ""},
		{State{Account: &Account{Profile: "p"}, Companies: []Company{{ID: "other"}}}, ""},
		{State{Account: &Account{Profile: "p"}, Companies: []Company{{ID: "p"}}}, "p"},
	}
	for _, tc := range cases {
		store.state = tc.state
		got, ok := service.ActiveProfile(context.Background())
		if got != tc.want || ok != (tc.want != "") {
			t.Fatalf("got %q %v", got, ok)
		}
	}
}
