package main

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/clitree/clitreetest"
)

// TestCAPV0974EveryNodeAnswersHelp runs the help entry point for every command-tree node and keeps
// the committed snapshot equal to `corvint-pulse help --json` (CAP-V0-972 to CAP-V0-974).
func TestCAPV0974EveryNodeAnswersHelp(t *testing.T) {
	clitreetest.CheckCompanion(t, func(args []string, stdout, stderr io.Writer) int {
		return run(args, strings.NewReader(""), stdout, stderr)
	}, "../../internal/capcatalog/trees/corvint-pulse.json")
}

// TestCAPV0972ServeRequiresRoot keeps --root ROOT, which serve needs before its word, required.
func TestCAPV0972ServeRequiresRoot(t *testing.T) {
	node, ok := companionHelp().Tree.Find([]string{"serve"})
	if !ok || !slices.Equal(node.Required, []string{"--root ROOT", "--stdio"}) || !slices.Contains(node.Flags, "--root") {
		t.Fatalf("serve node %v: required %v, flags %v", ok, node.Required, node.Flags)
	}
}
