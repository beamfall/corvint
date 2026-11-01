package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/Beamfall/corvint/internal/companioncli"
	"github.com/Beamfall/corvint/internal/update"
)

// usage is the companion contract's --help text (CAP-V0-700).
const usage = `usage: corvint-update check|apply|rollback --component core|tasks [--allow-network] [--github-token-env NAME]
                      [--bin-dir PATH] [--state-dir PATH] [--refresh-index ROOT]... [--restart-service PROGRAM]
                      [--post-switch CMD]... [--hook-timeout D] [--hook-attempts N] [--drain-timeout D] [--start-timeout D]
check reports whether a newer official release of the component exists; apply installs it with an atomic
switch and keeps the previous binaries for rollback; rollback restores them. No request leaves the host
without --allow-network.
Example: corvint-update check --component core --allow-network`

func main() {
	if code, handled := companioncli.Answer(companioncli.Spec{Binary: "corvint-update", Version: companioncli.ExperimentalVersion, Usage: usage}, os.Args[1:], os.Stdout); handled {
		os.Exit(code)
	}
	// CAP-V0-701: an unknown action is refused before any state is read.
	if len(os.Args) < 2 || (os.Args[1] != "check" && os.Args[1] != "apply" && os.Args[1] != "rollback") {
		fmt.Fprintln(os.Stderr, usage)
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
	fs.StringVar(&c.TokenEnv, "github-token-env", "", "authenticate release API requests to api.github.com with the token in this environment variable (never sent to download hosts)")
	fs.Func("refresh-index", "after a Core switch, also refresh this checkout's index snapshot (repeatable)", func(root string) error {
		c.RefreshRoots = append(c.RefreshRoots, root)
		return nil
	})
	fs.StringVar(&c.ServiceProgram, "restart-service", "", "apply: service-control executable run as PROGRAM status|stop|start|live-attempts around the switch")
	fs.Func("post-switch", "apply: shell command run after a switch that stands (repeatable)", func(command string) error {
		c.PostSwitch = append(c.PostSwitch, command)
		return nil
	})
	fs.DurationVar(&c.HookTimeout, "hook-timeout", 0, "apply: per-attempt post-switch hook bound (default 2m, at most 10m)")
	fs.IntVar(&c.HookAttempts, "hook-attempts", 0, "apply: attempts per post-switch hook (default 3, at most 5)")
	fs.DurationVar(&c.DrainTimeout, "drain-timeout", 0, "apply: wait for zero live attempts when formats differ (default 10m, at most 15m)")
	fs.DurationVar(&c.StartTimeout, "start-timeout", 0, "apply: wait for the started service to report running (default 30s, at most 2m)")
	fs.Parse(os.Args[2:])
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "unexpected positional arguments")
		os.Exit(2)
	}
	c.WorkDir, _ = os.Getwd()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	r, err := update.Run(ctx, os.Args[1], c)
	json.NewEncoder(os.Stdout).Encode(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
