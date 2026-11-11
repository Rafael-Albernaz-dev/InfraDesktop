package deviceparse

import (
	"bufio"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
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
	return `:local ` +
		name +
		` "?"; :do {:set ` +
		name +
		` ("=".[$esc [` +
		menu +
		` get ` +
		property +
		`]])} on-error={}; :put ("R\t` +
		property +
		`\t".$` +
		name +
		`); `
}
func buildDeviceOperationalCommand() string {
	s := routeros.EscapeScript + `; :local up [/system resource get uptime]; :put ("O\t1\t".[/system resource get version]."\t".$up); `
	for _, property := range []string{"cpu-load", "free-memory", "total-memory", "uptime", "version"} {
		s += deviceValue("/system resource", property)
	}
	// Both firmware grammars must be deferred: either parser rejects the other
	// version's health menu before an enclosing on-error could execute.
	temperatureSingleton := `:return ("=".[/system health get temperature])`
	temperatureFallback := `:local ppOpSensors [/system health find where name="temperature"` +
		`]; :if ([:len $ppOpSensors]=1) do={:return ("=".[/system health ` +
		`get [:pick $ppOpSensors 0] value])}; :return "?"`
	s += `:local ppOpTemp "?"; :do {:local ppOpF [:parse ` +
		routeros.Literal(temperatureSingleton) +
		`]; :set ppOpTemp [$ppOpF]} on-error={:do {:local ppOpF [:parse ` +
		routeros.Literal(temperatureFallback) +
		`]; :set ppOpTemp [$ppOpF]} on-error={}}; :put ("R\ttemperature\t` +

		`".$ppOpTemp); `
	s += deviceClockProjection
	s += `:local ppOpErrors "?"; :do {:set ppOpErrors ("=".[/log print cou` +
		`nt-only where topics~"error"])} on-error={}; :put ("R\terror-cou` +
		`nt\t".$ppOpErrors); `
	s += `:local n 0; :local bytes 0; :local reason "complete"; `
	s += `:do {:local ppOpIDs [/ip service find where name="telnet" or nam` +
		`e="ftp" or name="www"]; :if ([:len $ppOpIDs]>16) do={:put "G\tse` +
		`rvices\tlimited"} else={:foreach ppOpID in=$ppOpIDs do={` +
		routeros.Emit(`"S\t".[$esc [/ip service get $ppOpID name]]."\t".[/ip service ge`+

			`t $ppOpID port]."\t".[/ip service get $ppOpID disabled]`) + `}; :put "G\tservices\tcomplete"}} on-error={:put "G\tservices\tu` +
		`navailable"}; `
	s += `:do {:local ppOpIDs [/system package find]; :if ([:len $ppOpIDs]` +
		`>64) do={:put "G\tpackages\tlimited"} else={:foreach ppOpID in=$` +
		`ppOpIDs do={` + routeros.Emit(`"K\t".[$esc [/system package get $ppOpID name]]."\t".[$esc [/sys`+
		`tem package get $ppOpID version]]."\t".[/system package get $ppO`+
		`pID disabled]`) + `}; :put "G\tpackages\tcomplete"}} on-error={:put "G\tpackages\tu` +
		`navailable"}; `
	s += `:do {:local ppOpIDs [/interface find where dynamic!=yes]; :if ([` +
		`:len $ppOpIDs]>128) do={:put "G\tinterfaces\tlimited"} else={:fo` +
		`reach ppOpID in=$ppOpIDs do={:local ppOpRX "?"; :local ppOpTX "?` +
		`"; :local ppOpDown "?"; :do {:set ppOpRX [/interface get $ppOpID` +
		` rx-error]} on-error={}; :do {:set ppOpTX [/interface get $ppOpI` +
		`D tx-error]} on-error={}; :do {:set ppOpDown [/interface get $pp` +
		`OpID link-downs]} on-error={}; ` + routeros.Emit(`"I\t".[$esc [/interface get $ppOpID name]]."\t".[$esc [/interfac`+
		`e get $ppOpID type]]."\t".$ppOpRX."\t".$ppOpTX."\t".$ppOpDown`) + `}; :put "G\tinterfaces\tcomplete"}} on-error={:put "G\tinterface` +
		`s\tunavailable"}; `
	// Existing pool projection: only range/capacity and scalar usage, no used
	// addresses or client identity. No traffic-rate sampling is added here.
	s += `:do {:local ppOpIDs [/ip pool find]; :if ([:len $ppOpIDs]>32) do` +
		`={:put "G\tpools\tlimited"} else={:foreach ppOpID in=$ppOpIDs do` +
		`={:local ppOpPool [/ip pool get $ppOpID name]; :local ppOpUsed "` +
		`?"; :do {:set ppOpUsed [/ip pool used print count-only where poo` +
		`l=$ppOpPool]} on-error={}; ` + routeros.Emit(`"P\t".[$esc $ppOpPool]."\t".[$esc [/ip pool get $ppOpID ranges]]`+
		`."\t".$ppOpUsed`) + `}; :put "G\tpools\tcomplete"}} on-error={:put "G\tpools\tunavailable"}; `
	s += routeros.Footer()
	return s
}

var DeviceOperationalCommand = buildDeviceOperationalCommand()

// Auth messages are classified and reduced on the router. Strictly recognized
// identities and a timestamp are the only output; unknown templates are counted
// as an availability limitation. This audits the finite log buffer, not24h.
func buildDeviceAuthCommand() string {
	s := routeros.EscapeScript + `; :local up [/system resource get uptime]; :put ("A\t1\t".[/syst` +
		`em resource get version]."\t".$up); :local n 0; :local bytes 0; ` +
		`:local reason "complete"; :local ppOpUnknown 0; :do {:local ppOp` +
		`IDs [/log find where topics~"ppp" and !(topics~"debug") and mess` +
		`age~"authentication fail"]; :if ([:len $ppOpIDs]>2048) do={:set ` +
		`reason "rows"} else={:foreach ppOpID in=$ppOpIDs do={:if ($reaso` +
		`n="complete") do={:local ppOpMessage [/log get $ppOpID message];` +
		` :local ppOpLogin ""; :local ppOpSuffix " authentication failed"` +
		`; :local ppOpLen [:len $ppOpMessage]; :if (($ppOpLen>[:len $ppOp` +
		`Suffix])&&([:pick $ppOpMessage ($ppOpLen-[:len $ppOpSuffix]) $pp` +
		`OpLen]=$ppOpSuffix)) do={:set ppOpLogin [:pick $ppOpMessage 0 ($` +
		`ppOpLen-[:len $ppOpSuffix])]; :if ([:pick $ppOpLogin 0 5]="user ` +
		`") do={:set ppOpLogin [:pick $ppOpLogin 5 [:len $ppOpLogin]]}; :` +
		`if (([:pick $ppOpLogin 0 1]="'")&&([:pick $ppOpLogin ([:len $ppO` +
		`pLogin]-1) [:len $ppOpLogin]]="'")) do={:set ppOpLogin [:pick $p` +
		`pOpLogin 1 ([:len $ppOpLogin]-1)]}}; :if (($ppOpLogin~` +
		routeros.Literal(`^[A-Za-z0-9_.@+-]+$`) +
		`)&&([:len $ppOpLogin]<=128)) do={` +
		routeros.Emit(`"F\t".[$esc $ppOpLogin]."\t".[$esc [/log get $ppOpID time]]`) +
		`} else={:set ppOpUnknown ($ppOpUnknown+1)}}}}; :put ("G\tauth\t"` +

		`.$ppOpUnknown); } on-error={:put "G\tauth\tunavailable"}; ` + routeros.Footer()
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
	return routeros.Counter(*p)
}
func deviceBool(v string) *bool {
	p := deviceOptional(v)
	if p == nil {
		return nil
	}
	b, ok := routeros.Bool(*p, false)
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

func ParseDeviceAuthFailures(text string) ([]equipment.PPReadRow, equipment.Code) {
	// ignored: compatibility API returns rows and code without the diagnostic reason
	rows, code, _ := ParseDeviceAuthFailuresWithReason(text)
	return rows, code
}
func ParseDeviceAuthFailuresWithReason(text string) ([]equipment.PPReadRow, equipment.Code, string) {
	if routeros.CollectorSyntaxDiagnostic(text) {
		return nil, equipment.CollectorDefect, "A:syntax"
	}
	if len(text) > 32*1024 {
		return nil, equipment.Limit, ""
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	s := &deviceAuthParser{unknown: "unavailable", rows: []equipment.PPReadRow{}, indexes: map[string]int{}}
	for scanner.Scan() {
		f := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		if code, reason := s.consume(f); code != equipment.OK {
			return nil, code, reason
		}
	}
	rule := "A:missing"
	if scanner.Err() != nil {
		rule = "wire:line-limit"
	}
	if scanner.Err() != nil || !s.started || !s.ended || !s.availability {
		return nil, equipment.Partial, rule
	}
	if s.unknown == "unavailable" {
		return []equipment.PPReadRow{{Kind: "availability", Name: "auth", Value: "unavailable"}}, equipment.OK, ""
	}
	status := "complete"
	if s.unknown != "0" {
		status = "partial"
	}
	// Stable ranking by counts; equal counts use login only for local display.
	sortDeviceAuth(s.rows)
	recognized := uint64(s.count)
	s.rows = append(s.rows,
		equipment.PPReadRow{Kind: "availability",
			Name:  "auth",
			Value: status,
			Used:  &recognized,
			Size:  routeros.Counter(s.unknown)})
	return s.rows, equipment.OK, ""
}

type deviceAuthParser struct {
	started, ended, availability bool
	count                        int
	unknown                      string
	rows                         []equipment.PPReadRow
	indexes                      map[string]int
}

func (s *deviceAuthParser) consume(f []string) (equipment.Code, string) {
	if s.ended {
		return equipment.Partial, "Z:trailing"
	}
	if !s.started {
		if len(f) != 4 || f[0] != "A" || f[1] != "1" ||
			(!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
			return equipment.Partial, "A:header"
		}
		s.started = true
		return equipment.OK, ""
	}
	if f[0] == "Z" {
		return s.footer(f)
	}
	if len(f) != 3 {
		return equipment.Partial, "A:fields"
	}
	if f[0] == "G" {
		if f[1] != "auth" || s.availability {
			return equipment.Partial, "G:auth"
		}
		s.availability = true
		s.unknown = f[2]
		if s.unknown != "unavailable" && routeros.Counter(s.unknown) == nil {
			return equipment.Partial, "G:auth"
		}
		return equipment.OK, ""
	}
	return s.failure(f)
}

func (s *deviceAuthParser) footer(f []string) (equipment.Code, string) {
	if len(f) != 4 {
		return equipment.Partial, "A:footer"
	}
	n, e := strconv.Atoi(f[1])
	if f[2] == "rows" || f[2] == "wire_bytes" {
		return equipment.Limit, ""
	}
	if e != nil || n != s.count || f[2] != "complete" {
		return equipment.Partial, "A:footer"
	}
	s.ended = true
	return equipment.OK, ""
}

func (s *deviceAuthParser) failure(f []string) (equipment.Code, string) {
	if f[0] != "F" || s.count >= 2048 {
		return equipment.Partial, "F:fields"
	}
	login, ok := routeros.Unescape(f[1])
	if !ok || len(login) == 0 || len(login) > 128 || strings.ContainsAny(login, " \t\r\n'\"<>") {
		return equipment.Partial, "F:identity"
	}
	at, ok := routeros.Unescape(f[2])
	if !ok {
		return equipment.Partial, "F:time"
	}
	at = routeros.LogTime(at)
	if n, ok := s.indexes[login]; ok {
		v := *s.rows[n].Used + 1
		s.rows[n].Used = &v
		s.rows[n].Value = at
	} else {
		v := uint64(1)
		s.indexes[login] = len(s.rows)
		s.rows = append(s.rows, equipment.PPReadRow{Kind: "auth_failure", Login: login, Used: &v, At: at, Value: at})
	}
	s.count++
	return equipment.OK, ""
}
func sortDeviceAuth(rows []equipment.PPReadRow) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && (*rows[j].Used > *rows[j-1].Used || (*rows[j].Used == *rows[j-1].Used && rows[j].Login < rows[j-1].Login)); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}

const deviceClockProjection = `:local ppOpDate "?"; :local ppOpTime "?"; :local ppOpGMT "?"; :l` +
	`ocal ppOpNTP "?"; :local ppOpSync "?"; :do {:set ppOpDate ("=".[` +
	`/system clock get date]); :set ppOpTime ("=".[/system clock get ` +
	`time]); :set ppOpGMT ("=".[:tonum [/system clock get gmt-offset]` +
	`])} on-error={}; :do {:set ppOpNTP ("=".[/system ntp client get ` +
	`enabled]); :set ppOpSync ("=".[/system ntp client get status])} ` +
	`on-error={}; :put ("C\t".$ppOpDate."\t".$ppOpTime."\t".$ppOpGMT.` +
	`"\t".$ppOpNTP."\t".$ppOpSync); `
