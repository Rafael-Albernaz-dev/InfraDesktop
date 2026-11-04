package radio

import (
	"context"
	"testing"
)

func TestRadioClientIDRequiresConfirmedMACInterfaceAndBSSID(t *testing.T) {
	ap, cpe := radioLiveUUID(), radioLiveUUID()
	client := RadioClient{Name: "same observed name", MAC: "02:00:00:00:00:01", Interface: "wlan1",
		BSSID: "02:00:00:00:00:02"}
	link := RadioLink{AP: ap, CPE: cpe, MAC: client.MAC, Interface: client.Interface, BSSID: client.BSSID,
		Origin: "manual", At: "2026-11-04T12:00:00Z"}
	devices := []Device{{ID: cpe, Role: RoleCPE}}
	for _, tc := range []struct {
		name string
		edit func(*RadioLink)
		want string
	}{
		{"confirmed", func(*RadioLink) {}, cpe},
		{"same name different MAC", func(l *RadioLink) { l.MAC = "02:00:00:00:00:99" }, ""},
		{"other interface", func(l *RadioLink) { l.Interface = "wlan2" }, ""},
		{"other BSSID", func(l *RadioLink) { l.BSSID = "02:00:00:00:00:99" }, ""},
		{"unconfirmed", func(l *RadioLink) { l.At = "" }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := link
			tc.edit(&candidate)
			reading := &RadioReading{DeviceID: ap, Clients: []RadioClient{client}}
			got := readingWithRadioLinks(reading, []RadioLink{candidate}, devices)
			if got.Clients[0].CPE != tc.want {
				t.Fatal("CPE inferred without exact identity", got.Clients[0])
			}
			if reading.Clients[0].CPE != "" {
				t.Fatal("presentation mutated stored observation")
			}
		})
	}
}

func TestRadioClientWithoutLinkAndAmbiguousLinkStayUnbound(t *testing.T) {
	client := RadioClient{MAC: "02:00:00:00:00:01", Interface: "wlan1", BSSID: "02:00:00:00:00:02", CPE: "untrusted"}
	reading := &RadioReading{DeviceID: radioLiveUUID(), Clients: []RadioClient{client}}
	if got := readingWithRadioLinks(reading, nil, nil); got.Clients[0].CPE != "" {
		t.Fatal("unverified CPE preserved")
	}
	cpe1, cpe2 := radioLiveUUID(), radioLiveUUID()
	link := RadioLink{AP: reading.DeviceID, CPE: cpe1, MAC: client.MAC, Interface: client.Interface, BSSID: client.BSSID,
		Origin: "manual", At: "2026-11-04T12:00:00Z"}
	other := link
	other.CPE = cpe2
	devices := []Device{{ID: cpe1, Role: RoleCPE}, {ID: cpe2, Role: RoleCPE}}
	got := readingWithRadioLinks(reading, []RadioLink{link, other}, devices)
	if got.Clients[0].CPE != "" {
		t.Fatal("ambiguous CPE chosen")
	}
}

type radioDeviceLinkFixture struct {
	radioLiveStore
	links   []RadioLink
	reading *RadioReading
}

func (s radioDeviceLinkFixture) RadioDeviceLinks(context.Context, string, string) ([]RadioLink, Code) {
	return s.links, OK
}
func (s radioDeviceLinkFixture) RadioCurrent(context.Context, string, string) (*RadioReading, *RadioProfile, Code) {
	return s.reading, nil, OK
}

func TestRadioCurrentProjectsConfirmedInventoryLink(t *testing.T) {
	service, q, _, store := radioHarness(t, &radioLiveRemote{})
	link := store.link
	link.Concentrator = store.conc
	link.Origin = "manual"
	link.At = "2026-11-04T12:00:00Z"
	service.Store = radioDeviceLinkFixture{radioLiveStore: store, links: []RadioLink{link},
		reading: &RadioReading{DeviceID: link.AP, Clients: []RadioClient{{MAC: link.MAC, Interface: link.Interface, BSSID: link.BSSID}}}}
	q.Op = "radio.current"
	view, c := service.HandleRadio(context.Background(), q, RadioInput{DeviceID: link.AP})
	if c != OK || view.Reading.Clients[0].CPE != link.CPE {
		t.Fatal("current lost verified CPE", c, view)
	}
	fixture := service.Store.(radioDeviceLinkFixture)
	fixture.reading = &RadioReading{DeviceID: link.CPE}
	service.Store = fixture
	view, c = service.HandleRadio(context.Background(), q, RadioInput{DeviceID: link.CPE})
	if c != OK || view.Link == nil || view.Link.Concentrator != store.conc {
		t.Fatal("CPE lost local PPPoE association", c, view)
	}
}

func (s radioDeviceLinkFixture) ReadEquipment(context.Context, string) (State, Code) {
	state := State{}
	for _, device := range s.devices {
		state.Devices = append(state.Devices, device)
	}
	return state, OK
}
