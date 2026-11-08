// SPDX-License-Identifier: Apache-2.0
// The reference binary is built explicitly; it is not in the release manifest.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/Beamfall/corvint/internal/runnerprotocol"
	"github.com/Beamfall/corvint/internal/testrunner/registry"
	"os"
)

//go:embed command-tree.json
var commandTree string

func main() {
	if len(os.Args) == 3 && os.Args[1] == "help" && os.Args[2] == "--json" {
		fmt.Print(commandTree)
		return
	}
	if len(os.Args) == 2 && os.Args[1] == "help" {
		os.Args[1] = "--help"
	}
	if len(os.Args) == 2 && os.Args[1] == "--help" {
		fmt.Println("Usage: corvint-runner-reference < requests.jsonl\nRequired: corvint-runner/1 requests on stdin.\nExample: corvint-runner-reference < requests.jsonl\nUnqualified Go reference twin; describes commands and parses core-captured artifacts. No runner is executed.")
		return
	}
	if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "unexpected arguments; use --help")
		os.Exit(2)
	}
	if err := runnerprotocol.Serve(context.Background(), os.Stdin, os.Stdout, "reference", "1", map[string]registry.RunnerPlugin{runnerprotocol.ReferenceRunner: runnerprotocol.Reference{}}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
