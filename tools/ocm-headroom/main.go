// Command ocm-headroom warns when a spec's ## Requirements section nears the 256-obligation OCM
// bound that `ocm prepare` enforces (docs/specs/corvint-self-development-v0.md, ASD-V0-572;
// V1-3438). For each docs/specs/*.md whose count reaches the threshold it prints
//
//	ocm-headroom: warning: docs/specs/X.md holds 241 of 256 OCM obligations (warn at 240); define new requirements in a section of their own or a companion spec
//
// and always exits 0: the hard refusal stays TestSpecsStayWithinTheOCMObligationBound. The
// threshold is --warn-at N, or the OCM_HEADROOM_WARN_AT environment value, default 240.
// Exit 2 on a usage error or an unreadable spec directory.
//
// usage: go run ./tools/ocm-headroom [--warn-at N] [--dir docs/specs]
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Beamfall/corvint/internal/lrfrepo"
)

const defaultWarnAt = 240

func main() {
	os.Exit(run(os.Args[1:], os.Getenv("OCM_HEADROOM_WARN_AT"), os.Stdout, os.Stderr))
}

func run(args []string, envWarnAt string, stdout, stderr io.Writer) int {
	warnAt := defaultWarnAt
	if envWarnAt != "" {
		n, err := strconv.Atoi(envWarnAt)
		if err != nil || n < 1 {
			fmt.Fprintf(stderr, "ocm-headroom: OCM_HEADROOM_WARN_AT=%q is not a positive integer\n", envWarnAt)
			return 2
		}
		warnAt = n
	}
	fs := flag.NewFlagSet("ocm-headroom", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.IntVar(&warnAt, "warn-at", warnAt, "warn when a spec holds at least this many obligations")
	dir := fs.String("dir", "docs/specs", "spec directory")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || warnAt < 1 {
		fmt.Fprintln(stderr, "usage: ocm-headroom [--warn-at N] [--dir docs/specs]")
		return 2
	}
	paths, err := filepath.Glob(filepath.Join(*dir, "*.md"))
	if err != nil || len(paths) == 0 {
		fmt.Fprintf(stderr, "ocm-headroom: no specs under %s\n", *dir)
		return 2
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "ocm-headroom: %v\n", err)
			return 2
		}
		n, err := lrfrepo.IntentObligationCount(data)
		if err != nil || n < warnAt {
			continue
		}
		fmt.Fprintf(stdout, "ocm-headroom: warning: %s holds %d of %d OCM obligations (warn at %d); define new requirements in a section of their own or a companion spec\n", filepath.ToSlash(path), n, lrfrepo.MaxOCMObligations, warnAt)
	}
	return 0
}
