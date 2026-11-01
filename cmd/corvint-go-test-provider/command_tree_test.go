package main

import (
	"io"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0972EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-go-test-provider help --json` (CAP-V0-970 to CAP-V0-972).
func TestCAPV0972EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		if len(args) > 0 && args[0] == "session" {
			return runSessionCommand(args[1:], strings.NewReader(""), stdout, stderr)
		}
		return runOrTestRun(args, stdout, stderr)
	}, "../../internal/capcatalog/trees/corvint-go-test-provider.json")
}
