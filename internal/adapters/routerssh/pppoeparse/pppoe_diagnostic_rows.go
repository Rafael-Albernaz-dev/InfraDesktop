package pppoeparse

import (
	"infradesktop/internal/adapters/routerssh/internal/routeros"
	"infradesktop/internal/modules/equipment"
	"net/netip"
	"strconv"
	"strings"
)

func parsePDRow(scope string, f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	if len(f) < 2 {
		return r, false
	}
	if scope == equipment.PDCatalog {
		return parsePDCatalogRow(f)
	}
	if scope == equipment.PDRadius {
		return parsePDRadiusKind(f)
	}
	return r, false
}

func parsePDCatalogRow(f []string) (equipment.PDRow, bool) {
	switch f[0] {
	case "I":
		if len(f) == 7 {
			return parsePDInterfaceRow(f)
		}
	case "V":
		if len(f) == 8 {
			return parsePDVLANRow(f)
		}
	case "S":
		if len(f) == 8 {
			return parsePDServiceRow(f)
		}
	}
	return equipment.PDRow{}, false
}

func parsePDRadiusKind(f []string) (equipment.PDRow, bool) {
	if f[0] == "A" && len(f) == 3 {
		return parsePDAAARow(f)
	}
	if f[0] == "R" && len(f) == 16 {
		return parsePDRadiusRow(f)
	}
	return equipment.PDRow{}, false
}

func parsePDInterfaceRow(f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	ok := true

	r.Kind = "interface"
	r.RemoteID = f[1]
	r.Name = f[2]
	r.Type = f[3]
	r.Running, ok = routeros.Bool(f[4], false)
	if !ok {
		return r, false
	}
	r.Disabled, ok = routeros.Bool(f[5], false)
	if !ok {
		return r, false
	}
	r.Dynamic, ok = routeros.Bool(f[6], false)
	return r, ok && pdRemoteIdentity(r.RemoteID, true)
}

func parsePDVLANRow(f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	ok := true

	r.Kind = "vlan"
	r.RemoteID = f[1]
	r.Name = f[2]
	r.Parent = f[3]
	n, valid := pdNumber(f[4], 4094)
	if !valid || n == 0 {
		return r, false
	}
	r.VLANID = int(n)
	r.ServiceTag, ok = routeros.Bool(f[5], false)
	if !ok {
		return r, false
	}
	r.Running, ok = routeros.Bool(f[6], false)
	if !ok {
		return r, false
	}
	r.Disabled, ok = routeros.Bool(f[7], false)
	return r, ok && pdRemoteIdentity(r.RemoteID, true)
}

func parsePDServiceRow(f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	ok := true

	r.Kind = "service"
	r.RemoteID = f[1]
	r.Name = f[2]
	r.Parent = f[3]
	r.Disabled, ok = routeros.Bool(f[4], false)
	if !ok {
		return r, false
	}
	r.Invalid, ok = routeros.Bool(f[5], false)
	if !ok {
		return r, false
	}
	r.Range, ok = ParsePDRange(f[6])
	if !ok {
		return r, false
	}
	r.AcceptUntagged, ok = routeros.Bool(f[7], true)
	return r, ok && pdRemoteIdentity(r.RemoteID, true)
}

func parsePDAAARow(f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	ok := true

	r.Kind = "aaa"
	r.UseRadius, ok = routeros.Bool(f[1], false)
	if !ok {
		return r, false
	}
	r.Accounting, ok = routeros.Bool(f[2], false)
	return r, ok
}

func parsePDRadiusRow(f []string) (equipment.PDRow, bool) {
	r := equipment.PDRow{}
	ok := true

	r.Kind = "radius"
	r.RemoteID = f[1]
	r.Address = f[2]
	r.Services = strings.ReplaceAll(f[3], ";", ",")
	r.Protocol = f[7]
	r.Services = normalizePDRadiusServices(r.Services)
	// ignored: validate the scalar address without retaining its parsed representation
	if _, err := netip.ParseAddr(strings.Split(f[2], "@")[0]); err != nil {
		return r, false
	}
	if r.Protocol != "?" && r.Protocol != "udp" && r.Protocol != "radsec" {
		return r, false
	}
	r.Disabled, ok = routeros.Bool(f[4], false)
	if !ok {
		return r, false
	}
	a, valid := pdNumber(f[5], 65535)
	if !valid || a == 0 {
		return r, false
	}
	b, valid := pdNumber(f[6], 65535)
	if !valid || b == 0 {
		return r, false
	}
	r.AuthPort = int(a)
	r.AccountingPort = int(b)
	c := &equipment.PDRadiusCounters{}
	values := []*string{&c.Pending, &c.Requests, &c.Accepts, &c.Rejects, &c.Resends, &c.Timeouts, &c.BadReplies}
	for i, p := range values {
		n, valid := pdNumber(f[i+8], ^uint64(0))
		if !valid {
			return r, false
		}
		*p = strconv.FormatUint(n, 10)
	}
	c.LastRTT = ParsePDRTTInterval(f[15])
	r.Counters = c
	return r, ok && pdRemoteIdentity(r.RemoteID, true)
}

func normalizePDRadiusServices(services string) string {
	allowedService := map[string]bool{"ppp": true, "login": true, "hotspot": true,
		"wireless": true, "dhcp": true, "ipsec": true, "dot1x": true}
	if services != "" {
		for _, service := range strings.Split(services, ",") {
			if !allowedService[service] {
				services = "desconhecido"
				break
			}
		}
	}
	return services
}
