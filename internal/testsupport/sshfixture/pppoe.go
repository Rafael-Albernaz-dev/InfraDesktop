package sshfixture

import (
	"fmt"
	"infradesktop/internal/adapters/routerssh"
	"strings"
)

// Deterministic synthetic data only. This does not interpret the RouterOS script.
func (s *Server) ppOutput() string {
	n := s.config.PPRows
	if n == 0 {
		n = 32
	}
	tick := s.PPCalls.Add(1)
	version := "7.26"
	if s.config.Family == "v6" {
		version = "6.49.19"
	}
	return PPPoETranscript(n, version, int(tick))
}
func PPPoETranscript(n int, version string, tick int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "H\t1\t%d\t%s\t2d1h%ds\n", n, version, tick)
	if n > 2048 {
		b.WriteString("Z\t0\trows\n")
		return b.String()
	}
	for i := 0; i < n; i++ {
		name := fmt.Sprintf("synthetic-%04d", i/2)
		caller := fmt.Sprintf("02:00:00:00:%02x:%02x", i/256, i%256)
		fmt.Fprintf(&b, "R\t*%X\t%d\t%s\t198.51.%d.%d\t%s\t2h%ds\n", i+1, i+1, routerssh.PPPoEEscape(name), (i/250)%256, i%250+1, caller, tick)
	}
	fmt.Fprintf(&b, "Z\t%d\tcomplete\t2d1h%ds\n", n, tick)
	return b.String()
}
