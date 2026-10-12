// Command cem-convert-parity retains the CEM-V1-104 verdict-parity evidence for
// the cem/0.x -> cem/1.0 converter (V1-1275, decision 0525 §4). For every
// committed `.corvint/changes/<bind>.cem.json` it runs the 0.x native verifier
// on the source, converts it twice (byte equality), and runs the stable native
// verifier and the portable interop/cem01-go consumer on the converted map. It
// writes one JSONL row per map and a summary.
//
// Usage:
//
//	cem-convert-parity --repository CLONE --maps DIR --portable CEM01GO
//	                   --reachable-revs FILE --out DIR [--jobs N] [--every N]
//	                   [--migrated]
//
// --migrated is the V1-1277 migration check over maps already rewritten to
// cem/1.0: each converted map must carry converter provenance, its named source
// blob must exist with the recorded digests, and reconverting that blob must
// reproduce the committed bytes exactly; the parity run then verifies the
// committed bytes at the substituted target. A stable map without provenance
// (natively produced) is verified at its bind commit on both consumers. It also
// writes targets.txt ("<bind> <target>" lines) for cem-citation-baseline.
//
// CLONE is an absolute scratch `git clone --local --no-checkout` of the
// repository: the stable consumers refuse object alternates, and the
// substituted-sidecar targets are written only into it. FILE lists the commits
// reachable from the published history (one per line, `git rev-list
// origin/main`); it only labels rows.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var o Options
	flag.StringVar(&o.Repository, "repository", "", "absolute scratch clone")
	flag.StringVar(&o.Maps, "maps", "", "absolute .corvint/changes directory")
	flag.StringVar(&o.Portable, "portable", "", "absolute interop/cem01-go executable")
	flag.StringVar(&o.ReachableRevs, "reachable-revs", "", "file of commits reachable from the published history")
	flag.StringVar(&o.Out, "out", "", "absolute evidence directory")
	flag.IntVar(&o.Jobs, "jobs", 4, "parallel maps")
	flag.IntVar(&o.Every, "every", 1, "deterministic sample: every Nth map in byte order")
	flag.BoolVar(&o.Migrated, "migrated", false, "check committed cem/1.0 maps against their provenance-named source blobs")
	flag.Parse()
	for _, p := range []string{o.Repository, o.Maps, o.Portable, o.Out} {
		if !filepath.IsAbs(p) {
			fmt.Fprintln(os.Stderr, "cem-convert-parity: --repository, --maps, --portable and --out must be absolute")
			os.Exit(2)
		}
	}
	summary, err := Run(o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "cem-convert-parity:", err)
		os.Exit(1)
	}
	fmt.Printf("%s\n", summary)
}
