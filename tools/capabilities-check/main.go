// SPDX-License-Identifier: AGPL-3.0-or-later

// Command capabilities-check is the CAP-V0 drift gate. Run from the repository root, it
// fails unless docs/CAPABILITIES.md has exactly one row for every corvint command in the
// root help and every verb cmd/corvint dispatches (and the two lists agree), every cmd/ binary and every corvint.* MCP tool, no row for anything that no
// longer exists, command maturity that agrees with the root help's Core list, live relative
// links, and a README "What Corvint can do" table whose rows each map to a catalog row
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
	for _, f := range findings {
		fmt.Fprintln(stderr, "capabilities-check:", f)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "capabilities-check: ok (%d commands, %d binaries, %d MCP tools)\n", len(inv.Commands), len(inv.Binaries), len(inv.Tools))
	return 0
}
