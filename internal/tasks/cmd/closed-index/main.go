// SPDX-License-Identifier: Apache-2.0

// Command closed-index writes the Corvint Tasks closed-ticket index
// (docs/specs/corvint-tasks-closed-ticket-index-v0.md, TCX-V0-988) for the
// repository at DIR, whether or not its state directory exists. The CLI only
// rewrites the index after a committed mutation on a journaled store, so this
// development tool is how a benchmark indexes a journal-free copy of a store.
// It is not a shipped command and writes nothing but the index file.
//
// Usage: go run ./internal/tasks/cmd/closed-index DIR
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/Beamfall/corvint/internal/tasks/intent"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] == "" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(stderr, "usage: closed-index DIR\n\nWrites <git common dir>/taskman.closed-index for the repository at DIR,\nwith or without a journal, and prints its path.\n\nexample: go run ./internal/tasks/cmd/closed-index /tmp/store-copy")
		return 2
	}
	repo, err := intent.Resolve(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "closed-index: %v\n", err)
		return 1
	}
	if err := intent.WriteClosedIndex(repo.IntentRoot(), repo.StateDir); err != nil {
		fmt.Fprintf(stderr, "closed-index: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, intent.ClosedIndexPath(repo.StateDir))
	return 0
}
