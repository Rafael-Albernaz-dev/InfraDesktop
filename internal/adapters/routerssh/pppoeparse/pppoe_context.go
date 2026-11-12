package pppoeparse

import (
	"bufio"
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"math"
	"net/netip"
	"strconv"
	"strings"
)

// The monitor map stays on the router. Only the literal scalar allowlist below
// crosses SSH. Neither /ppp secret print nor its password property is requested.
var PPContextCommand = buildPPContext()

func buildPPContext() string {
	s := routeros.EscapeScript + `; :local ids [/interface pppoe-server find]; :local up [/system ` +
		`resource get uptime]; :put ("C\t1\t".[/system resource get versi` +
		`on]."\t".$up); :local n 0; :local bytes 0; :local reason "comple` +
		`te"; :if ([:len $ids]>2048) do={:set reason "rows"} else={:forea` +
		`ch i in=$ids do={:if ($reason="complete") do={:do {`
	s += `:local m [/interface pppoe-server monitor $i once as-value]; :lo` +
		`cal ppContextName [/interface pppoe-server get $i name]; :local ` +
		`ppContextUser ($m->"user"); :local ppContextIP ($m->"remote-addr` +
		`ess"); :local ppContextOrigin ($m->"interface"); :local vid "?";` +
		` :local ppContextProfile ""; :local ppContextLimits ""; :local p` +
		`pContextPrefix ""; :local pdstate "unavailable"; :local l6 "?"; ` +
		`:local r6 "?"; `
	s += `:if ([:typeof ($m->"local-ipv6-address")]!="nil") do={:set l6 [:` +
		`tostr ($m->"local-ipv6-address")]}; :if ([:typeof ($m->"remote-i` +
		`pv6-address")]!="nil") do={:set r6 [:tostr ($m->"remote-ipv6-add` +
		`ress")]}; :do {:set vid [/interface pppoe-server get $i vid]} on` +
		`-error={:do {:local vs [/interface vlan find where name=$ppConte` +
		`xtOrigin]; :if ([:len $vs]=1) do={:set vid [/interface vlan get ` +
		`[:pick $vs 0] vlan-id]}} on-error={}}; `
	s += ppContextSecretProjection
	s += `:do {:local qs [/queue simple find where dynamic=yes and (target` +
		`=($ppContextIP."/32") or target=$ppContextName)]; :if ([:len $qs` +
		`]=1) do={:set ppContextLimits [:tostr [/queue simple get [:pick ` +
		`$qs 0] max-limit]]}} on-error={:set ppContextLimits ""}; `
	s += `:do {:local servers [/ipv6 dhcp-server find where interface=$ppC` +
		`ontextName]; :if ([:len $servers]=0) do={:set pdstate "none"}; :` +
		`if ([:len $servers]>1) do={:set pdstate "ambiguous"}; :if ([:len` +
		` $servers]=1) do={:local sn [/ipv6 dhcp-server get [:pick $serve` +
		`rs 0] name]; :local bs [/ipv6 dhcp-server binding find where sta` +
		`tus=bound and dynamic=yes and server=$sn]; :if ([:len $bs]=0) do` +
		`={:set pdstate "none"}; :if ([:len $bs]=1) do={:set ppContextPre` +
		`fix [:tostr [/ipv6 dhcp-server binding get [:pick $bs 0] address` +
		`]]; :set pdstate "bound"}; :if ([:len $bs]>1) do={:set pdstate "` +
		`ambiguous"}}} on-error={:set pdstate "unavailable"; :set ppConte` +
		`xtPrefix ""}; `
	fields := []string{
		`$i`, `$ppContextName`, `$ppContextUser`, `($m->"caller-id")`, `$ppContextIP`, `($m->"uptime")`,
		`($m->"service")`, `$ppContextOrigin`, `$vid`, `$l6`, `$r6`, `$ppContextProfile`, `$ppContextLimits`,
		`$ppContextPrefix`, `$pdstate`, `$ppContextSecretState`, `$ppContextProfileRate`,
	}
	expr := `"T"`
	for _, field := range fields {
		expr += `."\t".[$esc ` + field + `]`
	}
	s += routeros.Emit(expr) + `} on-error={}}}}; ` + routeros.Footer()
	return s
}

func contextRate(value string) string {
	if value == "" {
		return ""
	}
	factor := uint64(1)
	switch value[len(value)-1] {
	case 'k', 'K':
		factor = 1000
		value = value[:len(value)-1]
	case 'M':
		factor = 1000000
		value = value[:len(value)-1]
	case 'G':
		factor = 1000000000
		value = value[:len(value)-1]
	}
	n, err := strconv.ParseUint(value, 10, 64)
	if err != nil || n > math.MaxUint64/factor {
		return ""
	}
	return strconv.FormatUint(n*factor, 10)
}

func ParsePPContext(text string) ([]equipment.PPTunnel, string, equipment.PPInterval, equipment.Code) {
	if len(text) > equipment.PPWireBytes {
		return nil, "", equipment.PPInterval{}, equipment.Limit
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 4096), 8192)
	s := &ppContextParser{out: []equipment.PPTunnel{}, seen: map[string]bool{}}
	for scanner.Scan() {
		if code := s.consume(strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t"), ""); code != equipment.OK {
			return nil, s.version, s.up, code
		}
	}
	if scanner.Err() != nil || !s.ended {
		return nil, s.version, s.up, equipment.Partial
	}
	return s.out, s.version, s.up, equipment.OK
}

type ppContextParser struct {
	out     []equipment.PPTunnel
	version string
	up      equipment.PPInterval
	ended   bool
	seen    map[string]bool
}

func (s *ppContextParser) consume(f []string, raw string) equipment.Code {

	if s.ended {
		return equipment.Partial
	}
	if s.version == "" {
		return s.header(f)
	}
	if f[0] == "Z" {
		return s.footer(f)
	}
	if len(f) != 18 || f[0] != "T" || len(s.out) >= equipment.PPMaxRows {
		return equipment.Partial
	}
	for i := 1; i < len(f); i++ {
		v, ok := routeros.Unescape(f[i])
		if !ok || len(v) > 256 {
			return equipment.Partial
		}
		f[i] = v
	}
	if f[1] == "" || s.seen[f[1]] {
		return equipment.Partial
	}
	s.seen[f[1]] = true
	item := equipment.PPTunnel{ID: f[1],
		Name:         f[2],
		Login:        f[3],
		Caller:       f[4],
		Address:      f[5],
		Uptime:       ParsePPDuration(f[6]),
		RouterUptime: s.up}
	if ip, e := netip.ParseAddr(item.Address); e == nil {
		item.Address = ip.Unmap().String()
	} else {
		item.Address = ""
	}
	if code := s.context(&item, f); code != equipment.OK {
		return code
	}
	s.out = append(s.out, item)
	return equipment.OK
}

func (s *ppContextParser) header(f []string) equipment.Code {

	if s.version == "" {
		if len(f) != 4 || f[0] != "C" || f[1] != "1" || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
			return equipment.Partial
		}
		s.version = f[2]
		if len(s.version) > 128 {
			return equipment.Partial
		}
		// ignored: validate escaping while retaining the original version token
		if _, ok := routeros.Unescape(s.version); !ok {
			return equipment.Partial
		}
		s.up = ParsePPDuration(f[3])
		return equipment.OK
	}
	return equipment.OK
}

func (s *ppContextParser) footer(f []string) equipment.Code {

	if f[0] == "Z" {
		if len(f) != 4 {
			return equipment.Partial
		}
		n, e := strconv.Atoi(f[1])
		final := ParsePPDuration(f[3])
		if e != nil || n != len(s.out) || !s.up.Known || !final.Known || final.Max < s.up.Min {
			return equipment.Partial
		}
		if f[2] == "rows" || f[2] == "wire_bytes" {
			return equipment.Limit
		}
		if f[2] != "complete" {
			return equipment.Partial
		}
		s.ended = true
		return equipment.OK
	}
	return equipment.OK
}

func (s *ppContextParser) context(item *equipment.PPTunnel, f []string) equipment.Code {

	c := &item.Context
	c.Service = f[7]
	c.Interface = f[8]
	if n, e := strconv.Atoi(f[9]); e == nil && n >= 1 && n <= 4094 {
		c.VLAN = &n
	}
	parseContextIPv6(c, f)

	c.SecretState = "unavailable"
	if len(f) == 18 {
		switch f[16] {
		case "active", "disabled", "ambiguous", "none", "unavailable":
			c.SecretState = f[16]
		default:
			return equipment.Partial
		}
	}
	parseContextProfile(c, f[12], f[17])
	if pair := strings.Split(f[13], "/"); len(pair) == 2 {
		c.UploadLimit = contextRate(pair[0])
		c.DownloadLimit = contextRate(pair[1])
	}
	c.PDStatus = "unavailable"
	if f[15] == "none" && f[14] == "" {
		c.PDStatus = "none"
	}
	if p, e := netip.ParsePrefix(f[14]); e == nil && p.Addr().Is6() && p.Bits() < 128 && f[15] == "bound" {
		c.DelegatedPrefix = p.Masked().String()
		c.PDStatus = "bound"
	}
	return equipment.OK
}

func parseContextIPv6(c *equipment.PPContext, f []string) {

	if f[10] != "?" && f[11] != "?" {
		a, ae := netip.ParseAddr(f[10])
		b, be := netip.ParseAddr(f[11])
		if ae == nil && be == nil && a.Is6() && b.Is6() {
			v := true
			c.IPv6 = &v
		} else if f[10] == "" && f[11] == "" {
			v := false
			c.IPv6 = &v
		}
	}
}

const ppContextSecretProjection = `:local ppContextSecretState "unavailable"; :local ppContextProfileRate ""; ` +
	`:do {:local ppSecrets [/ppp secret find where name=$ppContextUser]; ` +
	`:local ppActiveSecrets [/ppp secret find where name=$ppContextUser and disabled!=yes]; ` +
	`:local ppChosenSecret ""; :local ppHasChosen false; :set ppContextSecretState "none"; ` +
	`:if ([:len $ppSecrets]>0) do={:set ppContextSecretState "ambiguous"}; ` +
	`:if ([:len $ppSecrets]=1) do={:set ppChosenSecret [:pick $ppSecrets 0]; :set ppHasChosen true}; ` +
	`:if ([:len $ppActiveSecrets]=1) do={:set ppChosenSecret [:pick $ppActiveSecrets 0]; :set ppHasChosen true}; ` +
	`:if ($ppHasChosen) do={` +
	`:set ppContextSecretState "disabled"; ` +
	`:if ([/ppp secret get $ppChosenSecret disabled]!=true) do={:set ppContextSecretState "active"}; ` +
	`:set ppContextProfile [/ppp secret get $ppChosenSecret profile]; ` +
	`:do {:local ppProfiles [/ppp profile find where name=$ppContextProfile]; ` +
	`:if ([:len $ppProfiles]=1) do={` +
	`:set ppContextProfileRate [:tostr [/ppp profile get [:pick $ppProfiles 0] rate-limit]]}` +
	`} on-error={:set ppContextProfileRate ""}}` +
	`} on-error={:set ppContextProfile ""; :set ppContextProfileRate ""; ` +
	`:set ppContextSecretState "unavailable"}; `

func parseContextProfile(c *equipment.PPContext, profile, rate string) {
	if c.SecretState != "active" && c.SecretState != "disabled" {
		return
	}
	c.Profile = profile
	rates := strings.Fields(rate)
	if len(rates) == 0 {
		return
	}
	if pair := strings.Split(rates[0], "/"); len(pair) == 2 {
		c.ProfileUploadLimit = contextRate(pair[0])
		c.ProfileDownloadLimit = contextRate(pair[1])
	}
}
