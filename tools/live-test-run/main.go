// Command live-test-run runs the live test tier (AFP-V0-2700, AFP-V0-2701): the tests that read the
// live repository or build release binaries and therefore skip in PR shards. It discovers the tier
// with cishards.LiveTests and runs, per package, `go test -tags live -count=1 -timeout T -run
// '^(A|B)$' ./dir` from the repository root, with the output passed through. `make live-test`
// runs it in the gate, so nightly release gates and the merge queue cover the tier. --list prints
// the tier without running it. It exits 2 when the tier cannot be read or is empty, 1 when any
// package fails, and 0 otherwise.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"

	cishards "github.com/Beamfall/corvint/.github/cishards"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, cishards.LiveTests, goTest))
}

// lister lists the live tier of the repository at a root.
type lister func(root string) ([]cishards.LivePackage, error)

// runner runs one go test argv from the repository root and reports whether it passed.
type runner func(argv []string, stdout, stderr io.Writer) bool

func goTest(argv []string, stdout, stderr io.Writer) bool {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, stdout, stderr
	return cmd.Run() == nil
}

func run(args []string, stdout, stderr io.Writer, listTier lister, runOne runner) int {
	flags := flag.NewFlagSet("live-test-run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	timeout := flags.String("timeout", "30m", "go test -timeout for each package")
	list := flags.Bool("list", false, "print the live tier and exit")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "live-test-run: unexpected arguments %q\n", flags.Args())
		return 2
	}
	packages, err := listTier(".")
	if err != nil {
		fmt.Fprintln(stderr, "live-test-run: "+err.Error())
		return 2
	}
	if len(packages) == 0 {
		fmt.Fprintln(stderr, "live-test-run: the live tier is empty; no test calls testsupport.RequireLiveTier")
		return 2
	}
	failed := 0
	for _, p := range packages {
		if *list {
			fmt.Fprintf(stdout, "%s\t%s\n", p.Dir, strings.Join(p.Tests, " "))
			continue
		}
		if !runOne(argv(p, *timeout), stdout, stderr) {
			failed++
		}
	}
	if failed > 0 {
		fmt.Fprintf(stderr, "live-test-run: %d of %d live-tier packages failed\n", failed, len(packages))
		return 1
	}
	return 0
}

func argv(p cishards.LivePackage, timeout string) []string {
	names := make([]string, len(p.Tests))
	for i, name := range p.Tests {
		names[i] = regexp.QuoteMeta(name)
	}
	return []string{"go", "test", "-tags", "live", "-count=1", "-timeout", timeout,
		"-run", "^(" + strings.Join(names, "|") + ")$", "./" + p.Dir}
}
