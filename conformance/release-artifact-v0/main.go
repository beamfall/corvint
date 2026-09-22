// release-artifact-v0 is the offline reproducible-build gate for the Go
// candidate. It builds each declared target twice in one profile and proves the
// two outputs are byte-identical. It never publishes, uploads, signs, or tags.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, arguments []string, stdout, stderr *os.File) int {
	if len(arguments) != 0 && arguments[0] == "archive" {
		return runArchiveCLI(ctx, arguments[1:], stdout, stderr)
	}
	if len(arguments) != 0 && arguments[0] == "archive-status" {
		return runArchiveStatusCLI(arguments[1:], stdout, stderr)
	}
	if len(arguments) != 0 && arguments[0] == "publication-status" {
		return runPublicationStatusCLI(arguments[1:], stdout, stderr)
	}
	options, err := parse(arguments, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "release-artifact-v0:", err)
		return 2
	}
	report, err := verify(ctx, options)
	if err != nil {
		writeSummary(stdout, report)
		fmt.Fprintln(stderr, "release-artifact-v0:", err)
		return 2
	}
	if options.ReportPath != "" {
		if err := writeReport(options.ReportPath, report); err != nil {
			fmt.Fprintln(stderr, "release-artifact-v0:", err)
			return 2
		}
	}
	writeSummary(stdout, report)
	if report.Verdict != statusPass {
		printReasons(stderr, report)
		return 1
	}
	return 0
}

func printReasons(stderr *os.File, report Report) {
	for _, reason := range report.Reasons {
		if reason.Target != "" {
			fmt.Fprintf(stderr, "FAIL %s %s: %s\n", reason.Kind, reason.Target, reason.Detail)
			continue
		}
		fmt.Fprintf(stderr, "FAIL %s: %s\n", reason.Kind, reason.Detail)
	}
}

func parse(arguments []string, stdout *os.File) (Options, error) {
	flags := flag.NewFlagSet("release-artifact-v0", flag.ContinueOnError)
	flags.SetOutput(stdout)
	root := flags.String("root", ".", "repository root to build from")
	manifest := flags.String("manifest", "conformance/release-artifact-v0/manifest.json", "release profile manifest path")
	output := flags.String("output", "", "build and evidence directory; must be outside the repository")
	report := flags.String("report", "", "optional path for the full JSON report")
	if err := flags.Parse(arguments); err != nil {
		return Options{}, err
	}
	if flags.NArg() != 0 {
		return Options{}, fmt.Errorf("release-artifact-v0 accepts no positional arguments")
	}
	if *output == "" {
		return Options{}, fmt.Errorf("--output is required and must name a directory outside the repository")
	}
	absoluteRoot, err := filepath.Abs(*root)
	if err != nil {
		return Options{}, err
	}
	return Options{Root: absoluteRoot, ManifestPath: *manifest, Output: *output, ReportPath: *report, Writer: stdout}, nil
}
