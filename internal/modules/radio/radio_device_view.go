package radio

import (
	"context"
	"infradesktop/internal/modules/equipment"
)

func (s *Service) enrichRadioDeviceView(ctx context.Context, q Command, in RadioInput, v *RadioView) Code {
	device := in.DeviceID
	if v.Reading != nil {
		device = v.Reading.DeviceID
	}
	if device == "" || (v.Reading == nil && q.Op != "radio.current") {
		return OK
	}
	lookup, ok := s.Store.(equipment.RadioDeviceLinkStore)
	if !ok {
		v.Reading = readingWithRadioLinks(v.Reading, nil, nil)
		return OK
	}
	links, c := lookup.RadioDeviceLinks(ctx, q.Scope.Profile, device)
	if c != OK {
		return c
	}
	state, c := s.Store.ReadEquipment(ctx, q.Scope.Profile)
	if c != OK {
		return c
	}
	v.Reading = readingWithRadioLinks(v.Reading, links, state.Devices)
	if v.Link == nil {
		v.Link = uniqueCPELink(device, links, state.Devices)
	}
	return OK
}

func readingWithRadioLinks(reading *RadioReading, links []RadioLink, devices []Device) *RadioReading {
	if reading == nil {
		return nil
	}
	copy := *reading
	copy.Clients = append([]RadioClient{}, reading.Clients...)
	for i := range copy.Clients {
		client := &copy.Clients[i]
		client.CPE = ""
		matches := map[string]bool{}
		for _, link := range links {
			if confirmedRadioClientLink(copy.DeviceID, *client, link, devices) {
				matches[link.CPE] = true
			}
		}
		if len(matches) == 1 {
			for id := range matches {
				client.CPE = id
			}
		}
	}
	return &copy
}

func confirmedRadioClientLink(ap string, client RadioClient, link RadioLink, devices []Device) bool {
	mac, valid := RadioMAC(client.MAC)
	bssid, bssidValid := RadioMAC(client.BSSID)
	return valid && bssidValid && link.AP == ap && link.MAC == mac && link.BSSID == bssid &&
		link.Interface == client.Interface && confirmedLocalLink(link) && hasRadioCPE(devices, link.CPE)
}
func confirmedLocalLink(link RadioLink) bool {
	return link.At != "" && (link.Origin == "manual" || link.Origin == "sugerido-confirmado")
}
func hasRadioCPE(devices []Device, id string) bool {
	for _, device := range devices {
		if device.ID == id && device.Role == RoleCPE {
			return true
		}
	}
	return false
}
func uniqueCPELink(device string, links []RadioLink, devices []Device) *RadioLink {
	var found *RadioLink
	for _, link := range links {
		if link.CPE != device || !confirmedLocalLink(link) || !hasRadioCPE(devices, device) {
			continue
		}
		if found != nil {
			return nil
		}
		copy := link
		found = &copy
	}
	return found
}
