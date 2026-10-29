package sshfixture

import (
	"fmt"
	"infradesktop/internal/adapters/routerssh"
	"regexp"
	"strings"
	"time"
)

// Fabricated fixtures for isolated integration/native journeys only. Recognizing
// a known command template does not interpret RouterOS or homologate firmware.
func CatalogTranscript(version string) string {
	return "D\t1\tcatalog\t" + version + "\t2d1h1s\t7\nI\t*1\tfixture-ether\tether\ttrue\tfalse\tfalse\nI\t*2\tfixture-outer\tvlan\ttrue\tfalse\tfalse\nI\t*3\tfixture-inner\tvlan\ttrue\tfalse\tfalse\nV\t*2\tfixture-outer\tfixture-ether\t100\ttrue\ttrue\tfalse\nV\t*3\tfixture-inner\tfixture-outer\t200\tfalse\ttrue\tfalse\nS\t*1\tfixture-service-a\tfixture-inner\tfalse\tfalse\t100-115,120\ttrue\nS\t*2\tfixture-service-b\tfixture-inner\tfalse\tfalse\t?\t?\nZ\t7\tcomplete\t2d1h1s\n"
}
func RadiusTranscript(version string) string {
	return "D\t1\tradius\t" + version + "\t2d1h1s\t2\nA\ttrue\ttrue\nR\t*1\t198.51.100.20\tppp\tfalse\t1812\t1813\tudp\t0\t100\t90\t3\t2\t1\t0\t1ms45us\nZ\t2\tcomplete\t2d1h1s\n"
}
func PingTranscript(address string) string {
	return "SEQ HOST SIZE TTL TIME STATUS\n0 " + address + " 64 64 1ms45us\n1 " + address + " 64 64 1ms89us\n2 " + address + " 64 64 1ms163us\nsent=3 received=3 packet-loss=0% min-rtt=1ms45us avg-rtt=1ms99us max-rtt=1ms163us\n"
}

var fixturePingSuffix = regexp.MustCompile(`(/ping|/tool ping) address=(198\.51\.[0-9]+\.[0-9]+) count=3 interval=200ms size=64 ttl=64 (vrf|routing-table)=main\}\} on-error=\{:put "E\\tsession"\}\}$`)

func (s *Server) diagnosticOutput(command string) (string, bool) {
	family, version := "v7", "7.26"
	if s.config.Family == "v6" {
		family = "v6"
		version = "6.49.19"
	}
	switch command {
	case routerssh.DeviceFastTrackCommand:
		return "T3F\t1\t" + version + "\nF\t1\nD\t00:00:00\nZ\tcomplete\n", true
	case routerssh.DevicePortsCommand:
		return "T3E\t1\t" + version + "\nE\tfixture-uplink\tlink-ok\t100Mbps\ttrue\t2\nD\t00:00:00\t10\t10\nZ\t1\tcomplete\n", true
	case routerssh.DeviceOperationalCommand:
		return DeviceOperationalTranscript(version), true
	case routerssh.DeviceAuthFailuresCommand:
		return "A\t1\t" + version + "\t2d1h1s\nF\tsynthetic-0000\t01:00:00\nF\tsynthetic-0000\t01:00:01\nG\tauth\t0\nZ\t2\tcomplete\t2d1h1s\n", true

	case routerssh.PPHealthFirstCommand, routerssh.PPHealthCommand:
		tick := int(time.Since(s.started)/time.Second) + 1
		pools := "false"
		n := 1
		var b strings.Builder
		if command == routerssh.PPHealthCommand {
			pools = "true"
			n = 2
		}
		fmt.Fprintf(&b, "N\t1\t%s\t2d1h%ds\t%s\ttrue\n", version, tick, pools)
		if pools == "true" {
			b.WriteString("P\t*1\tfixture-pool\t198.51.100.1-198.51.100.3\t0\n")
		}
		fmt.Fprintf(&b, "I\t*1\tfixture-ether\tether\t2026-10-27 00:00:00\t%d\t%d\t0\t?\t0\t?\ttrue\tfalse\nZ\t%d\tcomplete\t2d1h%ds\n", tick*1024, tick*2048, n, tick)
		return b.String(), true
	case routerssh.PPContextCommand:
		return "C\t1\t" + version + "\t2d1h1s\nZ\t0\tcomplete\t2d1h1s\n", true
	case routerssh.PDCatalogCommand:
		return CatalogTranscript(version), true
	case routerssh.PDRadiusCommand:
		return RadiusTranscript(version), true
	case routerssh.PDPingContextV6:
		if family != "v6" {
			return "", false
		}
		return "C\tv6\t0\t0\t0\n", true
	case routerssh.PDPingContextV7:
		if family != "v7" {
			return "", false
		}
		return "C\tv7\t0\t0\t0\n", true
	}
	if strings.Contains(command, `:put ("B\t1\t"`) && strings.HasPrefix(command, ":local esc do=") {
		tick := int(time.Since(s.started)/time.Second) + 1
		n := s.config.PPRows
		if n == 0 {
			n = 32
		}
		var b strings.Builder
		fmt.Fprintf(&b, "B\t1\t%s\t2d1h%ds\n", version, tick)
		for i := 0; i < n; i++ {
			fmt.Fprintf(&b, "T\t*%X\ttunnel-%d\tsynthetic-%04d\t02:00:00:00:%02x:%02x\t198.51.%d.%d\t2h%ds\t%d\t%d\n", i+100, i, i/2, i/256, i%256, (i/250)%256, i%250+1, tick, tick*(i+1)*1024, tick*(i+1)*2048)
		}
		fmt.Fprintf(&b, "Z\t%d\tcomplete\t2d1h%ds\n", n, tick)
		return b.String(), true
	}
	if strings.Contains(command, `:put ("L\t1\t"`) && strings.Contains(command, `/log print count-only where topics~"ppp"`) {
		return "L\t1\t" + version + "\t2d1h1s\t2\nE\t01:00:00\tauth_failed\nE\t01:00:01\tdisconnected\nZ\t2\tcomplete\t2d1h1s\n", true
	}
	if match := fixturePingSuffix.FindStringSubmatch(command); match != nil && strings.HasPrefix(command, ":if (") && strings.Contains(command, `:put "E\tsession"`) && strings.Contains(command, `:put "E\tcontext"`) {
		expected, pingCommand := "vrf", "/tool ping"
		if family == "v6" {
			expected = "routing-table"
			pingCommand = "/ping"
		}
		if match[1] != pingCommand || match[3] != expected {
			return "", false
		}
		s.PingCalls.Add(1)
		return PingTranscript(match[2]), true
	}
	return "", false
}

func DeviceOperationalTranscript(version string) string {
	return "O\t1\t" + version + "\t2d1h1s\nR\tcpu-load\t=10\nR\tfree-memory\t=67108864\nR\ttotal-memory\t=134217728\nR\tuptime\t=2d1h1s\nR\tversion\t=" + version + "\nR\ttemperature\t?\nR\terror-count\t=2\nC\t=oct/28/2026\t=01:00:00\t=4294956496\t=true\t?\nS\ttelnet\t23\tfalse\nS\tftp\t21\ttrue\nS\twww\t80\tfalse\nG\tservices\tcomplete\nK\tfixture-package\t" + version + "\tfalse\nG\tpackages\tcomplete\nI\tfixture-uplink\tether\t0\t?\t2\nI\tfixture-vlan\tvlan\t3\t0\t?\nG\tinterfaces\tcomplete\nP\tfixture-pool\t198.51.100.1-198.51.100.10\t8\nG\tpools\tcomplete\nZ\t7\tcomplete\t2d1h1s\n"
}
