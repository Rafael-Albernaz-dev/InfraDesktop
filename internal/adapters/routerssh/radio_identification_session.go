package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"strings"
)

// Driver selection requires a positive manufacturer identity.
func (r *radioSession) identifyStandalone() (*equipment.RadioReading, equipment.Code) {
	if strings.HasPrefix(r.serverVersion, "SSH-2.0-ROSSSH") {
		return r.identifyRouterOS()
	}
	reading, identified, code := r.identifyAirOS()
	if identified || code != equipment.OK {
		return reading, code
	}
	return r.identifyWOM()
}

func (r *radioSession) identifyRouterOS() (*equipment.RadioReading, equipment.Code) {
	identityText, _, c := r.run(RadioIdentificationCommand)
	if c != equipment.OK {
		return &equipment.RadioReading{Failure: r.failure}, c
	}
	identity, supported, c := parseRadioIdentification(identityText)
	if c != equipment.OK {
		return nil, c
	}
	if !supported {
		return identity, equipment.RadioCodeUnsupported
	}
	text, seconds, c := r.run(RadioStationCommand)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, c
	}
	reading, c := ParseRadioStationV6(text)
	if c == equipment.RadioCodeUnsupported && radioStandaloneAPMode(text) {
		reading, text, seconds, c = r.readStandaloneAP(identity, text)
	}
	if c != equipment.OK {
		if c == equipment.RadioCodeUnsupported && reading == nil {
			reading = radioStandaloneUnsupported(text)
		}
		return reading, c
	}
	if reading.Version != identity.Version || reading.Model != identity.Model {
		return radioPartial("identity-changed", len(text))
	}
	reading.CommandSeconds = seconds
	reading.Legacy = append([]string{}, r.legacy...)
	reading.Negotiated = append([]string{}, r.negotiated...)
	r.initial = reading
	return reading, equipment.OK
}

func (r *radioSession) identifyAirOS() (*equipment.RadioReading, bool, equipment.Code) {
	const cmd = `if command -v mca-status >/dev/null 2>&1; then mca-status; cat /proc/uptime; else printf "L07-DRIVER-ABSENT\n"; fi`
	text, seconds, c := r.run(cmd)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, false, c
	}
	if !strings.Contains(text, "firmwareVersion=") {
		return nil, false, equipment.OK
	}
	identity, valid := identifyAirOS(text)
	if !valid {
		partial, code := radioPartial("identification", len(text))
		return partial, true, code
	}
	if (!strings.HasPrefix(identity.Version,
		"XM.") &&
		!strings.HasPrefix(identity.Version,
			"XW.")) ||
		!strings.Contains(identity.Version,
			".v6.") {
		return identity, true, equipment.RadioCodeUnsupported
	}
	if identity.Mode != "sta" {
		identity.DriverReason = "Modo airOS sem driver comprovado"
		return identity, true, equipment.RadioCodeUnsupported
	}
	reading, c := ParseRadioAirOS(text)
	if reading != nil {
		reading.CommandSeconds = seconds
		reading.Manufacturer = "Ubiquiti"
		reading.Mode = identity.Mode
		if c == equipment.RadioCodeUnsupported {
			reading.DriverReason = "Firmware airOS sem driver comprovado"
		}
	}
	if c == equipment.OK {
		r.initial = reading
	}
	return reading, true, c

}

func (r *radioSession) readStandaloneAP(identity *equipment.RadioReading, stationText string) (
	*equipment.RadioReading, string, float64, equipment.Code,
) {
	wireless := radioStandaloneWireless(stationText)
	text, seconds, c := r.run(RadioAPCommand)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, text, seconds, c
	}
	reading, c := ParseRadioAP(text)
	if c == equipment.OK && reading != nil {
		if reading.Version != identity.Version {
			partial, code := radioPartial("identity-changed", len(text))
			return partial, text, seconds, code
		}
		reading.Manufacturer, reading.Model = identity.Manufacturer, identity.Model
		reading.Mode, reading.WirelessProtocol = wireless["mode"], wireless["wireless-protocol"]
	}
	return reading, text, seconds, c
}
