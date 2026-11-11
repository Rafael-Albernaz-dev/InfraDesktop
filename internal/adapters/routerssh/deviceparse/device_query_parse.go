package deviceparse

import (
	"encoding/json"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"strconv"
	"strings"
)

func ParseSummary(identity, resource string) (*equipment.Observation, equipment.Code) {
	o := &equipment.Observation{Interfaces: []equipment.InterfaceReading{}}
	ids := routeros.Lines(identity)
	if len(ids) != 1 || !routeros.Clean(ids[0], 128) {
		return nil, equipment.Partial
	}
	o.Identity = ids[0]
	fields, ok := extractSummaryResourceFields(resource)
	if !ok {
		return nil, equipment.Partial
	}
	return populateSummaryObservation(o, fields)
}
func ParseInterfaces(o *equipment.Observation, interfaces string) equipment.Code {
	ls := routeros.Lines(interfaces)
	if len(ls) >= 4 && routeros.Decimal(ls[1]) {
		// The embedded dynamic count can be echoed before its labeled value.
		// It must match exactly; this is not a general skip-unknown-lines rule.
		if ls[2] != "dynamic="+ls[1] {
			return equipment.Partial
		}
		ls = append([]string{ls[0]}, ls[2:]...)
	}
	if len(ls) < 3 {
		return equipment.Partial
	}
	counts, code := parseInterfaceCounts(ls)
	if code != equipment.OK {
		return code
	}
	ls = ls[2:]
	if ls[len(ls)-1] != "done" || (len(ls)-1)%5 != 0 {
		return equipment.Partial
	}
	count := (len(ls) - 1) / 5
	expected := counts[0]
	if expected > 128 {
		expected = 128
	}
	if count != expected {
		return equipment.Partial
	}
	part := &equipment.InterfacePart{CountsKnown: true,
		Status: "complete",
		Code:   equipment.OK,
		Total: counts[0] +
			counts[1],
		Eligible: counts[0],
		Dynamic:  counts[1]}
	if counts[0] > count {
		part.Status = "limited"
	}
	if code := appendInterfaceReadings(o, ls); code != equipment.OK {
		return code
	}
	// JSON escaping also has a budget; never let interface bytes discard summary at persistence/IPC.
	for len(o.Interfaces) > 0 {
		// ignored: interface projection has only strings and bools and no custom JSON encoder
		encoded, _ := json.Marshal(o.Interfaces)
		if len(encoded) <= 32*1024 {
			break
		}
		o.Interfaces = o.Interfaces[:len(o.Interfaces)-1]
		part.Status = "limited"
	}
	o.InterfacePart = part
	return equipment.OK
}

func parseInterfaceCounts(ls []string) ([]int, equipment.Code) {
	counts := []int{}
	for n, k := range []string{"eligible=", "dynamic="} {
		if !strings.HasPrefix(ls[n], k) {
			return nil, equipment.Partial
		}
		count, e := strconv.Atoi(strings.TrimPrefix(ls[n], k))
		if e != nil || count < 0 || count > 1000000 {
			return nil, equipment.Partial
		}
		counts = append(counts, count)
	}
	return counts, equipment.OK
}

func extractSummaryResourceFields(resource string) (map[string]string, bool) {
	fields := map[string]string{}
	rs := routeros.Lines(resource)
	if len(rs) != 7 || rs[6] != "done" {
		return nil, false
	}
	for _, line := range rs[:6] {
		k, v, ok := strings.Cut(line, "=")
		if !ok || fields[k] != "" || !routeros.Clean(v, 128) {
			return nil, false
		}
		fields[k] = v
	}
	for _, k := range []string{"version", "board", "uptime", "cpu", "free", "total"} {
		if fields[k] == "" {
			return nil, false
		}
	}
	return fields, true
}

func populateSummaryObservation(
	o *equipment.Observation, fields map[string]string,
) (*equipment.Observation, equipment.Code) {
	o.Version = fields["version"]
	if strings.HasPrefix(o.Version, "6.") {
		o.Family = equipment.RouterFamilyV6
	} else if strings.HasPrefix(o.Version, "7.") {
		o.Family = equipment.RouterFamilyV7
	} else {
		return nil, equipment.RadioCodeUnsupported
	}
	if len(o.Version) < 3 || o.Version[2] < '0' || o.Version[2] > '9' {
		return nil, equipment.Partial
	}
	o.Board = fields["board"]
	o.Uptime = fields["uptime"]
	var e error
	o.CPU, e = strconv.Atoi(fields["cpu"])
	if e != nil || o.CPU < 0 || o.CPU > 100 {
		return nil, equipment.Partial
	}
	o.FreeMemory, e = strconv.ParseUint(fields["free"], 10, 64)
	if e != nil {
		return nil, equipment.Partial
	}
	o.TotalMemory, e = strconv.ParseUint(fields["total"], 10, 64)
	if e != nil || o.TotalMemory == 0 || o.FreeMemory > o.TotalMemory {
		return nil, equipment.Partial
	}
	return o, equipment.OK
}

func appendInterfaceReadings(o *equipment.Observation, ls []string) equipment.Code {

	seen := map[string]bool{}
	for i := 0; i < len(ls)-1; i += 5 {
		if ls[i+4] != "end" {
			return equipment.Partial
		}
		values := []string{}
		for n, k := range []string{"name", "type", "running", "disabled"} {
			prefix := k + "="
			if !strings.HasPrefix(ls[i+n], prefix) {
				return equipment.Partial
			}
			values = append(values, strings.TrimPrefix(ls[i+n], prefix))
		}
		if !routeros.Clean(values[0],
			128) ||
			!routeros.Clean(values[1],
				80) ||
			seen[values[0]] ||
			(values[2] != "true" &&
				values[2] != "false") ||
			(values[3] != "true" &&
				values[3] != "false") {
			return equipment.Partial
		}
		seen[values[0]] = true
		o.Interfaces = append(o.Interfaces,
			equipment.InterfaceReading{Name: values[0],
				Kind:     values[1],
				Running:  values[2] == "true",
				Disabled: values[3] == "true"})
	}
	return equipment.OK
}
