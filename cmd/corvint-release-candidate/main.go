// Command corvint-release-candidate assembles already-qualified retained
// artifacts into one immutable local candidate. It never publishes or installs.
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
	flags := flag.NewFlagSet("corvint-release-candidate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	core := flags.String("core-dir", "", "verified core archive-gate directory")
	companion := flags.String("companion-dir", "", "verified companion retained directory (omit for a Core-only candidate)")
	source := flags.String("source-root", "", "Corvint Git checkout containing the exact built commit")
	scratch := flags.String("scratch", "", "private scratch directory")
	output := flags.String("output-parent", "", "parent for the fresh versioned candidate")
	version := flags.String("version", "", "exact release version MAJOR.MINOR.PATCH, MAJOR.MINOR.PATCH-rc.N or MAJOR.MINOR.PATCHaN, without a v prefix")
	if err := flags.Parse(arguments); err != nil {
		return 2
	}
	if flags.NArg() != 0 || *core == "" || *source == "" || *scratch == "" || *output == "" || *version == "" {
		fmt.Fprintln(stderr, "corvint-release-candidate: -core-dir, -source-root, -scratch, -output-parent, and -version are required; positional arguments are forbidden")
		return 2
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	result, err := releasecandidate.Assemble(ctx, releasecandidate.Options{CoreDirectory: *core, CompanionDirectory: *companion, SourceRoot: *source, Scratch: *scratch, OutputParent: *output, Version: *version})
	if err != nil {
		fmt.Fprintln(stderr, "corvint-release-candidate:", err)
		return 1
	}
	fmt.Fprintf(stdout, "candidate: %s\nprofile: %s\nversion: %s\ncommit: %s\ntree: %s\npublication: NOT_RUN\n", result.Directory, result.Manifest.Profile, result.Manifest.CorvintVersion, result.Manifest.Sources[0].Commit, result.Manifest.Sources[0].Tree)
	return 0
}
