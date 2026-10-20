// SPDX-License-Identifier: AGPL-3.0-or-later

// Command capabilities-check is the CAP-V0 drift gate. Run from the repository root, it
// fails unless docs/CAPABILITIES.md has exactly one row for every corvint command in the
// root help, every cmd/ binary and every corvint.* MCP tool, no row for anything that no
// longer exists, command maturity that agrees with the root help's Core list, live relative
// links, and a README "What Corvint can do" table whose rows each map to a catalog row
// (docs/specs/capability-catalog-v0.md). It reads files only; it runs nothing and writes nothing.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	os.Exit(run(os.Stdout, os.Stderr))
}

func run(stdout, stderr io.Writer) int {
	inv, err := Collect("cmd/corvint/help.go", "cmd", "internal/mcp")
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	catalog, err := os.ReadFile("docs/CAPABILITIES.md")
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	readme, err := os.ReadFile("README.md")
	if err != nil {
		fmt.Fprintln(stderr, "capabilities-check:", err)
		return 2
	}
	findings := Check(catalog, readme, inv, "docs")
	for _, f := range findings {
		fmt.Fprintln(stderr, "capabilities-check:", f)
	}
	if len(findings) > 0 {
		return 1
	}
	fmt.Fprintf(stdout, "capabilities-check: ok (%d commands, %d binaries, %d MCP tools)\n", len(inv.Commands), len(inv.Binaries), len(inv.Tools))
	return 0
}
