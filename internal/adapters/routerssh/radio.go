package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Executed on the real P02 v6 before integration. No user text is interpolated.
const RadioAPCommand = (":put \"L07-CPU-BEFORE\"; /system resource print; :put \"L07-INTERFACES\"; /int" +
	"erface wireless print detail without-paging; :put \"L07-MONITORS\"; :foreach w" +
	" in=[/interface wireless find] do={:put (\"L07-INTERFACE=\" . [/interface wire" +
	"less get $w name]); /interface wireless monitor $w once}; :put \"L07-CPU-DURIN" +
	"G\"; :put (\"cpu-load=\" . [/system resource get cpu-load]); :put \"L07-CLIENT" +
	"S\"; /interface wireless registration-table print stats detail without-paging;" +
	" :put \"L07-CPU-AFTER\"; :put (\"cpu-load=\" . [/system resource get cpu-load]" +
	"); :put \"L07-DONE\"")
const RadioAirOSCommand = `mca-status; cat /proc/uptime`
const RadioWOMCommand = `iw dev; iw dev wlan0 link; iw dev wlan0 station dump; cat /proc/uptime`

var radioAssignment = regexp.MustCompile(`([A-Za-z0-9_-]+)=("(?:\\.|[^"\\])*"|[^\s]+)`)

// RouterOS print detail aligns record indexes in the first three columns.
// Continuations are indented further and can themselves start with a number
// when strength-at-rates wraps between HT40- and its numeric suffix.
var radioRecord = regexp.MustCompile(`^[ \t]{0,2}[0-9]+\s`)
var radioNumber = regexp.MustCompile(`^(-?[0-9]+(?:\.[0-9]+)?)`)
var womSignal = regexp.MustCompile(`(?m)^\s*signal:\s*(-?[0-9]+)(?:\s+dBm)?\s*(?:\[([^\]]+)\])?`)
var womRate = regexp.MustCompile(`(?m)^\s*(tx|rx) bitrate:\s*([0-9.]+) MBit/s`)
var radioColonKey = regexp.MustCompile(`^[a-z][a-z-]*$`)
var radioProcFooter = regexp.MustCompile(`(?:^|\n)[0-9]+(?:\.[0-9]+)? [0-9]+(?:\.[0-9]+)?\s*$`)

func radioRecords(text string) ([]map[string]string, bool) {
	var records []string
	for _, line := range strings.Split(text, "\n") {
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "Flags:") {
			continue
		}
		if radioRecord.MatchString(line) {
			records = append(records, line)
		} else {
			if len(records) == 0 {
				return nil, false
			}
			records[len(records)-1] += " " + strings.TrimSpace(line)
		}
	}
	out := make([]map[string]string, 0, len(records))
	for _, text := range records {
		fields := map[string]string{}
		for _, match := range radioAssignment.FindAllStringSubmatch(text, -1) {
			if _, exists := fields[match[1]]; exists {
				return nil, false
			}
			value := match[2]
			if strings.HasPrefix(value, `"`) {
				var err error
				value, err = strconv.Unquote(value)
				if err != nil {
					return nil, false
				}
			}
			fields[match[1]] = value
		}
		out = append(out, fields)
	}
	return out, true
}
func radioColon(text string) map[string]string {
	fields := map[string]string{}
	key := ""
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		if k, v, found := strings.Cut(trim, ":"); found && radioColonKey.MatchString(k) {
			key = k
			fields[k] = strings.TrimSpace(v)
		} else if key != "" && trim != "" {
			fields[key] += trim
		}
	}
	return fields
}
func radioMetric(raw, name, unit, direction string, divisor float64) (equipment.RadioMetric, bool) {
	m := equipment.RadioMetric{Name: name, Unit: unit, Direction: direction}
	match := radioNumber.FindStringSubmatch(raw)
	if len(match) != 2 {
		return m, false
	}
	n, err := strconv.ParseFloat(match[1], 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return m, false
	}
	m.Value = n / divisor
	if unit == "dBm" && !strings.HasPrefix(name, "power_") && (m.Value < -150 || m.Value > 0) {
		return m, false
	}
	if unit == "dBm" && strings.HasPrefix(name, "power_") && (m.Value < -30 || m.Value > 70) {
		return m, false
	}
	if unit == "%" && (m.Value < 0 || m.Value > 100) {
		return m, false
	}
	if unit != "dBm" && m.Value < 0 {
		return m, false
	}
	return m, true
}
func addRadioMetric(out *[]equipment.RadioMetric, fields map[string]string, key, name, unit, direction string, divisor float64) bool {
	text, exists := fields[key]
	if !exists || text == "" {
		return true
	}
	m, valid := radioMetric(text, name, unit, direction, divisor)
	if !valid {
		return false
	}
	*out = append(*out, m)
	return true
}
func (Adapter) ReadRadio(ctx context.Context,
	d equipment.Device,
	user,
	secret string,
	family equipment.RadioFamily,
	check func() equipment.Code) (*equipment.RadioReading,
	equipment.Code) {
	session, c := (Adapter{}).OpenRadioSession(ctx, d, user, secret, family, nil, check)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer session.Close()
	r, c := session.Initial()
	if c != equipment.OK {
		return r, c
	}
	if d.Role == equipment.RoleAP {
		load, probe := session.Probe()
		if probe == equipment.OK && len(load.PingRTT) > 0 {
			r.PingRTT = &load.PingRTT[len(load.PingRTT)-1]
		}
	}
	return r, c
}

func radioPartial(reason string, size int) (*equipment.RadioReading, equipment.Code) {
	return &equipment.RadioReading{Failure: &equipment.RadioFailure{Reason: reason, Bytes: size}}, equipment.Partial
}
