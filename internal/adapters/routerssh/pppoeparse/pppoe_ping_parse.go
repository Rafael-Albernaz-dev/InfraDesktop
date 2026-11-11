package pppoeparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"math"
	"strconv"
	"strings"
	"time"
)

// ParsePDRTTInterval accepts explicitly ordered, unique units and exact decimal
// fractions. It preserves the last printed unit's quantization without floats.
func ParsePDRTTInterval(value string) equipment.PPInterval {
	if strings.Contains(value, ":") {
		v := ParsePPDuration(value)
		if v.Known && v.Max <= int64(equipment.QueryBudget) {
			return v
		}
		return equipment.PPInterval{}
	}
	if value == "" || len(value) > 80 {
		return equipment.PPInterval{}
	}
	total, previous, quantum := int64(0), int64(math.MaxInt64), int64(0)
	for value != "" {
		var addition int64
		var ok bool
		value, addition, previous, quantum, ok = consumeRTTUnit(value, previous, total)
		if !ok {
			return equipment.PPInterval{}
		}
		total += addition
	}
	if quantum < 1 || total > int64(equipment.QueryBudget)-quantum {
		return equipment.PPInterval{}
	}
	return equipment.PPInterval{Min: total, Max: total + quantum - 1, Known: true}
}

func consumeRTTUnit(
	value string, previous, currentTotal int64,
) (string, int64, int64, int64, bool) {
	whole, fraction, rest, ok := splitRTTDigits(value)
	if !ok {
		return "", 0, 0, 0, false
	}
	factor, rest := matchRTTUnitSuffix(rest)
	if factor == 0 || factor >= previous {
		return "", 0, 0, 0, false
	}
	quantum := factor
	number, ok := pdNumber(whole, uint64(equipment.QueryBudget))
	if !ok || number > uint64(int64(equipment.QueryBudget)/factor) {
		return "", 0, 0, 0, false
	}
	addition := int64(number) * factor
	if fraction != "" {
		fracAdd, fracQuantum, ok := parseRTTFraction(fraction, factor)
		if !ok {
			return "", 0, 0, 0, false
		}
		addition += fracAdd
		quantum = fracQuantum
	}
	if addition > int64(equipment.QueryBudget)-currentTotal {
		return "", 0, 0, 0, false
	}
	return rest, addition, factor, quantum, true
}

func splitRTTDigits(value string) (string, string, string, bool) {
	n := countLeadingDigits(value)
	if n == 0 {
		return "", "", "", false
	}
	whole := value[:n]
	fraction := ""
	value = value[n:]
	if strings.HasPrefix(value, ".") {
		value = value[1:]
		n = countLeadingDigits(value)
		if n == 0 {
			return "", "", "", false
		}
		fraction = value[:n]
		value = value[n:]
	}
	return whole, fraction, value, true
}

func countLeadingDigits(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

func matchRTTUnitSuffix(value string) (int64, string) {
	units := []struct {
		name   string
		factor int64
	}{
		{"ms", int64(time.Millisecond)},
		{"us", int64(time.Microsecond)},
		{"ns", 1},
		{"s", int64(time.Second)},
	}
	for _, unit := range units {
		if strings.HasPrefix(value, unit.name) {
			return unit.factor, value[len(unit.name):]
		}
	}
	return 0, value
}

func parseRTTFraction(fraction string, factor int64) (int64, int64, bool) {
	quantum := factor
	for range fraction {
		quantum /= 10
	}
	if quantum < 1 {
		return 0, 0, false
	}
	decimal, ok := pdNumber(fraction, uint64(factor))
	if !ok {
		return 0, 0, false
	}
	return int64(decimal) * quantum, quantum, true
}

type pdPingParseState struct {
	count     int
	address   string
	result    *equipment.PDPingResult
	sequences map[int]bool
	summary   map[string]string
	replies   int
	header    bool
	ended     bool
}

var pdPingStatuses = []string{
	"timeout",
	"net unreachable",
	"host unreachable",
	"admin prohibited",
	"TTL exceeded",
	"fragmentation needed and DF set",
	"packet too large and cannot be fragmented",
	"no route to host",
	"echo reply",
}

func ParsePDPing(
	text string, count int, address string,
) (*equipment.PDPingResult, equipment.Code) {
	if len(text) > 8*1024 || count < 1 || count > 5 {
		return nil, equipment.Partial
	}
	text, counts, ok := ppCountPrelude(text, 4)
	if !ok || (len(counts) != 0 &&
		(counts[0] != 0 || counts[1] != 0 || counts[2] != 0 || counts[3] != 1)) {
		return nil, equipment.Partial
	}
	st := &pdPingParseState{
		count:     count,
		address:   address,
		result:    &equipment.PDPingResult{Context: "concentrador · main"},
		sequences: map[int]bool{},
		summary:   map[string]string{},
	}
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !st.consumeLine(line) {
			return nil, equipment.Partial
		}
	}
	if !st.finalize() {
		return nil, equipment.Partial
	}
	return st.result, equipment.OK
}

func (st *pdPingParseState) consumeLine(line string) bool {
	if st.ended || strings.HasPrefix(line, "sent=") {
		st.ended = true
		return st.parseSummaryLine(line)
	}
	if strings.HasPrefix(line, "SEQ ") {
		if st.header || len(st.result.Probes) > 0 {
			return false
		}
		st.header = true
		return true
	}
	return st.parseProbeLine(line)
}

func (st *pdPingParseState) parseSummaryLine(line string) bool {
	for _, pair := range strings.Fields(line) {
		fields := strings.Split(pair, "=")
		if len(fields) != 2 || fields[1] == "" || st.summary[fields[0]] != "" {
			return false
		}
		switch fields[0] {
		case "sent", "received", "packet-loss", "min-rtt", "avg-rtt", "max-rtt":
			st.summary[fields[0]] = fields[1]
		default:
			return false
		}
	}
	return true
}

func (st *pdPingParseState) parseProbeLine(line string) bool {
	fields := strings.Fields(line)
	if !st.header || len(fields) < 2 || len(st.result.Probes) >= st.count {
		return false
	}
	seq, ok := pdNumber(fields[0], uint64(st.count-1))
	if !ok || st.sequences[int(seq)] {
		return false
	}
	st.sequences[int(seq)] = true
	probe := equipment.PDPingProbe{Sequence: int(seq), Status: matchPDPingStatus(line)}
	if len(fields) >= 5 {
		duration := ParsePDRTTInterval(fields[4])
		if duration.Known {
			value := duration.Min
			probe.RTT = &value
		}
	}
	if probe.Status == "" || probe.Status == "echo reply" {
		if !st.validateEchoReplyFields(fields, probe) {
			return false
		}
		st.replies++
	}
	st.result.Probes = append(st.result.Probes, probe)
	return true
}

func matchPDPingStatus(line string) string {
	for _, status := range pdPingStatuses {
		if strings.HasSuffix(line, " "+status) {
			return status
		}
	}
	return ""
}

func (st *pdPingParseState) validateEchoReplyFields(
	fields []string, probe equipment.PDPingProbe,
) bool {
	if len(fields) < 5 || fields[1] != st.address || probe.RTT == nil {
		return false
	}
	if _, ok := pdNumber(fields[2], 65535); !ok {
		return false
	}
	if _, ok := pdNumber(fields[3], 255); !ok {
		return false
	}
	return probe.Status != "" || len(fields) == 5
}

func (st *pdPingParseState) finalize() bool {
	sent, ok := pdNumber(st.summary["sent"], uint64(st.count))
	if !ok || int(sent) != st.count || len(st.result.Probes) != st.count || !st.ended {
		return false
	}
	received, ok := pdNumber(st.summary["received"], uint64(st.count))
	if !ok || int(received) != st.replies {
		return false
	}
	st.result.Sent = int(sent)
	st.result.Received = int(received)
	st.result.Loss = float64(sent-received) * 100 / float64(sent)
	if !validateReportedLoss(st.summary["packet-loss"], st.result.Loss) {
		return false
	}
	return st.finalizeSummaryRTT()
}

func validateReportedLoss(rawLoss string, expected float64) bool {
	loss := strings.TrimSuffix(rawLoss, "%")
	if loss == rawLoss {
		return false
	}
	parts := strings.Split(loss, ".")
	if len(parts) > 2 || !routeros.Decimal(parts[0]) ||
		(len(parts) == 2 && (!routeros.Decimal(parts[1]) || len(parts[1]) > 3)) {
		return false
	}
	reported, err := strconv.ParseFloat(loss, 64)
	if err != nil || reported < 0 || reported > 100 {
		return false
	}
	scale := 1.0
	if len(parts) == 2 {
		scale = math.Pow10(len(parts[1]))
	}
	return reported >= math.Floor(expected*scale)/scale &&
		reported <= math.Ceil(expected*scale)/scale
}

func (st *pdPingParseState) finalizeSummaryRTT() bool {
	if st.replies == 0 {
		return st.summary["min-rtt"] == "" &&
			st.summary["avg-rtt"] == "" &&
			st.summary["max-rtt"] == ""
	}
	values := []*int64{}
	for _, name := range []string{"min-rtt", "avg-rtt", "max-rtt"} {
		duration := ParsePDRTTInterval(st.summary[name])
		if !duration.Known {
			return false
		}
		v := duration.Min
		values = append(values, &v)
	}
	st.result.Min, st.result.Avg, st.result.Max = values[0], values[1], values[2]
	return *st.result.Min <= *st.result.Avg && *st.result.Avg <= *st.result.Max
}
