package main

import (
	"io"
	"slices"
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

// TestCAPV0972QualifyKeepReportersFlags keeps the qualify-keep-reporters node to the e2e flags the
// command accepts: --external-server is required, and the flags it refuses are not listed.
func TestCAPV0972QualifyKeepReportersFlags(t *testing.T) {
	node, ok := companionHelp().Tree.Find([]string{"qualify-keep-reporters"})
	if !ok {
		t.Fatal("no qualify-keep-reporters node")
	}
	for _, want := range []string{"--external-server", "--config", "--test-file", "--server-arg", "--retain"} {
		if !slices.Contains(node.Flags, want) {
			t.Errorf("flags %v lack %s", node.Flags, want)
		}
	}
	for _, refused := range []string{"--keep-reporters", "--keep-reporters-qualification", "--preflight", "--watch", "--foreground"} {
		if slices.Contains(node.Flags, refused) {
			t.Errorf("flags %v list %s, which the command refuses", node.Flags, refused)
		}
	}
	if !slices.Equal(node.Required, []string{"--external-server"}) {
		t.Errorf("required = %v", node.Required)
	}
}
