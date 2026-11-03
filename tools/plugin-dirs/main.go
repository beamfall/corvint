// SPDX-License-Identifier: Apache-2.0

// Command plugin-dirs prints one language plugin's directories, one per line,
// so one language and its tooling can be tested apart from the rest of
// Corvint (LGP-V0-021):
//
//	plugin-dirs NAME
//
// The directories are decl.Dirs(NAME), equal to langplugin.Dirs(NAME): the
// plugin's owned paths and every sub-plugin path, sorted. It reads the
// provider-free decl package, so printing them compiles no language provider. An unknown name, or any other argument count, is
// refused with exit 2 and the known names. `make plugin-test PLUGIN=NAME`
// runs `go test` over each printed directory.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/decl"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	known := decl.Names()
	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(stderr, "usage: plugin-dirs NAME\nexample: go run ./tools/plugin-dirs ruby\nplugins: %s\n", strings.Join(known, " "))
		return 2
	}
	dirs, ok := decl.Dirs(args[0])
	if !ok {
		fmt.Fprintf(stderr, "plugin-dirs: unknown plugin %q; plugins: %s\n", args[0], strings.Join(known, " "))
		return 2
	}
	for _, dir := range dirs {
		fmt.Fprintln(stdout, dir)
	}
	return 0
}
