package main

import (
	"io"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0972EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-postmerge-workflow help --json` (CAP-V0-970 to CAP-V0-972).
func TestCAPV0972EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		code, handled := serveHelp(args, stdout, stderr)
		if !handled {
			t.Fatalf("%v: help not answered", args)
		}
		return code
	}, "../../internal/capcatalog/trees/corvint-postmerge-workflow.json")
}
