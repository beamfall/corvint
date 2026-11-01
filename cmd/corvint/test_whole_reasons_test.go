package main

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
	"github.com/Beamfall/corvint/internal/sessiontest"
)

// TestWholePackageSelectionsNameTheirReader_V13573 (V1-3573): every whole-package selection
// names the reader it rests on: a plan selection names the command it runs, the cause of its
// unbounded reads or the unit its dependency comes through, and a package the AFP-V0-012
// attribution adds beyond the plan carries its own attribution line; a plan selection keeps its
// slice and the verdict line is never a package.
func TestWholePackageSelectionsNameTheirReader_V13573(t *testing.T) {
	for want, witness := range map[string]affected.Witness{
		" (runs the built go:m/cmd/tool binary)":    {Kind: affected.WitnessBinaryExec, Via: []string{"go:m/lib", "go:m/cmd/tool", "go:m/runner"}},
		" (r/r_test.go carries the literal \"..\")": {Kind: affected.WitnessUnboundedReader, Cause: &affected.UnboundedCause{Unit: "go:m/r", Reads: "r/r_test.go carries the literal \"..\""}},
		" (imports go:m/lib)":                       {Kind: affected.WitnessDependency, Via: []string{"go:m/lib", "go:m/app"}},
		"":                                          {Kind: affected.WitnessPathLiteralReader, Via: []string{"go:m/doc"}},
	} {
		if got := wholeReader(witness); got != want {
			t.Errorf("%s: reader %q, want %q", witness.Kind, got, want)
		}
	}
	slices := map[string]sessiontest.GoSlice{"m/app": {Pattern: "^TestApp$", Reason: "selected: its own test"}}
	testAttributionReasons(slices, []string{
		"selected m/app",
		"reader m/docs <- docs/guide.md",
		"importer m/app <- lib/lib.go",
		"unresolved m/r: \"..\"",
		"run m/app m/docs m/r",
	})
	if slices["m/docs"].Reason != "whole: attributed: reader m/docs <- docs/guide.md" || slices["m/docs"].Pattern != "" {
		t.Fatalf("attributed package reason = %+v", slices["m/docs"])
	}
	if slices["m/r"].Reason != "whole: attributed: unresolved m/r: \"..\"" {
		t.Fatalf("unresolved package reason = %+v", slices["m/r"])
	}
	if slices["m/app"].Pattern != "^TestApp$" || len(slices) != 3 {
		t.Fatalf("a plan selection's slice changed, or the verdict became a package: %+v", slices)
	}
}
