package pppoeparse

import (
	"bufio"
	"encoding/json"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"math"
	"net/netip"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// ParsePPDuration preserves quantization and rejects overflow/unknown formats.
func ParsePPDuration(s string) equipment.PPInterval {
	if s == "" || len(s) > 128 {
		return equipment.PPInterval{}
	}
	if strings.Contains(s, ":") {
		return ppClockDuration(s)
	}
	total := int64(0)
	quantum := int64(0)
	previousFactor := int64(math.MaxInt64)
	digits := ""
	for _, c := range s {
		if c >= '0' && c <= '9' {
			digits += string(c)
			if len(digits) > 18 {
				return equipment.PPInterval{}
			}
			continue
		}
		var ok bool
		total, quantum, previousFactor, ok = accumulatePPDurationUnit(
			digits, c, total, previousFactor,
		)
		if !ok {
			return equipment.PPInterval{}
		}
		digits = ""
	}
	if digits != "" || quantum == 0 || total > math.MaxInt64-quantum {
		return equipment.PPInterval{}
	}
	return equipment.PPInterval{Min: total, Max: total + quantum - 1, Known: true}
}

func accumulatePPDurationUnit(
	digits string, c rune, total, previousFactor int64,
) (int64, int64, int64, bool) {
	factor := ppUnitFactor(c)
	if factor == 0 || factor >= previousFactor {
		return 0, 0, 0, false
	}
	v, e := strconv.ParseInt(digits, 10, 64)
	if e != nil || v < 0 || v > (math.MaxInt64-total-factor)/factor {
		return 0, 0, 0, false
	}
	return total + v*factor, factor, factor, true
}

func ppUnitFactor(c rune) int64 {
	switch c {
	case 'w':
		return int64(7 * 24 * time.Hour)
	case 'd':
		return int64(24 * time.Hour)
	case 'h':
		return int64(time.Hour)
	case 'm':
		return int64(time.Minute)
	case 's':
		return int64(time.Second)
	default:
		return 0
	}
}

type pppoeParseState struct {
	obs      *equipment.PPObservation
	counts   []uint64
	line     int
	ended    bool
	rowBytes int
	seen     map[string]bool
}

func ParsePPPoE(text string) (*equipment.PPObservation, equipment.Code) {
	o := &equipment.PPObservation{
		Scope: equipment.PPScope,
		Quality: equipment.PPQuality{
			Status:    "partial",
			Code:      equipment.Partial,
			WireBytes: len(text),
		},
	}
	st := &pppoeParseState{obs: o, seen: map[string]bool{}}
	var ok bool
	text, st.counts, ok = ppCountPrelude(text, 1)
	if !ok {
		return st.fail("codec")
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	for scanner.Scan() {
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		if done, reason := st.consumeLine(raw); done {
			if reason != "" {
				return st.fail(reason)
			}
			return st.obs, equipment.OK
		}
	}
	if scanner.Err() != nil || !st.ended {
		return st.fail("codec")
	}
	if st.obs.Quality.Status == "complete" {
		st.obs.Count = len(st.obs.Rows)
	}
	return st.obs, equipment.OK
}

func (st *pppoeParseState) fail(reason string) (*equipment.PPObservation, equipment.Code) {
	st.obs.Quality.Returned = len(st.obs.Rows)
	st.obs.Rows = nil
	st.obs.Quality.Status = "partial"
	st.obs.Quality.Code = equipment.Partial
	st.obs.Quality.Reason = reason
	return st.obs, equipment.OK
}

func (st *pppoeParseState) consumeLine(raw string) (bool, string) {
	st.line++
	f := strings.Split(raw, "\t")
	if st.ended {
		return true, "codec"
	}
	if st.line == 1 {
		return st.parseHeader(f)
	}
	if (len(f) == 3 || len(f) == 4) && f[0] == "Z" {
		return st.parseFooter(f)
	}
	return st.parseRow(raw, f)
}

func (st *pppoeParseState) parseHeader(f []string) (bool, string) {
	if len(f) != 5 || f[0] != "H" || f[1] != "1" {
		return true, "codec"
	}
	n, e := strconv.Atoi(f[2])
	if e != nil || n < 0 || n > 1000000 || (len(st.counts) != 0 && st.counts[0] != uint64(n)) {
		return true, "codec"
	}
	st.obs.Quality.CountsKnown = true
	st.obs.Quality.Eligible = n
	st.obs.Provenance.Projection = "active-v1"
	st.obs.Provenance.Codec = "pppoe-fields-v1"
	st.obs.Provenance.Capabilities = "base-fields-v1"
	st.obs.Provenance.Version = f[3]
	if !validPPHeaderVersion(f[3]) {
		return true, "codec"
	}
	if strings.HasPrefix(f[3], "6.") {
		st.obs.Provenance.Family = equipment.RouterFamilyV6
	} else if strings.HasPrefix(f[3], "7.") {
		st.obs.Provenance.Family = equipment.RouterFamilyV7
	} else {
		st.obs.Quality.Status = string(equipment.RadioUnsupported)
		st.obs.Quality.Code = equipment.RadioCodeUnsupported
		return true, ""
	}
	st.obs.Provenance.RouterUptime = ParsePPDuration(f[4])
	return false, ""
}

func validPPHeaderVersion(v string) bool {
	if len(v) > 128 || !utf8.ValidString(v) {
		return false
	}
	return strings.IndexFunc(v, func(r rune) bool { return r < 32 || r == 127 }) < 0
}

func (st *pppoeParseState) parseFooter(f []string) (bool, string) {
	n, e := strconv.Atoi(f[1])
	if e != nil || n != len(st.obs.Rows) {
		return true, "codec"
	}
	st.ended = true
	st.obs.Quality.Terminated = true
	st.obs.Quality.Returned = n
	switch f[2] {
	case "complete":
		return st.parseCompleteFooter(f)
	case "rows", "wire_bytes":
		st.obs.Quality.Status = "limited"
		st.obs.Quality.Code = equipment.Limit
		st.obs.Quality.Reason = f[2]
		st.obs.Rows = nil
		return false, ""
	case "churn", "reboot", "item_error", "item_changed":
		return true, "churn"
	default:
		return true, "codec"
	}
}

func (st *pppoeParseState) parseCompleteFooter(f []string) (bool, string) {
	if len(f) != 4 {
		return true, "codec"
	}
	st.obs.Provenance.RouterFinalUptime = ParsePPDuration(f[3])
	if !st.obs.Provenance.RouterFinalUptime.Known {
		return true, "codec"
	}
	if st.obs.Quality.Eligible > equipment.PPMaxRows {
		return true, "rows"
	}
	if !st.obs.Provenance.RouterUptime.Known ||
		st.obs.Provenance.RouterFinalUptime.Max < st.obs.Provenance.RouterUptime.Min {
		return true, "reboot"
	}
	st.obs.Quality.Status = "complete"
	st.obs.Quality.Code = equipment.OK
	st.obs.Quality.MembershipChecked = true
	return false, ""
}

func (st *pppoeParseState) parseRow(raw string, f []string) (bool, string) {
	if len(f) != 7 || f[0] != "R" ||
		len(st.obs.Rows) >= equipment.PPMaxRows || st.obs.Quality.Eligible > equipment.PPMaxRows {
		return true, "rows"
	}
	if reason := unescapePPRowFields(f); reason != "" {
		return true, reason
	}
	r := equipment.PPRow{
		RemoteID:  f[1],
		SessionID: f[2],
		Login:     f[3],
		Address:   f[4],
		Caller:    f[5],
		Uptime:    ParsePPDuration(f[6]),
	}
	if r.RemoteID == "" || st.seen[r.RemoteID] {
		return true, "churn"
	}
	st.seen[r.RemoteID] = true
	if r.Address != "" {
		ip, ok := normalizePPRowAddress(r.Address)
		if !ok {
			return true, "codec"
		}
		r.Address = ip
	}
	b, _ := json.Marshal(r)
	if len(b)+200 > equipment.PPRowBytes {
		return true, "row_bytes"
	}
	st.rowBytes += len(raw) + 2
	if st.rowBytes > equipment.PPWireBytes {
		return true, "wire_bytes"
	}
	st.obs.Rows = append(st.obs.Rows, r)
	return false, ""
}

func unescapePPRowFields(f []string) string {
	limits := []int{32, 64, 256, 45, 128, 64}
	for i := 1; i < len(f); i++ {
		v, ok := routeros.Unescape(f[i])
		if !ok {
			return "codec"
		}
		if len(v) > limits[i-1] {
			return "row_bytes"
		}
		f[i] = v
	}
	return ""
}

func normalizePPRowAddress(addr string) (string, bool) {
	ip, e := netip.ParseAddr(addr)
	if e != nil || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() {
		return "", false
	}
	return ip.String(), true
}

func ppCountPrelude(text string, count int) (string, []uint64, bool) {
	first, _, _ := strings.Cut(text, "\n")
	if !routeros.Decimal(strings.TrimSuffix(first, "\r")) {
		return text, nil, true
	}
	values := make([]uint64, 0, count)
	for range count {
		line, rest, found := strings.Cut(text, "\n")
		value, ok := pdNumber(strings.TrimSuffix(line, "\r"), 1000000)
		if !found || !ok {
			return "", nil, false
		}
		values = append(values, value)
		text = rest
	}
	return text, values, true
}

func ppClockDuration(s string) equipment.PPInterval {
	total, restStr, ok := parsePPDayWeekPrefix(s)
	if !ok {
		return equipment.PPInterval{}
	}
	parts := strings.Split(restStr, ":")
	if len(parts) != 3 || !routeros.Decimal(parts[0]) || !routeros.Decimal(parts[1]) {
		return equipment.PPInterval{}
	}
	h, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || h < 0 || h > (math.MaxInt64-total)/int64(time.Hour) {
		return equipment.PPInterval{}
	}
	total += h * int64(time.Hour)
	m, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || m < 0 || m > 59 {
		return equipment.PPInterval{}
	}
	rest, quantum, ok := parsePPClockSeconds(parts[2], m)
	if !ok || total > math.MaxInt64-rest-quantum {
		return equipment.PPInterval{}
	}
	total += rest
	return equipment.PPInterval{Min: total, Max: total + quantum - 1, Known: true}
}

func parsePPDayWeekPrefix(s string) (int64, string, bool) {
	total := int64(0)
	units := []struct {
		suffix string
		factor int64
	}{
		{"w", int64(7 * 24 * time.Hour)},
		{"d", int64(24 * time.Hour)},
	}
	for _, unit := range units {
		if at := strings.Index(s, unit.suffix); at >= 0 {
			if !routeros.Decimal(s[:at]) {
				return 0, "", false
			}
			v, e := strconv.ParseInt(s[:at], 10, 64)
			if e != nil || v < 0 || v > (math.MaxInt64-total)/unit.factor {
				return 0, "", false
			}
			total += v * unit.factor
			s = s[at+1:]
		}
	}
	return total, s, true
}

func parsePPClockSeconds(secPart string, minutes int64) (int64, int64, bool) {
	seconds := strings.Split(secPart, ".")
	if len(seconds) > 2 || !routeros.Decimal(seconds[0]) {
		return 0, 0, false
	}
	sec, e := strconv.ParseInt(seconds[0], 10, 64)
	if e != nil || sec < 0 || sec > 59 {
		return 0, 0, false
	}
	rest := minutes*int64(time.Minute) + sec*int64(time.Second)
	quantum := int64(time.Second)
	if len(seconds) == 2 {
		f := seconds[1]
		if len(f) > 9 || !routeros.Decimal(f) {
			return 0, 0, false
		}
		v, e := strconv.ParseInt(f, 10, 64)
		if e != nil || v < 0 {
			return 0, 0, false
		}
		quantum = 1
		for i := len(f); i < 9; i++ {
			v *= 10
			quantum *= 10
		}
		rest += v
	}
	return rest, quantum, true
}
