package routerssh

import (
	"context"
	"infradesktop/internal/modules/equipment"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// This program reads individual allowlisted scalar properties only. No secret
// map, print, password, SQL receipt or export path participates in this scope.
func BuildPPSecretCommand(login string) (string, bool) {
	if login == "" || len(login) > 256 || !utf8.ValidString(login) {
		return "", false
	}
	s := strings.Replace(ppEscapeScript, "> 512", "> 1024", 1) + `; :local ppSecretLogin ` + ppLiteral(login) + `; :do {:local ppSecretIDs [/ppp secret find where name=$ppSecretLogin]; :local up [/system resource get uptime]; :put ("S\t1\t".[/system resource get version]."\t".$up); :local n 0; :local bytes 0; :local reason "complete"; :if ([:len $ppSecretIDs]>4) do={:set reason "rows"} else={:foreach ppSecretID in=$ppSecretIDs do={:local ppSecretDisabled "?"; :do {:local ppSecretFlag [/ppp secret get $ppSecretID disabled]; :if ($ppSecretFlag=true) do={:set ppSecretDisabled "true"} else={:if (([:typeof $ppSecretFlag]="nil")||($ppSecretFlag=false)) do={:set ppSecretDisabled "false"}}} on-error={}; `
	fields := []string{`$ppSecretDisabled`}
	for i, property := range []string{"service", "profile", "last-logged-out", "last-caller-id", "last-disconnect-reason", "comment"} {
		variable := "ppSecretField" + strconv.Itoa(i)
		s += `:local ` + variable + ` "?"; :do {:local ppSecretValue [/ppp secret get $ppSecretID ` + property + `]; :if ([:typeof $ppSecretValue]!="nil") do={`
		if property == "comment" {
			// RouterOS strings are byte-oriented: bound work/SSH to 512 bytes;
			// the UTF-8-aware parser applies the final 128-character ceiling.
			s += `:local ppSecretClean ""; :local ppSecretText [:pick [:tostr $ppSecretValue] 0 512]; :if ([:len $ppSecretText]>0) do={:for ppSecretPos from=0 to=([:len $ppSecretText]-1) do={:local ppSecretChar [:pick $ppSecretText $ppSecretPos ($ppSecretPos+1)]; :if (!($ppSecretChar~"[[:cntrl:]]")) do={:set ppSecretClean ($ppSecretClean.$ppSecretChar)}}}; :set ` + variable + ` ("=".$ppSecretClean)`
		} else {
			s += `:set ` + variable + ` ("=".[:pick [:tostr $ppSecretValue] 0 256])`
		}
		s += `}} on-error={}; `
		fields = append(fields, "$"+variable)
	}
	expr := `"E"`
	for _, field := range fields {
		expr += `."\t".[$esc ` + field + `]`
	}
	s += pdEmit(expr) + `}}; ` + pdFooter() + `} on-error={:put "U\tsecret"}`
	return s, true
}

func ppSecretText(value string, limit int) *string {
	if !strings.HasPrefix(value, "=") {
		return nil
	}
	text := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || r == utf8.RuneError {
			return -1
		}
		return r
	}, value[1:])
	runes := []rune(text)
	if len(runes) > limit {
		text = string(runes[:limit])
	}
	return &text
}

func ParsePPSecret(text string) ([]equipment.PPReadRow, string, equipment.Code) {
	if len(text) > 32*1024 {
		return nil, "", equipment.Limit
	}
	lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(text, "\r\n", "\n"), "\n"), "\n")
	if len(lines) < 2 {
		return nil, "", equipment.Partial
	}
	header := strings.Split(lines[0], "\t")
	if len(header) != 4 || header[0] != "S" || header[1] != "1" || len(header[2]) > 128 || (!strings.HasPrefix(header[2], "6.") && !strings.HasPrefix(header[2], "7.")) {
		return nil, "", equipment.Partial
	}
	version, up := header[2], ParsePPDuration(header[3])
	footer := strings.Split(lines[len(lines)-1], "\t")
	if len(footer) != 4 || footer[0] != "Z" {
		return nil, version, equipment.Partial
	}
	if footer[2] == "rows" || footer[2] == "wire_bytes" {
		return nil, version, equipment.Limit
	}
	n, err := strconv.Atoi(footer[1])
	end := ParsePPDuration(footer[3])
	if err != nil || n != len(lines)-2 || n > 4 || footer[2] != "complete" || !up.Known || !end.Known || end.Max < up.Min {
		return nil, version, equipment.Partial
	}
	rows := make([]equipment.PPReadRow, 0, n)
	for _, line := range lines[1 : len(lines)-1] {
		f := strings.Split(line, "\t")
		if len(f) != 8 || f[0] != "E" {
			return nil, version, equipment.Partial
		}
		for i := 1; i < len(f); i++ {
			value, ok := ppUnescape(strings.ToValidUTF8(f[i], ""))
			if !ok || len(value) > 513 || (i > 1 && value != "?" && !strings.HasPrefix(value, "=")) {
				return nil, version, equipment.Partial
			}
			f[i] = value
		}
		e := &equipment.PPSecretEvidence{}
		switch f[1] {
		case "true", "false":
			b := f[1] == "true"
			e.Disabled = &b
		case "?":
		default:
			return nil, version, equipment.Partial
		}
		e.Service, e.Profile = ppSecretText(f[2], 256), ppSecretText(f[3], 256)
		e.LastOut, e.LastCaller, e.LastReason = ppSecretText(f[4], 256), ppSecretText(f[5], 256), ppSecretText(f[6], 256)
		e.Comment = ppSecretText(f[7], 128)
		rows = append(rows, equipment.PPReadRow{Kind: "secret", Secret: e})
	}
	// Do not depend on a firmware's find/configuration order. Equal projections
	// are indistinguishable; otherwise all seven projected fields define order.
	key := func(e *equipment.PPSecretEvidence) string {
		flag := "?"
		if e.Disabled != nil {
			flag = strconv.FormatBool(*e.Disabled)
		}
		for _, field := range []*string{e.Service, e.Profile, e.LastOut, e.LastCaller, e.LastReason, e.Comment} {
			flag += "\x00"
			if field != nil {
				flag += "=" + *field
			}
		}
		return flag
	}
	sort.SliceStable(rows, func(i, j int) bool { return key(rows[i].Secret) < key(rows[j].Secret) })
	return rows, version, equipment.OK
}

func readPPSecret(ctx context.Context, d equipment.Device, user, secret string, target *equipment.PPReadTarget, check func() equipment.Code) (*equipment.PPReadResult, equipment.Code) {
	if target == nil || len(target.Rows) != 1 {
		return nil, equipment.Invalid
	}
	command, ok := BuildPPSecretCommand(target.Rows[0].Login)
	if !ok {
		return nil, equipment.Invalid
	}
	start := time.Now()
	connection, c := openPPConnection(ctx, d, user, secret, check, equipment.QueryBudget)
	secret = ""
	if c != equipment.OK {
		return nil, c
	}
	defer connection.close()
	connection.tighten(32 * 1024)
	text, c := connection.run(command)
	if c != equipment.OK {
		return nil, c
	}
	rows, version, c := ParsePPSecret(text)
	if c != equipment.OK {
		return nil, c
	}
	if version != target.Observation.Provenance.Version {
		return nil, equipment.ObservationIncompatible
	}
	commands, bytes := connection.metrics()
	return ppMeasureRead(&equipment.PPReadResult{Scope: "secret", Version: version, Started: equipment.PPTime(start), Ended: equipment.PPTime(time.Now()), Duration: time.Since(start).Nanoseconds(), Rows: rows, Cost: equipment.PPReadCost{Commands: commands, Bytes: bytes}})
}
