// Command corvint-companion-release assembles the optional companion
// distribution bundle for a single pinned target (darwin/arm64 only; see
// docs/specs/public-release-v0.md). It never mutates its input checkout:
// -source-root must already be clean, and every build/assembly step runs
// under -scratch. The corvint-tasks companion builds from the same checkout
// (decision 0397). On any failure it retains no
// output and exits non-zero.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/Beamfall/corvint/internal/companionrelease"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("corvint-companion-release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	sourceRoot := fs.String("source-root", "", "clean Corvint source checkout (required)")
	target := fs.String("target", "darwin/arm64", "GOOS/GOARCH pair to build (only darwin/arm64 is supported)")
	scratch := fs.String("scratch", "", "scratch working directory (required)")
	outputParent := fs.String("output-parent", "", "directory the retained bundle is placed under (required; must be outside -source-root)")
	bundleName := fs.String("bundle-name", "", "name of the retained bundle directory under -output-parent (required)")
	npmCache := fs.String("npm-cache", "", "retained compatibility argument; unused by core bundle")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *sourceRoot == "" || *scratch == "" || *outputParent == "" || *bundleName == "" {
		fmt.Fprintln(stderr, "corvint-companion-release: -source-root, -scratch, -output-parent, and -bundle-name are all required")
		return 2
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	report, err := companionrelease.Run(ctx, companionrelease.Options{
		CorvintRoot:  *sourceRoot,
		Target:       *target,
		Scratch:      *scratch,
		OutputParent: *outputParent,
		BundleName:   *bundleName,
		NPMCache:     *npmCache,
	})
	if err != nil {
		fmt.Fprintf(stderr, "corvint-companion-release: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "companion bundle retained at %s\n", report.BundlePath)
	fmt.Fprintf(stdout, "archive: %s sha256=%s\n", report.ArchivePath, report.ArchiveSHA256)
	fmt.Fprintf(stdout, "smoke report: %s\n", report.SmokeReportPath)
	fmt.Fprintf(stdout, "target: %s\n", report.Target)
	fmt.Fprintf(stdout, "go: %s   git: %s\n", report.Toolchain.GoVersion, report.Toolchain.GitVersion)
	for _, c := range report.Manifest.Components {
		fmt.Fprintf(stdout, "  %-28s sha256=%s commit=%s\n", c.Name, c.BinarySHA256, c.Commit)
	}
	fmt.Fprintf(stdout, "not run (no attempt made): %v\n", report.NotRun)
	fmt.Fprintf(stdout, "smoke steps:\n")
	for _, s := range report.SmokeSteps {
		status := "ok"
		if !s.OK {
			status = "FAILED"
		}
		fmt.Fprintf(stdout, "  [%s] %s %s\n", status, s.Name, s.Detail)
		if s.InvokedPath != "" {
			fmt.Fprintf(stdout, "       bundle=%s component=%s source=%s tree=%s path=%s\n", s.BundleSHA256, s.ComponentSHA256, s.SourceCommit, s.SourceTree, s.InvokedPath)
		}
	}
	fmt.Fprintln(stdout, "browser/UI qualification is a separate step and is not claimed by this build")
	return 0
}
