package routerssh

import (
	"infradesktop/internal/modules/equipment"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Exact .rsc candidates were executed by the human in6.49.18 and7.14.3 before
// these commands were wired to the runtime. See sanitized public goldens.
const DeviceFastTrackCommand = `:local started [/system resource get uptime]; :put ("T3F\t1\t".[/system resource get version]); :put ("F\t".[:len [/ip firewall filter find where action="fasttrack-connection" and disabled=no]]); :put ("D\t".([/system resource get uptime]-$started)); :put "Z\tcomplete"`
const DevicePortsCommand = ppEscapeScript + `; :local started [/system resource get uptime]; :local cpu [/system resource get cpu-load]; :put ("T3E\t1\t".[/system resource get version]); :local ids [/interface ethernet find]; :local n 0; :if ([:len $ids]>128) do={:error "port-limit"}; :foreach id in=$ids do={:local m [/interface ethernet monitor $id once as-value]; :local downs "?"; :do {:set downs [/interface get $id link-downs]} on-error={}; :put ("E\t".[$esc [/interface ethernet get $id name]]."\t".[$esc ($m->"status")]."\t".[$esc ($m->"rate")]."\t".[$esc ($m->"full-duplex")]."\t".$downs); :set n ($n+1)}; :put ("D\t".([/system resource get uptime]-$started)."\t".$cpu."\t".[/system resource get cpu-load]); :put ("Z\t".$n."\tcomplete")`

func t3Lines(text, header string) ([][]string, equipment.Code) {
	if collectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect
	}
	if len(text) > 64*1024 {
		return nil, equipment.Limit
	}
	text = strings.TrimSuffix(strings.TrimSuffix(text, "\n"), "\r")
	lines := strings.Split(text, "\n")
	out := make([][]string, 0, len(lines))
	for _, line := range lines {
		if len(line) > 2048 {
			return nil, equipment.Partial
		}
		out = append(out, strings.Split(strings.TrimSuffix(line, "\r"), "\t"))
	}
	if len(out) < 3 || len(out[0]) != 3 || out[0][0] != header || out[0][1] != "1" || len(out[0][2]) > 128 || (!strings.HasPrefix(out[0][2], "6.") && !strings.HasPrefix(out[0][2], "7.")) {
		return nil, equipment.Partial
	}
	return out, equipment.OK
}
func parseDeviceFastTrack(text string) (*uint64, equipment.Code) {
	lines, c := t3Lines(text, "T3F")
	if c != equipment.OK {
		return nil, c
	}
	if len(lines) != 4 || len(lines[1]) != 2 || lines[1][0] != "F" || len(lines[2]) != 2 || lines[2][0] != "D" || !ParsePPDuration(lines[2][1]).Known || len(lines[3]) != 2 || lines[3][0] != "Z" || lines[3][1] != "complete" {
		return nil, equipment.Partial
	}
	n := ppCounter(lines[1][1])
	if n == nil {
		return nil, equipment.Partial
	}
	return n, equipment.OK
}

var portRatePattern = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?)(bps|Kbps|Mbps|Gbps|Tbps)$`)

func parsePortRate(raw string) *uint64 {
	f := portRatePattern.FindStringSubmatch(raw)
	if len(f) != 3 {
		return nil
	}
	n, e := strconv.ParseFloat(f[1], 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n <= 0 {
		return nil
	}
	n *= map[string]float64{"bps": 1, "Kbps": 1e3, "Mbps": 1e6, "Gbps": 1e9, "Tbps": 1e12}[f[2]]
	if n > 1e15 || n != math.Trunc(n) {
		return nil
	}
	v := uint64(n)
	return &v
}
func parseDevicePorts(text string) ([]equipment.PPReadRow, equipment.Code) {
	lines, c := t3Lines(text, "T3E")
	if c != equipment.OK {
		return nil, c
	}
	if len(lines) > 131 {
		return nil, equipment.Limit
	}
	footer, duration := lines[len(lines)-1], lines[len(lines)-2]
	if len(footer) != 3 || footer[0] != "Z" || footer[2] != "complete" || len(duration) != 4 || duration[0] != "D" || !ParsePPDuration(duration[1]).Known {
		return nil, equipment.Partial
	}
	for _, cpu := range duration[2:] {
		n := ppCounter(cpu)
		if n == nil || *n > 100 {
			return nil, equipment.Partial
		}
	}
	n := ppCounter(footer[1])
	if n == nil || *n != uint64(len(lines)-3) {
		return nil, equipment.Partial
	}
	rows := []equipment.PPReadRow{}
	seen := map[string]bool{}
	for _, f := range lines[1 : len(lines)-2] {
		if len(f) != 6 || f[0] != "E" {
			return nil, equipment.Partial
		}
		name, ok := ppUnescape(f[1])
		if !ok || name == "" || len(name) > 256 || seen[name] {
			return nil, equipment.Partial
		}
		seen[name] = true
		port := &equipment.DevicePort{}
		if f[5] != "?" {
			if ppCounter(f[5]) == nil {
				return nil, equipment.Partial
			}
			down := f[5]
			port.LinkDowns = &down
		}
		switch f[2] {
		case "link-ok":
			running := true
			port.Running = &running
			port.SpeedBPS = parsePortRate(f[3])
			full, known := pdBool(f[4], false)
			if port.SpeedBPS == nil || !known || full == nil {
				return nil, equipment.Partial
			}
			port.FullDuplex = full
		case "no-link":
			if f[3] != "" || f[4] != "" {
				return nil, equipment.Partial
			}
			running := false
			port.Running = &running
		default:
			return nil, equipment.Partial
		}
		rows = append(rows, equipment.PPReadRow{Kind: "port", Name: name, Port: port})
	}
	return rows, equipment.OK
}
