// Isolated loopback fixture, never linked into the daemon or desktop.
package main

import (
	"encoding/json"
	"flag"
	"infradesktop/internal/testsupport/sshfixture"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	family := flag.String("family", "v7", "test output family")
	mode := flag.String("mode", "", "isolated test response mode")
	flag.Parse()
	if *mode != "" && *mode != "interfaces" {
		os.Exit(2)
	}
	server, e := sshfixture.Start(sshfixture.Config{Family: *family, Mode: *mode})
	if e != nil {
		os.Exit(1)
	}
	defer server.Close()
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"port": server.Port(), "fingerprint": server.Fingerprint(), "scope": "isolated loopback fixture"})
	signalc := make(chan os.Signal, 1)
	signal.Notify(signalc, syscall.SIGTERM, syscall.SIGINT)
	<-signalc
	server.Close()
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"authentications": server.Auth.Load(), "commands": server.Commands.Load(), "scope": "isolated loopback fixture final counters"})
}
