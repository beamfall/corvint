// SPDX-License-Identifier: Apache-2.0

// Command plugin-dirs prints one language plugin's directories, one per line,
// so one language and its tooling can be tested apart from the rest of
// Corvint (LGP-V0-021):
//
//	plugin-dirs NAME
//
// The directories are langplugin.Dirs(NAME): the plugin's owned paths and every
// sub-plugin path, sorted. An unknown name, or any other argument count, is
// refused with exit 2 and the known names. `make plugin-test PLUGIN=NAME`
// runs `go test` over each printed directory through script/heavy.sh.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	var known []string
	for _, m := range langplugin.All() {
		known = append(known, m.Name)
	}
	if len(args) != 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintf(stderr, "usage: plugin-dirs NAME\nexample: go run ./tools/plugin-dirs ruby\nplugins: %s\n", strings.Join(known, " "))
		return 2
	}
	dirs, ok := langplugin.Dirs(args[0])
	if !ok {
		fmt.Fprintf(stderr, "plugin-dirs: unknown plugin %q; plugins: %s\n", args[0], strings.Join(known, " "))
		return 2
	}
	for _, dir := range dirs {
		fmt.Fprintln(stdout, dir)
	}
	return 0
}
