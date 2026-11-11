package routerssh

import (
	"infradesktop/internal/adapters/routerssh/radioparse"
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
	// ignored: identification uses stdout; execution code already includes stderr diagnostics
	identityText, _, c := r.run(radioparse.RadioIdentificationCommand)
	if c != equipment.OK {
		return &equipment.RadioReading{Failure: r.failure}, c
	}
	identity, supported, c := radioparse.ParseRadioIdentification(identityText)
	if c != equipment.OK {
		return nil, c
	}
	if !supported {
		return identity, equipment.RadioCodeUnsupported
	}
	text, seconds, c := r.run(radioparse.RadioStationCommand)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, c
	}
	reading, c := radioparse.ParseRadioStationV6(text)
	if c == equipment.RadioCodeUnsupported && radioparse.RadioStandaloneAPMode(text) {
		reading, text, seconds, c = r.readStandaloneAP(identity, text)
	}
	if c != equipment.OK {
		if c == equipment.RadioCodeUnsupported && reading == nil {
			reading = radioparse.RadioStandaloneUnsupported(text)
		}
		return reading, c
	}
	if reading.Version != identity.Version || reading.Model != identity.Model {
		return radioparse.PartialReading("identity-changed", len(text))
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
	identity, valid := radioparse.IdentifyAirOS(text)
	if !valid {
		partial, code := radioparse.PartialReading("identification", len(text))
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
	reading, c := radioparse.ParseRadioAirOS(text)
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
	wireless := radioparse.RadioStandaloneWireless(stationText)
	text, seconds, c := r.run(radioparse.RadioAPCommand)
	if c != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, text, seconds, c
	}
	reading, c := radioparse.ParseRadioAP(text)
	if c == equipment.OK && reading != nil {
		if reading.Version != identity.Version {
			partial, code := radioparse.PartialReading("identity-changed", len(text))
			return partial, text, seconds, code
		}
		reading.Manufacturer, reading.Model = identity.Manufacturer, identity.Model
		reading.Mode, reading.WirelessProtocol = wireless["mode"], wireless["wireless-protocol"]
	}
	return reading, text, seconds, c
}

func (r *radioSession) identifyWOM() (*equipment.RadioReading, equipment.Code) {
	identity, seconds, code := r.run(radioparse.RadioWOMIdentificationCommand)
	if code != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, code
	}
	if !radioparse.IdentifiedWOMLibrary(identity) {
		return unknownStandaloneRadio(), equipment.RadioCodeUnsupported
	}
	text, seconds, code := r.run(radioparse.RadioWOMCommand)
	if code != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, code
	}
	reading, code := radioparse.ParseRadioWOM(text)
	if reading != nil {
		reading.Manufacturer = "Intelbras"
		reading.CommandSeconds = seconds
		reading.Legacy = append([]string{}, r.legacy...)
		reading.Negotiated = append([]string{}, r.negotiated...)
		if strings.Contains("\n"+text, "\n\t\ttype managed\n") {
			reading.Mode = "managed"
		}
	}
	if code == equipment.OK {
		r.initial = reading
	}
	return reading, code
}

func unknownStandaloneRadio() *equipment.RadioReading {
	return &equipment.RadioReading{Family: equipment.RadioUnsupported,
		DriverReason: "Fabricante e família não identificados", Perspective: "CPE",
		Metrics: []equipment.RadioMetric{}, Interfaces: []equipment.RadioInterface{}, Clients: []equipment.RadioClient{}}
}
