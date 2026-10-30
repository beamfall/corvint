// Command gate-affected-select is the selection step of script/gate-affected.sh
// (AFP-V0-011, AFP-V0-012). It prints internal/gateselect's audit lines and
// verdict for one affected-plan receipt; the package comment there documents
// the line shapes.
//
// Usage: gate-affected-select PLAN MODULE ROOT, where ROOT is the repository
// under test that dirty paths are resolved against.
//
// Usage: gate-affected-select -unresolved MODULE ROOT prints only the
// `unresolved <pkg>: <reason>` lines for ROOT, one per package, with no verdict.
// tools/gate-ledger (GL-V0-004) reads it to decide which packages `go test` may
// answer from its cache and which the ledger keys on the whole tree.
//
// Usage: gate-affected-select -bounds MODULE ROOT reads worktree paths, one per
// line, on stdin and prints the same `unresolved` lines plus one
// `bound <pkg> <rule> <path>` line per path rules (a) to (c) attribute to each
// resolved package. tools/gate-ledger (GL-V0-009) keys a resolved package on
// the content of its bound.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/gateselect"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: gate-affected-select PLAN MODULE ROOT | -unresolved MODULE ROOT | -bounds MODULE ROOT < PATHS")
		os.Exit(2)
	}
	if os.Args[1] == "-unresolved" || os.Args[1] == "-bounds" {
		lines, err := gateselect.Unresolved(os.Args[2], os.Args[3])
		if os.Args[1] == "-bounds" {
			lines, err = gateselect.Bounds(os.Args[2], os.Args[3], gateselect.ReadPaths(os.Stdin))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Print(strings.Join(lines, ""))
		return
	}
	plan, err := gateselect.ReadPlan(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(strings.Join(gateselect.Select(plan, os.Args[2], os.Args[3]), "\n"))
}
