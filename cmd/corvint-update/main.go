package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/update"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: corvint-update check|apply|rollback --component core|tasks [--allow-network] [--bin-dir PATH] [--state-dir PATH]")
		os.Exit(2)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	var c update.Config
	fs.StringVar(&c.Component, "component", "core", "core or tasks")
	fs.StringVar(&c.BinDir, "bin-dir", filepath.Join(home, ".local/bin"), "existing managed binary directory")
	fs.StringVar(&c.StateDir, "state-dir", filepath.Join(home, ".local/share/corvint/updates"), "private recovery state directory")
	fs.BoolVar(&c.AllowNetwork, "allow-network", false, "explicitly allow official release requests")
	fs.Parse(os.Args[2:])
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, err := update.Run(ctx, os.Args[1], c)
	json.NewEncoder(os.Stdout).Encode(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
