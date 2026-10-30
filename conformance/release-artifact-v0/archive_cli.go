package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type ArchiveOptions struct {
	Root         string
	ManifestPath string
	Revision     string
	Output       string
	Writer       io.Writer
}

func runArchiveCLI(ctx context.Context, arguments []string, stdout, stderr *os.File) int {
	options, err := parseArchive(arguments, stdout)
	if err != nil {
		fmt.Fprintln(stderr, "release-artifact-v0 archive:", err)
		return 2
	}
	report, runErr := buildArchives(ctx, options)
	witnessErr := writeArchiveWitnessBestEffort(options.Root, report.witness())
	if witnessErr != nil {
		fmt.Fprintln(stderr, "release-artifact-v0 archive: private witness not recorded:", witnessErr)
	}
	if runErr != nil {
		fmt.Fprintln(stderr, "release-artifact-v0 archive:", runErr)
		return 1
	}
	fmt.Fprintf(stdout, "SUMMARY revision=%s targets=%d verdict=%s output=%s\n", report.Revision, len(report.Targets), report.Verdict, filepath.Base(options.Output))
	return 0
}

func parseArchive(arguments []string, stdout *os.File) (ArchiveOptions, error) {
	flags := flag.NewFlagSet("release-artifact-v0 archive", flag.ContinueOnError)
	flags.SetOutput(stdout)
	root := flags.String("root", ".", "repository root")
	manifest := flags.String("manifest", "conformance/release-artifact-v0/manifest.json", "pinned manifest")
	revision := flags.String("revision", "HEAD", "exact clean commit")
	output := flags.String("output", "", "new retained directory under an existing owner-only parent")
	if err := flags.Parse(arguments); err != nil {
		return ArchiveOptions{}, err
	}
	if flags.NArg() != 0 || *output == "" {
		return ArchiveOptions{}, fmt.Errorf("--output is required and positional arguments are forbidden")
	}
	absoluteRoot, err := filepath.Abs(*root)
	if err != nil {
		return ArchiveOptions{}, err
	}
	absoluteOutput, err := filepath.Abs(*output)
	if err != nil {
		return ArchiveOptions{}, err
	}
	return ArchiveOptions{Root: absoluteRoot, ManifestPath: *manifest, Revision: *revision, Output: absoluteOutput, Writer: stdout}, nil
}
