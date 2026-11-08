// SPDX-License-Identifier: Apache-2.0
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/runnerprotocol"
)

//go:embed command-tree.json
var commandTree string

const usage = `Usage: corvint-runner-plugin install --manifest FILE [--store DIRECTORY]
Required: --manifest, an operator-reviewed corvint-runner-plugins/1 file.
Installs the manifest's digest-pinned executable files in the per-user store.
Relative source paths resolve beside the supplied manifest. No download or PATH lookup.
The store defaults to the user config directory's corvint/runner-plugins directory.
Example: corvint-runner-plugin install --manifest ./runner-plugins.json
This explicit operator write does not edit repository pins or qualify any runner.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }
func run(args []string, out, diagnostics io.Writer) int {
	if len(args) == 2 && args[0] == "help" && args[1] == "--json" {
		fmt.Fprint(out, commandTree)
		return 0
	}
	if len(args) == 0 || len(args) == 1 && (args[0] == "--help" || args[0] == "help") {
		fmt.Fprint(out, usage)
		return 0
	}
	if args[0] != "install" {
		fmt.Fprint(diagnostics, usage)
		return 2
	}
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	flags.SetOutput(diagnostics)
	flags.Usage = func() { fmt.Fprint(out, usage) }
	manifest := flags.String("manifest", "", "operator manifest file (required)")
	store := flags.String("store", "", "per-user plugin store")
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if *manifest == "" || flags.NArg() != 0 {
		fmt.Fprint(diagnostics, usage)
		return 2
	}
	var err error
	if *store == "" {
		*store, err = runnerprotocol.DefaultStore()
		if err != nil {
			fmt.Fprintln(diagnostics, err)
			return 1
		}
	}
	config, err := runnerprotocol.ReadDiscovery(*manifest)
	if err != nil {
		fmt.Fprintln(diagnostics, err)
		return 1
	}
	if config.Schema != runnerprotocol.DiscoverySchema {
		fmt.Fprintln(diagnostics, "unsupported-schema")
		return 1
	}
	for _, pin := range config.Plugins {
		source := pin.Executable
		if !filepath.IsAbs(source) {
			source = filepath.Join(filepath.Dir(*manifest), source)
		}
		installed, e := runnerprotocol.Install(*store, pin, source)
		if e != nil {
			fmt.Fprintln(diagnostics, e)
			return 1
		}
		fmt.Fprintf(out, "%s %s\n", installed.Name, installed.SHA256)
	}
	return 0
}
