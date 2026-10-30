package affected_test

import (
	"context"
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/liveverify/affected/golang"
	"github.com/Beamfall/corvint/internal/liveverify/affected/languages"
)

// TestCEMCodeRegistryChangeSelectsObservations_V13215: a change to the cemcode
// registry selects internal/observations, whose admitted dogfood reasons are
// read from that registry by a Go import, so a lane adding a code runs the
// observations tests locally instead of finding the drift in CI (V1-3215).
func TestCEMCodeRegistryChangeSelectsObservations_V13215(t *testing.T) {
	root := repositoryRoot(t)
	graph, err := affected.Build(context.Background(), root, languages.All()...)
	if err != nil {
		t.Fatal(err)
	}
	module, err := golang.ReadModulePath(root)
	if err != nil {
		t.Fatal(err)
	}
	plan := affected.Select(graph, []string{"internal/cem/cemcode/cemcode.go"})
	witnesses := map[string]string{}
	for _, selection := range plan.Selected {
		witnesses[selection.UnitID] = selection.Witness.Kind
	}
	if kind := witnesses["go:"+module+"/internal/cem/cemcode"]; kind != affected.WitnessDirectSource {
		t.Errorf("cemcode witness %q, want %s", kind, affected.WitnessDirectSource)
	}
	// The selection rests on the Go import, not on a test reading the
	// registry source by path, which an edit of that test could drop.
	if kind := witnesses["go:"+module+"/internal/observations"]; kind != affected.WitnessDependency {
		t.Errorf("internal/observations witness %q, want %s", kind, affected.WitnessDependency)
	}
}
