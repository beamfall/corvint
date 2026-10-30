package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/taskman"
)

// taskmanPreview is a variable only so a test can reach the stdout write path
// without a live trusted-local executor.
var taskmanPreview = taskman.Preview

// taskmanFixtureHelp states the opt-in preview's usage and its ticket bound
// (NTP-V0-246); it answers without I/O.
const taskmanFixtureHelp = `Preview a read-only native priority-first plan from a trusted-local executor.

Usage:
  corvint [--root PATH] work plan-fixture --executor ABSOLUTE_FILE --observations FILE

It admits at most 16384 tickets and decodes at most 16384 plan entries and 16384
blockers per entry; one more refuses and nothing is truncated. Each executor
response stays within 16 MiB and all ticket exports within 32 MiB.
`

func runTaskmanFixture(ctx context.Context, root string, args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		_, err := io.WriteString(stdout, taskmanFixtureHelp)
		if err != nil {
			return 2
		}
		return 0
	}
	flags := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		if i+1 >= len(args) || (args[i] != "--executor" && args[i] != "--observations") || args[i+1] == "" {
			fmt.Fprintln(stderr, "taskman fixture: expected --executor ABSOLUTE_FILE --observations FILE")
			return 2
		}
		if _, exists := flags[args[i]]; exists {
			fmt.Fprintln(stderr, "taskman fixture: duplicate argument")
			return 2
		}
		flags[args[i]] = args[i+1]
	}
	if len(flags) != 2 {
		fmt.Fprintln(stderr, "taskman fixture: executor and observations are required")
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	raw, e := taskmanPreview(ctx, root, flags["--executor"], flags["--observations"])
	if e != nil {
		fmt.Fprintln(stderr, "taskman fixture:", e)
		return 2
	}
	_, e = stdout.Write(raw)
	if e != nil {
		emitError(stderr, &gokernel.Error{Code: "output-failed", Message: "cannot write fixture output"})
		return 2
	}
	return 0
}
