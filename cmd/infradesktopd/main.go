package main

import (
	"context"
	"flag"
	"fmt"
	"infradesktop/internal/composition"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	runtime := flag.String("runtime", composition.RuntimePath(), "private existing absolute runtime directory (0700)")
	state := flag.String("state", composition.StatePath(), "private existing durable state directory (0700)")
	version := flag.Bool("version", false, "print product version")
	flag.Parse()
	if *version {
		fmt.Println("InfraDesktop 0.4.0")
		return
	}
	os.Setenv("INFRADESKTOP_STATE", *state)
	if os.Getuid() == 0 {
		fmt.Fprintln(os.Stderr, "daemon requires a non-root technician UID")
		os.Exit(1)
	}
	server, e := composition.Daemon(*runtime)
	if e != nil {
		fmt.Fprintln(os.Stderr, "cannot open private runtime endpoint")
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()
	fmt.Println("InfraDesktop local service ready")
	if e = server.Serve(ctx); e != nil {
		fmt.Fprintln(os.Stderr, "daemon transport stopped")
		os.Exit(1)
	}
}
