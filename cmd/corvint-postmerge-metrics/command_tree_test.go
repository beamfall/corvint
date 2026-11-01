//go:build darwin || linux

package main

import (
	"context"
	"io"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0972EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-postmerge-metrics help --json` (CAP-V0-970 to CAP-V0-972).
func TestCAPV0972EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		return run(context.Background(), args, stdout, stderr)
	}, "../../internal/capcatalog/trees/corvint-postmerge-metrics.json")
}
