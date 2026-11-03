package routerssh

import (
	"bufio"
	"infradesktop/internal/modules/equipment"
	"math"
	"strconv"
	"strings"
	"time"
)

func ParseDeviceOperational(
	text string, begin, end time.Time,
) (*equipment.DeviceHealth, []equipment.PPReadRow, equipment.Code) {
	h, rows, code, _ := parseDeviceOperational(text, begin, end)
	return h, rows, code
}

type operationalParseState struct {
	begin, end   time.Time
	health       *equipment.DeviceHealth
	rows         []equipment.PPReadRow
	started      bool
	ended        bool
	count        int
	pendingCount *uint64
	seen         map[string]bool
	group        map[string]string
}

func parseDeviceOperational(
	text string, begin, end time.Time,
) (*equipment.DeviceHealth, []equipment.PPReadRow, equipment.Code, string) {
	if collectorSyntaxDiagnostic(text) {
		return nil, nil, equipment.CollectorDefect, "O:syntax"
	}
	if len(text) > equipment.PPWireBytes {
		return nil, nil, equipment.Limit, ""
	}
	st := &operationalParseState{
		begin:  begin,
		end:    end,
		health: &equipment.DeviceHealth{Started: equipment.PPTime(begin), At: equipment.PPTime(end)},
		rows:   []equipment.PPReadRow{},
		seen:   map[string]bool{},
		group:  map[string]string{},
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	for scanner.Scan() {
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		if code, rule := st.consumeLine(raw); code != equipment.OK {
			return nil, nil, code, rule
		}
	}
	return st.finalize(scanner.Err())
}

func (st *operationalParseState) consumeLine(raw string) (equipment.Code, string) {
	if st.ended {
		return equipment.Partial, "Z:trailing"
	}
	f := strings.Split(raw, "\t")
	if st.started && len(f) == 1 && ppCounter(raw) != nil {
		if st.pendingCount != nil {
			return equipment.Partial, "ROW:fields"
		}
		st.pendingCount = ppCounter(raw)
		return equipment.OK, ""
	}
	if !st.started {
		return st.parseHeader(f)
	}
	if f[0] == "Z" {
		return st.parseFooter(f)
	}
	return st.parseRecord(f)
}

func (st *operationalParseState) parseHeader(f []string) (equipment.Code, string) {
	if len(f) != 4 || f[0] != "O" || f[1] != "1" ||
		(!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
		return equipment.Partial, "O:header"
	}
	st.health.Version = f[2]
	st.health.Uptime = ParsePPDuration(f[3])
	st.started = true
	return equipment.OK, ""
}

func (st *operationalParseState) parseFooter(f []string) (equipment.Code, string) {
	if len(f) != 4 {
		return equipment.Partial, "Z:fields"
	}
	n, err := strconv.Atoi(f[1])
	if f[2] == "rows" || f[2] == "wire_bytes" {
		return equipment.Limit, ""
	}
	final := ParsePPDuration(f[3])
	if err != nil || n != st.count || f[2] != "complete" || st.pendingCount != nil ||
		!st.health.Uptime.Known || !final.Known || final.Max < st.health.Uptime.Min {
		return equipment.Partial, "Z:footer"
	}
	st.ended = true
	return equipment.OK, ""
}

func (st *operationalParseState) parseRecord(f []string) (equipment.Code, string) {
	for i := 1; i < len(f); i++ {
		v, ok := ppUnescape(f[i])
		if !ok || len(v) > 256 {
			return equipment.Partial, "wire:codec"
		}
		f[i] = v
	}
	if len(f) < 3 {
		return equipment.Partial, "ROW:fields"
	}
	key := f[0] + "/" + f[1]
	if st.seen[key] {
		return equipment.Partial, "ROW:duplicate"
	}
	st.seen[key] = true
	code, rule := st.dispatchRecord(f)
	if code != equipment.OK {
		return code, rule
	}
	if st.count > equipment.PPMaxRows {
		return equipment.Limit, ""
	}
	return equipment.OK, ""
}

func (st *operationalParseState) dispatchRecord(f []string) (equipment.Code, string) {
	switch f[0] {
	case "R":
		return st.parseResourceField(f)
	case "C":
		return st.parseClockField(f)
	case "G":
		return st.parseGroupField(f)
	case "S":
		return st.parseServiceField(f)
	case "K":
		return st.parsePackageField(f)
	case "I":
		return st.parseInterfaceField(f)
	case "P":
		return st.parsePoolField(f)
	default:
		return equipment.Partial, "ROW:type"
	}
}

func (st *operationalParseState) parseResourceField(f []string) (equipment.Code, string) {
	if len(f) != 3 {
		return equipment.Partial, "R:fields"
	}
	switch f[1] {
	case "cpu-load":
		st.health.CPU = deviceNumber(f[2])
		if st.health.CPU != nil && *st.health.CPU > 100 {
			st.health.CPU = nil
		}
	case "free-memory":
		st.health.FreeMemory = deviceNumber(f[2])
	case "total-memory":
		st.health.TotalMemory = deviceNumber(f[2])
	case "temperature":
		st.health.Temperature = parseDeviceTemperature(f[2])
	case "error-count":
		return st.parseErrorCount(f[2])
	case "uptime", "version":
	default:
		return equipment.Partial, "R:fields"
	}
	return equipment.OK, ""
}

func parseDeviceTemperature(raw string) *float64 {
	v := deviceOptional(raw)
	if v == nil {
		return nil
	}
	n, err := strconv.ParseFloat(*v, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < -100 || n > 200 {
		return nil
	}
	return &n
}

func (st *operationalParseState) parseErrorCount(raw string) (equipment.Code, string) {
	st.health.ErrorCount = deviceNumber(raw)
	if st.pendingCount != nil {
		if st.health.ErrorCount == nil || *st.health.ErrorCount != *st.pendingCount {
			return equipment.Partial, "R:echo-mismatch"
		}
		st.pendingCount = nil
	}
	return equipment.OK, ""
}

func (st *operationalParseState) parseClockField(f []string) (equipment.Code, string) {
	if len(f) != 6 {
		return equipment.Partial, "C:fields"
	}
	st.health.ClockOffset = deviceClock(f[1], f[2], f[3], st.begin, st.end)
	st.health.NTPEnabled = deviceBool(f[4])
	if v := deviceOptional(f[5]); v != nil {
		switch *v {
		case "synchronized", "stopped", "waiting", "using-local-clock":
			st.health.NTPStatus = *v
		}
	}
	return equipment.OK, ""
}

func (st *operationalParseState) parseGroupField(f []string) (equipment.Code, string) {
	if len(f) != 3 || (f[2] != "complete" && f[2] != "unavailable" && f[2] != "limited") {
		return equipment.Partial, "G:fields"
	}
	switch f[1] {
	case "services", "packages", "interfaces", "pools":
		st.group[f[1]] = f[2]
		return equipment.OK, ""
	default:
		return equipment.Partial, "G:fields"
	}
}

func (st *operationalParseState) parseServiceField(f []string) (equipment.Code, string) {
	if len(f) != 4 {
		return equipment.Partial, "S:fields"
	}
	switch f[1] {
	case "telnet", "ftp", "www":
	default:
		return equipment.Partial, "S:fields"
	}
	b, ok := pdBool(f[3], false)
	if !ok {
		b = nil
	}
	st.rows = append(st.rows, equipment.PPReadRow{
		Kind:     "service",
		Name:     f[1],
		Used:     ppCounter(f[2]),
		Disabled: b,
	})
	st.count++
	return equipment.OK, ""
}

func (st *operationalParseState) parsePackageField(f []string) (equipment.Code, string) {
	if len(f) != 4 {
		return equipment.Partial, "K:fields"
	}
	b, ok := pdBool(f[3], true)
	if !ok {
		b = nil
	}
	st.rows = append(st.rows, equipment.PPReadRow{
		Kind:     "package",
		Name:     f[1],
		Value:    f[2],
		Disabled: b,
	})
	st.count++
	return equipment.OK, ""
}

func (st *operationalParseState) parseInterfaceField(f []string) (equipment.Code, string) {
	if len(f) != 6 {
		return equipment.Partial, "I:fields"
	}
	st.rows = append(st.rows, equipment.PPReadRow{
		Kind:      "interface",
		Name:      f[1],
		Value:     f[2],
		RXErrors:  ppCounterString(f[3]),
		TXErrors:  ppCounterString(f[4]),
		LinkDowns: ppCounterString(f[5]),
	})
	st.count++
	return equipment.OK, ""
}

func (st *operationalParseState) parsePoolField(f []string) (equipment.Code, string) {
	if len(f) != 4 {
		return equipment.Partial, "P:fields"
	}
	used, size := ppCounter(f[3]), PPPoolSize(f[2])
	if st.pendingCount != nil {
		if used == nil || *used != *st.pendingCount {
			return equipment.Partial, "P:echo-mismatch"
		}
		st.pendingCount = nil
	}
	if used != nil && size != nil && *used > *size {
		used = nil
	}
	st.rows = append(st.rows, equipment.PPReadRow{
		Kind: "pool",
		Name: f[1],
		Used: used,
		Size: size,
	})
	st.count++
	return equipment.OK, ""
}

func (st *operationalParseState) finalize(
	scanErr error,
) (*equipment.DeviceHealth, []equipment.PPReadRow, equipment.Code, string) {
	if scanErr != nil {
		return nil, nil, equipment.Partial, "wire:line-limit"
	}
	if !st.started || !st.ended {
		return nil, nil, equipment.Partial, "Z:missing"
	}
	if st.health.FreeMemory != nil && st.health.TotalMemory != nil &&
		*st.health.FreeMemory > *st.health.TotalMemory {
		st.health.FreeMemory = nil
	}
	clean := []equipment.PPReadRow{{Kind: "resource", Health: st.health}}
	groupKind := map[string]string{
		"service":   "services",
		"package":   "packages",
		"interface": "interfaces",
		"pool":      "pools",
	}
	for _, r := range st.rows {
		if st.group[groupKind[r.Kind]] == "complete" {
			clean = append(clean, r)
		}
	}
	for _, name := range []string{"services", "packages", "interfaces", "pools"} {
		status := st.group[name]
		if status == "" {
			status = "unavailable"
		}
		clean = append(clean, equipment.PPReadRow{
			Kind:  "availability",
			Name:  name,
			Value: status,
		})
	}
	return st.health, clean, equipment.OK, ""
}
