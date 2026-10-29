package routerssh

import (
	"bufio"
	"context"
	"infradesktop/internal/modules/equipment"
	"math"
	"strconv"
	"strings"
	"time"
)

// Named properties only: neither configuration maps nor log messages leave the
// router. Optional groups have independent availability markers.
func deviceValue(menu, property string) string {
	name := "ppOpR" + strings.ReplaceAll(property, "-", "")
	return `:local ` + name + ` "?"; :do {:set ` + name + ` ("=".[$esc [` + menu + ` get ` + property + `]])} on-error={}; :put ("R\t` + property + `\t".$` + name + `); `
}
func buildDeviceOperationalCommand() string {
	s := ppEscapeScript + `; :local up [/system resource get uptime]; :put ("O\t1\t".[/system resource get version]."\t".$up); `
	for _, property := range []string{"cpu-load", "free-memory", "total-memory", "uptime", "version"} {
		s += deviceValue("/system resource", property)
	}
	// Both firmware grammars must be deferred: either parser rejects the other
	// version's health menu before an enclosing on-error could execute.
	temperatureSingleton := `:return ("=".[/system health get temperature])`
	temperatureFallback := `:local ppOpSensors [/system health find where name="temperature"]; :if ([:len $ppOpSensors]=1) do={:return ("=".[/system health get [:pick $ppOpSensors 0] value])}; :return "?"`
	s += `:local ppOpTemp "?"; :do {:local ppOpF [:parse ` + ppLiteral(temperatureSingleton) + `]; :set ppOpTemp [$ppOpF]} on-error={:do {:local ppOpF [:parse ` + ppLiteral(temperatureFallback) + `]; :set ppOpTemp [$ppOpF]} on-error={}}; :put ("R\ttemperature\t".$ppOpTemp); `
	s += `:local ppOpDate "?"; :local ppOpTime "?"; :local ppOpGMT "?"; :local ppOpNTP "?"; :local ppOpSync "?"; :do {:set ppOpDate ("=".[/system clock get date]); :set ppOpTime ("=".[/system clock get time]); :set ppOpGMT ("=".[:tonum [/system clock get gmt-offset]])} on-error={}; :do {:set ppOpNTP ("=".[/system ntp client get enabled]); :set ppOpSync ("=".[/system ntp client get status])} on-error={}; :put ("C\t".$ppOpDate."\t".$ppOpTime."\t".$ppOpGMT."\t".$ppOpNTP."\t".$ppOpSync); `
	s += `:local ppOpErrors "?"; :do {:set ppOpErrors ("=".[/log print count-only where topics~"error"])} on-error={}; :put ("R\terror-count\t".$ppOpErrors); `
	s += `:local n 0; :local bytes 0; :local reason "complete"; `
	s += `:do {:local ppOpIDs [/ip service find where name="telnet" or name="ftp" or name="www"]; :if ([:len $ppOpIDs]>16) do={:put "G\tservices\tlimited"} else={:foreach ppOpID in=$ppOpIDs do={` + pdEmit(`"S\t".[$esc [/ip service get $ppOpID name]]."\t".[/ip service get $ppOpID port]."\t".[/ip service get $ppOpID disabled]`) + `}; :put "G\tservices\tcomplete"}} on-error={:put "G\tservices\tunavailable"}; `
	s += `:do {:local ppOpIDs [/system package find]; :if ([:len $ppOpIDs]>64) do={:put "G\tpackages\tlimited"} else={:foreach ppOpID in=$ppOpIDs do={` + pdEmit(`"K\t".[$esc [/system package get $ppOpID name]]."\t".[$esc [/system package get $ppOpID version]]."\t".[/system package get $ppOpID disabled]`) + `}; :put "G\tpackages\tcomplete"}} on-error={:put "G\tpackages\tunavailable"}; `
	s += `:do {:local ppOpIDs [/interface find where dynamic!=yes]; :if ([:len $ppOpIDs]>128) do={:put "G\tinterfaces\tlimited"} else={:foreach ppOpID in=$ppOpIDs do={:local ppOpRX "?"; :local ppOpTX "?"; :local ppOpDown "?"; :do {:set ppOpRX [/interface get $ppOpID rx-error]} on-error={}; :do {:set ppOpTX [/interface get $ppOpID tx-error]} on-error={}; :do {:set ppOpDown [/interface get $ppOpID link-downs]} on-error={}; ` + pdEmit(`"I\t".[$esc [/interface get $ppOpID name]]."\t".[$esc [/interface get $ppOpID type]]."\t".$ppOpRX."\t".$ppOpTX."\t".$ppOpDown`) + `}; :put "G\tinterfaces\tcomplete"}} on-error={:put "G\tinterfaces\tunavailable"}; `
	// Existing pool projection: only range/capacity and scalar usage, no used
	// addresses or client identity. No traffic-rate sampling is added here.
	s += `:do {:local ppOpIDs [/ip pool find]; :if ([:len $ppOpIDs]>32) do={:put "G\tpools\tlimited"} else={:foreach ppOpID in=$ppOpIDs do={:local ppOpPool [/ip pool get $ppOpID name]; :local ppOpUsed "?"; :do {:set ppOpUsed [/ip pool used print count-only where pool=$ppOpPool]} on-error={}; ` + pdEmit(`"P\t".[$esc $ppOpPool]."\t".[$esc [/ip pool get $ppOpID ranges]]."\t".$ppOpUsed`) + `}; :put "G\tpools\tcomplete"}} on-error={:put "G\tpools\tunavailable"}; `
	s += pdFooter()
	return s
}

var DeviceOperationalCommand = buildDeviceOperationalCommand()

// Auth messages are classified and reduced on the router. Strictly recognized
// identities and a timestamp are the only output; unknown templates are counted
// as an availability limitation. This audits the finite log buffer, not24h.
func buildDeviceAuthCommand() string {
	s := ppEscapeScript + `; :local up [/system resource get uptime]; :put ("A\t1\t".[/system resource get version]."\t".$up); :local n 0; :local bytes 0; :local reason "complete"; :local ppOpUnknown 0; :do {:local ppOpIDs [/log find where topics~"ppp" and !(topics~"debug") and message~"authentication fail"]; :if ([:len $ppOpIDs]>2048) do={:set reason "rows"} else={:foreach ppOpID in=$ppOpIDs do={:if ($reason="complete") do={:local ppOpMessage [/log get $ppOpID message]; :local ppOpLogin ""; :local ppOpSuffix " authentication failed"; :local ppOpLen [:len $ppOpMessage]; :if (($ppOpLen>[:len $ppOpSuffix])&&([:pick $ppOpMessage ($ppOpLen-[:len $ppOpSuffix]) $ppOpLen]=$ppOpSuffix)) do={:set ppOpLogin [:pick $ppOpMessage 0 ($ppOpLen-[:len $ppOpSuffix])]; :if ([:pick $ppOpLogin 0 5]="user ") do={:set ppOpLogin [:pick $ppOpLogin 5 [:len $ppOpLogin]]}; :if (([:pick $ppOpLogin 0 1]="'")&&([:pick $ppOpLogin ([:len $ppOpLogin]-1) [:len $ppOpLogin]]="'")) do={:set ppOpLogin [:pick $ppOpLogin 1 ([:len $ppOpLogin]-1)]}}; :if (($ppOpLogin~` + ppLiteral(`^[A-Za-z0-9_.@+-]+$`) + `)&&([:len $ppOpLogin]<=128)) do={` + pdEmit(`"F\t".[$esc $ppOpLogin]."\t".[$esc [/log get $ppOpID time]]`) + `} else={:set ppOpUnknown ($ppOpUnknown+1)}}}}; :put ("G\tauth\t".$ppOpUnknown); } on-error={:put "G\tauth\tunavailable"}; ` + pdFooter()
	return s
}

var DeviceAuthFailuresCommand = buildDeviceAuthCommand()

func deviceOptional(v string) *string {
	if !strings.HasPrefix(v, "=") {
		return nil
	}
	v = strings.TrimPrefix(v, "=")
	return &v
}
func deviceNumber(v string) *uint64 {
	p := deviceOptional(v)
	if p == nil {
		return nil
	}
	return ppCounter(*p)
}
func deviceBool(v string) *bool {
	p := deviceOptional(v)
	if p == nil {
		return nil
	}
	b, ok := pdBool(*p, false)
	if !ok {
		return nil
	}
	return b
}
func deviceClock(date, clock, offset string, begin, end time.Time) equipment.PPInterval {
	d, c, o := deviceOptional(date), deviceOptional(clock), deviceOptional(offset)
	if d == nil || c == nil || o == nil {
		return equipment.PPInterval{}
	}
	raw, e := strconv.ParseInt(*o, 10, 64)
	if e != nil {
		return equipment.PPInterval{}
	}
	if raw > math.MaxInt32 && raw <= math.MaxUint32 {
		raw = int64(int32(uint32(raw)))
	}
	if raw < -86400 || raw > 86400 {
		return equipment.PPInterval{}
	}
	zone := time.FixedZone("router", int(raw))
	for _, layout := range []string{"2006-01-02 15:04:05", "Jan/02/2006 15:04:05", "jan/02/2006 15:04:05"} {
		stamp, e := time.ParseInLocation(layout, *d+" "+*c, zone)
		if e == nil {
			return equipment.PPInterval{Known: true, Min: stamp.Sub(end).Nanoseconds(), Max: stamp.Add(time.Second).Sub(begin).Nanoseconds()}
		}
	}
	return equipment.PPInterval{}
}
func ParseDeviceOperational(text string, begin, end time.Time) (*equipment.DeviceHealth, []equipment.PPReadRow, equipment.Code) {
	h, rows, code, _ := parseDeviceOperational(text, begin, end)
	return h, rows, code
}
func parseDeviceOperational(text string, begin, end time.Time) (*equipment.DeviceHealth, []equipment.PPReadRow, equipment.Code, string) {
	rule := "O:header"
	if collectorSyntaxDiagnostic(text) {
		return nil, nil, equipment.CollectorDefect, "O:syntax"
	}
	if len(text) > equipment.PPWireBytes {
		return nil, nil, equipment.Limit, ""
	}
	h := &equipment.DeviceHealth{Started: equipment.PPTime(begin), At: equipment.PPTime(end)}
	rows := []equipment.PPReadRow{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	started, ended := false, false
	count := 0
	var pendingCount *uint64
	seen := map[string]bool{}
	group := map[string]string{}
	for scanner.Scan() {
		raw := strings.TrimSuffix(scanner.Text(), "\r")
		f := strings.Split(raw, "\t")
		rule = "Z:trailing"
		if ended {
			return nil, nil, equipment.Partial, rule
		}
		rule = "ROW:fields"
		if started && len(f) == 1 && ppCounter(raw) != nil {
			if pendingCount != nil {
				return nil, nil, equipment.Partial, rule
			}
			pendingCount = ppCounter(raw)
			continue
		}
		if !started {
			rule = "O:header"
			if len(f) != 4 || f[0] != "O" || f[1] != "1" || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
				return nil, nil, equipment.Partial, rule
			}
			h.Version = f[2]
			h.Uptime = ParsePPDuration(f[3])
			started = true
			continue
		}
		if f[0] == "Z" {
			rule = "Z:fields"
			if len(f) != 4 {
				return nil, nil, equipment.Partial, rule
			}
			n, e := strconv.Atoi(f[1])
			if f[2] == "rows" || f[2] == "wire_bytes" {
				return nil, nil, equipment.Limit, ""
			}
			rule = "Z:footer"
			final := ParsePPDuration(f[3])
			if e != nil || n != count || f[2] != "complete" || pendingCount != nil || !h.Uptime.Known || !final.Known || final.Max < h.Uptime.Min {
				return nil, nil, equipment.Partial, rule
			}
			ended = true
			continue
		}
		rule = "wire:codec"
		for i := 1; i < len(f); i++ {
			v, ok := ppUnescape(f[i])
			if !ok || len(v) > 256 {
				return nil, nil, equipment.Partial, rule
			}
			f[i] = v
		}
		rule = "ROW:fields"
		if len(f) < 3 {
			return nil, nil, equipment.Partial, rule
		}
		key := f[0] + "/" + f[1]
		rule = "ROW:duplicate"
		if seen[key] {
			return nil, nil, equipment.Partial, rule
		}
		seen[key] = true
		rule = "ROW:type"
		switch f[0] {
		case "R":
			rule = "R:fields"
			if len(f) != 3 {
				return nil, nil, equipment.Partial, rule
			}
			switch f[1] {
			case "cpu-load":
				h.CPU = deviceNumber(f[2])
				if h.CPU != nil && *h.CPU > 100 {
					h.CPU = nil
				}
			case "free-memory":
				h.FreeMemory = deviceNumber(f[2])
			case "total-memory":
				h.TotalMemory = deviceNumber(f[2])
			case "temperature":
				if v := deviceOptional(f[2]); v != nil {
					if n, e := strconv.ParseFloat(*v, 64); e == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= -100 && n <= 200 {
						h.Temperature = &n
					}
				}
			case "error-count":
				h.ErrorCount = deviceNumber(f[2])
				if pendingCount != nil {
					rule = "R:echo-mismatch"
					if h.ErrorCount == nil || *h.ErrorCount != *pendingCount {
						return nil, nil, equipment.Partial, rule
					}
					pendingCount = nil
				}
			case "uptime", "version":
			default:
				return nil, nil, equipment.Partial, rule
			}
		case "C":
			rule = "C:fields"
			if len(f) != 6 {
				return nil, nil, equipment.Partial, rule
			}
			h.ClockOffset = deviceClock(f[1], f[2], f[3], begin, end)
			h.NTPEnabled = deviceBool(f[4])
			if v := deviceOptional(f[5]); v != nil {
				switch *v {
				case "synchronized", "stopped", "waiting", "using-local-clock":
					h.NTPStatus = *v
				}
			}
		case "G":
			rule = "G:fields"
			if len(f) != 3 || (f[2] != "complete" && f[2] != "unavailable" && f[2] != "limited") {
				return nil, nil, equipment.Partial, rule
			}
			switch f[1] {
			case "services", "packages", "interfaces", "pools":
				group[f[1]] = f[2]
			default:
				return nil, nil, equipment.Partial, rule
			}
		case "S":
			rule = "S:fields"
			if len(f) != 4 {
				return nil, nil, equipment.Partial, rule
			}
			switch f[1] {
			case "telnet", "ftp", "www":
			default:
				return nil, nil, equipment.Partial, rule
			}
			b, ok := pdBool(f[3], false)
			if !ok {
				b = nil
			}
			rows = append(rows, equipment.PPReadRow{Kind: "service", Name: f[1], Used: ppCounter(f[2]), Disabled: b})
			count++
		case "K":
			rule = "K:fields"
			if len(f) != 4 {
				return nil, nil, equipment.Partial, rule
			}
			b, ok := pdBool(f[3], true)
			if !ok {
				b = nil
			}
			rows = append(rows, equipment.PPReadRow{Kind: "package", Name: f[1], Value: f[2], Disabled: b})
			count++
		case "I":
			rule = "I:fields"
			if len(f) != 6 {
				return nil, nil, equipment.Partial, rule
			}
			rows = append(rows, equipment.PPReadRow{Kind: "interface", Name: f[1], Value: f[2], RXErrors: ppCounterString(f[3]), TXErrors: ppCounterString(f[4]), LinkDowns: ppCounterString(f[5])})
			count++
		case "P":
			rule = "P:fields"
			if len(f) != 4 {
				return nil, nil, equipment.Partial, rule
			}
			used, size := ppCounter(f[3]), PPPoolSize(f[2])
			if pendingCount != nil {
				rule = "P:echo-mismatch"
				if used == nil || *used != *pendingCount {
					return nil, nil, equipment.Partial, rule
				}
				pendingCount = nil
			}
			if used != nil && size != nil && *used > *size {
				used = nil
			}
			rows = append(rows, equipment.PPReadRow{Kind: "pool", Name: f[1], Used: used, Size: size})
			count++
		default:
			return nil, nil, equipment.Partial, rule
		}
		if count > equipment.PPMaxRows {
			return nil, nil, equipment.Limit, ""
		}
	}
	rule = "Z:missing"
	if scanner.Err() != nil {
		rule = "wire:line-limit"
	}
	if scanner.Err() != nil || !started || !ended {
		return nil, nil, equipment.Partial, rule
	}
	if h.FreeMemory != nil && h.TotalMemory != nil && *h.FreeMemory > *h.TotalMemory {
		h.FreeMemory = nil
	}
	clean := []equipment.PPReadRow{{Kind: "resource", Health: h}}
	groupKind := map[string]string{"service": "services", "package": "packages", "interface": "interfaces", "pool": "pools"}
	for _, r := range rows {
		if group[groupKind[r.Kind]] == "complete" {
			clean = append(clean, r)
		}
	}
	for _, name := range []string{"services", "packages", "interfaces", "pools"} {
		status := group[name]
		if status == "" {
			status = "unavailable"
		}
		clean = append(clean, equipment.PPReadRow{Kind: "availability", Name: name, Value: status})
	}
	return h, clean, equipment.OK, ""
}
func ParseDeviceAuthFailures(text string) ([]equipment.PPReadRow, equipment.Code) {
	rows, code, _ := parseDeviceAuthFailures(text)
	return rows, code
}
func parseDeviceAuthFailures(text string) ([]equipment.PPReadRow, equipment.Code, string) {
	rule := "A:header"
	if collectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect, "A:syntax"
	}
	if len(text) > 32*1024 {
		return nil, equipment.Limit, ""
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	started, ended, availability := false, false, false
	count := 0
	unknown := "unavailable"
	rows := []equipment.PPReadRow{}
	indexes := map[string]int{}
	for scanner.Scan() {
		f := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		rule = "Z:trailing"
		if ended {
			return nil, equipment.Partial, rule
		}
		if !started {
			rule = "A:header"
			if len(f) != 4 || f[0] != "A" || f[1] != "1" || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
				return nil, equipment.Partial, rule
			}
			started = true
			continue
		}
		if f[0] == "Z" {
			rule = "A:footer"
			if len(f) != 4 {
				return nil, equipment.Partial, rule
			}
			n, e := strconv.Atoi(f[1])
			if f[2] == "rows" || f[2] == "wire_bytes" {
				return nil, equipment.Limit, ""
			}
			if e != nil || n != count || f[2] != "complete" {
				return nil, equipment.Partial, rule
			}
			ended = true
			continue
		}
		rule = "A:fields"
		if len(f) != 3 {
			return nil, equipment.Partial, rule
		}
		if f[0] == "G" {
			rule = "G:auth"
			if f[1] != "auth" || availability {
				return nil, equipment.Partial, rule
			}
			availability = true
			unknown = f[2]
			if unknown != "unavailable" && ppCounter(unknown) == nil {
				return nil, equipment.Partial, rule
			}
			continue
		}
		rule = "F:fields"
		if f[0] != "F" || count >= 2048 {
			return nil, equipment.Partial, rule
		}
		rule = "F:identity"
		login, ok := ppUnescape(f[1])
		if !ok || len(login) == 0 || len(login) > 128 || strings.ContainsAny(login, " \t\r\n'\"<>") {
			return nil, equipment.Partial, rule
		}
		rule = "F:time"
		at, ok := ppUnescape(f[2])
		if !ok {
			return nil, equipment.Partial, rule
		}
		at = ppLogTime(at)
		if n, ok := indexes[login]; ok {
			v := *rows[n].Used + 1
			rows[n].Used = &v
			rows[n].Value = at
		} else {
			v := uint64(1)
			indexes[login] = len(rows)
			rows = append(rows, equipment.PPReadRow{Kind: "auth_failure", Login: login, Used: &v, At: at, Value: at})
		}
		count++
	}
	rule = "A:missing"
	if scanner.Err() != nil {
		rule = "wire:line-limit"
	}
	if scanner.Err() != nil || !started || !ended || !availability {
		return nil, equipment.Partial, rule
	}
	if unknown == "unavailable" {
		return []equipment.PPReadRow{{Kind: "availability", Name: "auth", Value: "unavailable"}}, equipment.OK, ""
	}
	status := "complete"
	if unknown != "0" {
		status = "partial"
	}
	// Stable ranking by counts; equal counts use login only for local display.
	sortDeviceAuth(rows)
	recognized := uint64(count)
	rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "auth", Value: status, Used: &recognized, Size: ppCounter(unknown)})
	return rows, equipment.OK, ""
}
func sortDeviceAuth(rows []equipment.PPReadRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && (*rows[j].Used > *rows[j-1].Used || (*rows[j].Used == *rows[j-1].Used && rows[j].Login < rows[j-1].Login)); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
func readDeviceOperational(ctx context.Context, d equipment.Device, user, secret string, check func() equipment.Code) (*equipment.PPReadResult, equipment.Code) {
	started := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	handshake := time.Since(started).Nanoseconds()
	begin := time.Now()
	text, c := connection.run(DeviceOperationalCommand)
	end := time.Now()
	if c != equipment.OK {
		if c == equipment.CollectorDefect {
			return collectorFailure(c, "O:syntax")
		}
		return collectorFailure(c, "O:execution")
	}
	health, rows, c, reason := parseDeviceOperational(text, begin, end)
	if c != equipment.OK {
		return collectorFailure(c, reason)
	}
	connection.tighten(32 * 1024)
	text, c = connection.run(DeviceAuthFailuresCommand)
	if c == equipment.OK {
		auth, code, authReason := parseDeviceAuthFailures(text)
		if code == equipment.OK {
			rows = append(rows, auth...)
		} else if code == equipment.Partial || code == equipment.CollectorDefect {
			return collectorFailure(code, authReason)
		} else if code == equipment.Limit {
			rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "auth", Value: "unavailable"})
		} else {
			return nil, code
		}
	} else if c == equipment.Partial || c == equipment.CollectorDefect {
		if c == equipment.CollectorDefect {
			return collectorFailure(c, "A:syntax")
		}
		return collectorFailure(c, "A:execution")
	} else if c == equipment.Limit || c == "REMOTE_PERMISSION_DENIED" {
		rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: "auth", Value: "unavailable"})
	} else {
		return nil, c
	}
	// Human-proved narrow reads share the connection and unchanged20s deadline.
	// Syntax/structural failures are never treated as capability absence.
	for _, group := range []struct{ name, command string }{{"fasttrack", DeviceFastTrackCommand}, {"ports", DevicePortsCommand}} {
		text, code := connection.run(group.command)
		if code == "REMOTE_PERMISSION_DENIED" {
			rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: group.name, Value: "unavailable"})
			continue
		}
		if code != equipment.OK {
			return nil, code
		}
		if group.name == "fasttrack" {
			n, code := parseDeviceFastTrack(text)
			if code != equipment.OK {
				return nil, code
			}
			health.FastTrack = n
		} else {
			ports, code := parseDevicePorts(text)
			if code != equipment.OK {
				return nil, code
			}
			rows = append(rows, ports...)
		}
		rows = append(rows, equipment.PPReadRow{Kind: "availability", Name: group.name, Value: "complete"})
	}
	commands, bytes := connection.metrics()
	collector := time.Since(begin).Nanoseconds()
	result := &equipment.PPReadResult{Scope: "operations", Started: equipment.PPTime(started), Ended: equipment.PPTime(time.Now()), Version: health.Version, Duration: time.Since(started).Nanoseconds(), Rows: rows, Cost: equipment.PPReadCost{Handshake: &handshake, Collector: &collector, Commands: commands, Bytes: bytes, CPUStart: health.CPU, CPUEnd: health.CPU}}
	result.Availability = map[string]string{}
	for _, r := range rows {
		if r.Kind == "availability" {
			result.Availability[r.Name] = r.Value
		}
	}
	return ppMeasureRead(result)
}
