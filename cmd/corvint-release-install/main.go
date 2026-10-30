// Command corvint-release-install installs one verified host core archive into
// a fresh version/platform path. It never changes a current/latest selector.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/releasecandidate"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

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
