// SPDX-License-Identifier: Apache-2.0

// Command capabilities-check is the CAP-V0 drift gate. Run from the repository root, it
// fails unless docs/CAPABILITIES.md has exactly one row for every corvint command in the
// root help and every verb cmd/corvint dispatches (and the two lists agree), every cmd/ binary and every corvint.* MCP tool, no row for anything that no
// longer exists, command maturity that agrees with the root help's Core and Experimental groups,
// every code-declared subcommand named in its verb's row, every listed refusal code still present
// in the source, live relative links, a README "What Corvint can do" table whose rows each map to a catalog row,
// and, for each command-tree snapshot under internal/capcatalog/trees, every dispatched subcommand
// and option named by the tree, with every other binary on a ratchet that may only shrink
// (docs/specs/capability-catalog-v0.md). It reads files only; it runs nothing and writes nothing.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func main() {
	os.Exit(run(".", os.Stdout, os.Stderr))
}

// run checks the repository at root; main passes the working directory.
func run(root string, stdout, stderr io.Writer) int {
	inv, err := Collect(filepath.Join(root, "cmd", "corvint", "help.go"), filepath.Join(root, "cmd", "corvint", "main.go"), filepath.Join(root, "cmd"), filepath.Join(root, "internal", "mcp"))
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	inv.Words, err = ScanWords([]string{filepath.Join(root, "cmd"), filepath.Join(root, "internal")})
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	catalog, err := os.ReadFile(filepath.Join(root, "docs", "CAPABILITIES.md"))
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	findings := Check(catalog, readme, inv, filepath.Join(root, "docs"))
	findings = append(findings, CheckTrees(root, inv.Binaries)...)
	findings = append(findings, CheckEntryPoint(root)...)
	for _, f := range findings {
		fmt.Fprintln(stderr, "capabilities-check:", f)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "capabilities-check: ok (%d commands, %d binaries, %d MCP tools)\n", len(inv.Commands), len(inv.Binaries), len(inv.Tools))
	return 0
}
