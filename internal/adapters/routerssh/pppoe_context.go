package routerssh

import (
	"bufio"
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
	s := ppEscapeScript + `; :local ids [/interface pppoe-server find]; :local up [/system resource get uptime]; :put ("C\t1\t".[/system resource get version]."\t".$up); :local n 0; :local bytes 0; :local reason "complete"; :if ([:len $ids]>2048) do={:set reason "rows"} else={:foreach i in=$ids do={:if ($reason="complete") do={:do {`
	s += `:local m [/interface pppoe-server monitor $i once as-value]; :local ppContextName [/interface pppoe-server get $i name]; :local ppContextUser ($m->"user"); :local ppContextIP ($m->"remote-address"); :local ppContextOrigin ($m->"interface"); :local vid "?"; :local ppContextProfile ""; :local ppContextLimits ""; :local ppContextPrefix ""; :local pdstate "unavailable"; :local l6 "?"; :local r6 "?"; `
	s += `:if ([:typeof ($m->"local-ipv6-address")]!="nil") do={:set l6 [:tostr ($m->"local-ipv6-address")]}; :if ([:typeof ($m->"remote-ipv6-address")]!="nil") do={:set r6 [:tostr ($m->"remote-ipv6-address")]}; :do {:set vid [/interface pppoe-server get $i vid]} on-error={:do {:local vs [/interface vlan find where name=$ppContextOrigin]; :if ([:len $vs]=1) do={:set vid [/interface vlan get [:pick $vs 0] vlan-id]}} on-error={}}; `
	s += `:local ppContextSecretState "unavailable"; :do {:local ppSecrets [/ppp secret find where name=$ppContextUser]; :local ppActiveSecrets [/ppp secret find where name=$ppContextUser and disabled!=yes]; :set ppContextSecretState "none"; :if ([:len $ppSecrets]>0) do={:set ppContextSecretState "disabled"}; :if ([:len $ppActiveSecrets]>1) do={:set ppContextSecretState "ambiguous"}; :if ([:len $ppActiveSecrets]=1) do={:local ppActiveSecret [:pick $ppActiveSecrets 0]; :if ([/ppp secret get $ppActiveSecret disabled]!=true) do={:set ppContextProfile [/ppp secret get $ppActiveSecret profile]; :if ([/ppp secret get $ppActiveSecret disabled]!=true) do={:set ppContextSecretState "active"} else={:set ppContextProfile ""; :set ppContextSecretState "unavailable"}}}} on-error={:set ppContextProfile ""; :set ppContextSecretState "unavailable"}; `
	s += `:do {:local qs [/queue simple find where dynamic=yes and (target=($ppContextIP."/32") or target=$ppContextName)]; :if ([:len $qs]=1) do={:set ppContextLimits [:tostr [/queue simple get [:pick $qs 0] max-limit]]}} on-error={:set ppContextLimits ""}; `
	s += `:do {:local servers [/ipv6 dhcp-server find where interface=$ppContextName]; :if ([:len $servers]=0) do={:set pdstate "none"}; :if ([:len $servers]>1) do={:set pdstate "ambiguous"}; :if ([:len $servers]=1) do={:local sn [/ipv6 dhcp-server get [:pick $servers 0] name]; :local bs [/ipv6 dhcp-server binding find where status=bound and dynamic=yes and server=$sn]; :if ([:len $bs]=0) do={:set pdstate "none"}; :if ([:len $bs]=1) do={:set ppContextPrefix [:tostr [/ipv6 dhcp-server binding get [:pick $bs 0] address]]; :set pdstate "bound"}; :if ([:len $bs]>1) do={:set pdstate "ambiguous"}}} on-error={:set pdstate "unavailable"; :set ppContextPrefix ""}; `
	fields := []string{`$i`, `$ppContextName`, `$ppContextUser`, `($m->"caller-id")`, `$ppContextIP`, `($m->"uptime")`, `($m->"service")`, `$ppContextOrigin`, `$vid`, `$l6`, `$r6`, `$ppContextProfile`, `$ppContextLimits`, `$ppContextPrefix`, `$pdstate`, `$ppContextSecretState`}
	expr := `"T"`
	for _, field := range fields {
		expr += `."\t".[$esc ` + field + `]`
	}
	s += pdEmit(expr) + `} on-error={}}}}; ` + pdFooter()
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
	out := []equipment.PPTunnel{}
	version := ""
	up := equipment.PPInterval{}
	ended := false
	seen := map[string]bool{}
	for scanner.Scan() {
		f := strings.Split(strings.TrimSuffix(scanner.Text(), "\r"), "\t")
		if ended {
			return nil, version, up, equipment.Partial
		}
		if version == "" {
			if len(f) != 4 || f[0] != "C" || f[1] != "1" || (!strings.HasPrefix(f[2], "6.") && !strings.HasPrefix(f[2], "7.")) {
				return nil, version, up, equipment.Partial
			}
			version = f[2]
			if len(version) > 128 {
				return nil, version, up, equipment.Partial
			}
			if _, ok := ppUnescape(version); !ok {
				return nil, version, up, equipment.Partial
			}
			up = ParsePPDuration(f[3])
			continue
		}
		if f[0] == "Z" {
			if len(f) != 4 {
				return nil, version, up, equipment.Partial
			}
			n, e := strconv.Atoi(f[1])
			final := ParsePPDuration(f[3])
			if e != nil || n != len(out) || !up.Known || !final.Known || final.Max < up.Min {
				return nil, version, up, equipment.Partial
			}
			if f[2] == "rows" || f[2] == "wire_bytes" {
				return nil, version, up, equipment.Limit
			}
			if f[2] != "complete" {
				return nil, version, up, equipment.Partial
			}
			ended = true
			continue
		}
		if (len(f) != 16 && len(f) != 17) || f[0] != "T" || len(out) >= equipment.PPMaxRows {
			return nil, version, up, equipment.Partial
		}
		for i := 1; i < len(f); i++ {
			v, ok := ppUnescape(f[i])
			if !ok || len(v) > 256 {
				return nil, version, up, equipment.Partial
			}
			f[i] = v
		}
		if f[1] == "" || seen[f[1]] {
			return nil, version, up, equipment.Partial
		}
		seen[f[1]] = true
		item := equipment.PPTunnel{ID: f[1], Name: f[2], Login: f[3], Caller: f[4], Address: f[5], Uptime: ParsePPDuration(f[6]), RouterUptime: up}
		if ip, e := netip.ParseAddr(item.Address); e == nil {
			item.Address = ip.Unmap().String()
		} else {
			item.Address = ""
		}
		c := &item.Context
		c.Service = f[7]
		c.Interface = f[8]
		if n, e := strconv.Atoi(f[9]); e == nil && n >= 1 && n <= 4094 {
			c.VLAN = &n
		}
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
		c.SecretState = "unavailable"
		if len(f) == 17 {
			switch f[16] {
			case "active", "disabled", "ambiguous", "none", "unavailable":
				c.SecretState = f[16]
			default:
				return nil, version, up, equipment.Partial
			}
		}
		if c.SecretState == "active" {
			c.Profile = f[12]
		}
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
		out = append(out, item)
	}
	if scanner.Err() != nil || !ended {
		return nil, version, up, equipment.Partial
	}
	return out, version, up, equipment.OK
}
