package main

import (
	"io"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0974EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-dashboard-snapshot help --json` (CAP-V0-972 to CAP-V0-974).
func TestCAPV0974EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		return run(args, stdout, stderr)
	}, "../../internal/capcatalog/trees/corvint-dashboard-snapshot.json")
}
