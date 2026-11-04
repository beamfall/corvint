// Command corvint-release-install installs one verified host core archive into
// a fresh version/platform path. It never changes a current/latest selector.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Beamfall/corvint/internal/cliapp"
	"github.com/Beamfall/corvint/internal/clierror"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/releasecandidate"
)

// usage is the source-owned synopsis for offline discovery.
const usage = `usage: corvint-release-install --candidate DIR --store DIR`

func main() {
	streams := &clierror.Streams{Binary: "corvint-release-install", Out: os.Stdout, Err: os.Stderr}
	defer streams.Finish()

	if code, handled := cliapp.Answer("corvint-release-install", os.Args[1:], streams.Stdout(), streams.Stderr()); handled {
		os.Exit(streams.Status(code))
	}
	os.Exit(streams.Status(run(os.Args[1:], streams.Stdout(), streams.Stderr())))
}

func run(arguments []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("corvint-release-install", flag.ContinueOnError)
	flags.SetOutput(stderr)
	candidate := flags.String("candidate", "", "verified qualified candidate directory")
	store := flags.String("store", "", "versioned install store")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *candidate == "" || *store == "" {
		fmt.Fprintln(stderr, "corvint-release-install: -candidate and -store are required; positional arguments are forbidden")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	path, err := releasecandidate.InstallCore(ctx, *candidate, *store)
	if err != nil {
		fmt.Fprintln(stderr, "corvint-release-install:", err)
		return 1
	}
	fmt.Fprintln(stdout, path)
	return 0
}
