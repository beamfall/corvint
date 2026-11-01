package main

import (
	"io"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0974EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-js-test-provider help --json` (CAP-V0-972 to CAP-V0-974).
func TestCAPV0974EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		code, handled := companionHelp().Serve(args, stdout, stderr)
		if !handled {
			t.Fatalf("%v: help not answered", args)
		}
		return code
	}, "../../internal/capcatalog/trees/corvint-js-test-provider.json")
}
