package sshfixture

import (
	"infradesktop/internal/adapters/routerssh"
	"regexp"
	"strings"
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

var fixturePingSuffix = regexp.MustCompile(`/tool ping address=(198\.51\.[0-9]+\.[0-9]+) count=3 interval=200ms size=64 ttl=64 (vrf|routing-table)=main$`)

func (s *Server) diagnosticOutput(command string) (string, bool) {
	family, version := "v7", "7.26"
	if s.config.Family == "v6" {
		family = "v6"
		version = "6.49.19"
	}
	switch command {
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
	if match := fixturePingSuffix.FindStringSubmatch(command); match != nil && strings.HasPrefix(command, ":if (") && strings.Contains(command, "session-changed") && strings.Contains(command, "context-unavailable") {
		expected := "vrf"
		if family == "v6" {
			expected = "routing-table"
		}
		if match[2] != expected {
			return "", false
		}
		s.PingCalls.Add(1)
		return PingTranscript(match[1]), true
	}
	return "", false
}
