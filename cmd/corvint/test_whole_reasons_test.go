package main

import (
	"testing"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// TestWholePackageSelectionsNameTheirReader_V13573 (V1-3573): every whole-package selection
// names the reader it rests on: a plan selection names the command it runs, the cause of its
// unbounded reads or the unit its dependency comes through. (AFP-V0-2402 removed the packages
// the AFP-V0-012 attribution added beyond the plan, so no run-set package needs an attribution
// line: TestAFPV02402RunSetIsThePlanGoSelection.)
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
}

// TestAFPV02402RunSetIsThePlanGoSelection (V1-4363): a run verdict of the AFP-V0-012 attribution
// runs exactly the plan's Go selection, so `corvint test` and `corvint affected` agree on the Go
// packages; FALLBACK and NOTHING verdicts are unchanged (negative controls), and a run verdict over
// a plan that selects no Go package is NOTHING with its reason.
func TestAFPV02402RunSetIsThePlanGoSelection(t *testing.T) {
	for _, tc := range []struct {
		verdict, want string
		planned       []string
	}{
		{"run m/app m/docs m/r", "run m/app m/lib", []string{"m/lib", "m/app", "m/app"}},
		{"FALLBACK root module definition is dirty: go.mod", "FALLBACK root module definition is dirty: go.mod", []string{"m/app"}},
		{"NOTHING no dirty or committed path; no Go package to test", "NOTHING no dirty or committed path; no Go package to test", nil},
		{"run m/docs", "NOTHING the plan selects no Go package; the AFP-V0-012 attribution names only packages the plan's rules exclude (AFP-V0-2402)", nil},
	} {
		if got := testPlanRunSet(tc.verdict, tc.planned); got != tc.want {
			t.Errorf("testPlanRunSet(%q, %v) = %q, want %q", tc.verdict, tc.planned, got, tc.want)
		}
	}
}
