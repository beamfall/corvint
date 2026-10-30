// Command corvint-web-flows is an explicitly invoked experimental browser companion.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Beamfall/corvint/internal/appflows"
)

func run(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("corvint-web-flows", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	root := f.String("root", ".", "Git application root")
	manifest := f.String("manifest", "", "tracked relative intent JSON path")
	assets := f.String("assets", "", "installed tools/web-flows directory")
	live := f.Bool("observe", false, "explicitly start owned local server and browser")
	experimental := f.Bool("experimental", false, "enable the proposed experimental profile")
	trusted := f.Bool("trusted-local", false, "server and runtime are trusted local code")
	flows := f.String("flows", "", "committed navigation flows directory")
	packet := f.String("navigation", "", "native evidence-free navigation packet")
	execution := f.String("execution", "", "committed experimental execution input")
	fixtures := f.String("fixtures", "", "private fixture ID to value JSON")
	maxEffect := f.String("max-effect", appflows.EffectRead, "independent maximum effect grant")
	timeout := f.Duration("timeout", 5*time.Minute, "execution deadline, at most five minutes")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if !*experimental || !*trusted || *manifest == "" || *assets == "" || f.NArg() != 0 {
		fmt.Fprintln(diagnostic, "requires --experimental --trusted-local --manifest FILE --assets DIR; --observe opts into execution")
		return 2
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		fmt.Fprintln(diagnostic, err)
		return 2
	}
	navigationFlag := false
	f.Visit(func(option *flag.Flag) {
		switch option.Name {
		case "flows", "navigation", "execution", "fixtures", "max-effect", "timeout":
			navigationFlag = true
		}
	})
	if navigationFlag {
		if !*live || *packet == "" || *flows == "" || *execution == "" {
			fmt.Fprintln(diagnostic, "navigation requires --observe --flows DIR --navigation FILE --execution FILE")
			return 2
		}
		receipt, err := appflows.ObserveNavigation(ctx, abs, appflows.NavigationOptions{Manifest: *manifest, Assets: *assets, Flows: *flows, Packet: *packet, Execution: *execution, Fixtures: *fixtures, MaxEffect: *maxEffect, Timeout: *timeout})
		if err != nil {
			fmt.Fprintln(diagnostic, err)
			return 2
		}
		if json.NewEncoder(out).Encode(receipt) != nil {
			return 2
		}
		if receipt.Status != "passed" {
			return 1
		}
		return 0
	}
	e, err := appflows.Observe(ctx, abs, *manifest, *assets, *live)
	if err != nil {
		fmt.Fprintln(diagnostic, err)
		return 2
	}
	if err = json.NewEncoder(out).Encode(e); err != nil {
		return 2
	}
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
