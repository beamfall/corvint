package main

import (
	"io"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0726EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-work-queue help --json` (CAP-V0-726, CAP-V0-974).
func TestCAPV0726EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		code, _ := serveHelp(args, stdout, stderr)
		return code
	}, "../../internal/capcatalog/trees/corvint-work-queue.json")
}
