package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"strings"
)

// Only a bounded fixed token is returned from the observed firmware library.
// Whole-word matching refuses prefix lookalikes; this is a software-family
// fingerprint, not evidence of the currently active model or firmware version.
const RadioWOMIdentificationCommand = `if test -f /lib/libfirmware.so; then ` +
	`printf 'L07-LIBFIRMWARE-EXISTS\nEXISTE\nL07-LIBFIRMWARE-VENDOR-TOKENS\n'; ` +
	`grep -owF -e WOM5A23 /lib/libfirmware.so; result=$?; printf '\nL07-GREP-EXIT=%s\n' "$result"; ` +
	`else printf 'L07-LIBFIRMWARE-EXISTS\nNAO\n'; fi`

func identifiedWOMLibrary(text string) bool {
	var lines []string
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 5 || lines[0] != "L07-LIBFIRMWARE-EXISTS" || lines[1] != "EXISTE" {
		return false
	}
	if lines[2] != "L07-LIBFIRMWARE-VENDOR-TOKENS" {
		return false
	}
	if lines[len(lines)-1] != "L07-GREP-EXIT=0" {
		return false
	}
	for _, line := range lines[3 : len(lines)-1] {
		if line != "WOM5A23" {
			return false
		}
	}
	return true
}

func (r *radioSession) identifyWOM() (*equipment.RadioReading, equipment.Code) {
	identity, seconds, code := r.run(RadioWOMIdentificationCommand)
	if code != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, code
	}
	if !identifiedWOMLibrary(identity) {
		return unknownStandaloneRadio(), equipment.RadioCodeUnsupported
	}
	text, seconds, code := r.run(RadioWOMCommand)
	if code != equipment.OK {
		return &equipment.RadioReading{CommandSeconds: seconds, Failure: r.failure}, code
	}
	reading, code := ParseRadioWOM(text)
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
