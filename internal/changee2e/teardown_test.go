package changee2e

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/Beamfall/corvint/internal/behaviorfalsify"
	"github.com/Beamfall/corvint/internal/jstestprovider"
)

// CCE-V0-048 (V1-2410): a run-level reporter error is attributed by its PWP-V3-009 location like a
// test failure: inside the staged test tree it is the test's own teardown failure; unlocated, or
// located outside the tree or in the linked node_modules, it stays infrastructure.
func TestCCEV0048TeardownAttribution(t *testing.T) {
	tree := t.TempDir()
	roots := treeRoots(tree)
	global := func(loc *jstestprovider.Anchor) jstestprovider.Receipt {
		return jstestprovider.Receipt{Tests: []jstestprovider.TestOutcome{{State: jstestprovider.StatePassed}},
			Infrastructure: &jstestprovider.InfrastructureFailure{Reason: jstestprovider.ReasonReporterGlobalError, Detail: "Error: boom", Location: loc}}
	}
	for _, tc := range []struct {
		name string
		r    jstestprovider.Receipt
		want string
	}{
		{"scenario file", global(&jstestprovider.Anchor{File: filepath.Join(tree, "e2e", "a.spec.ts"), Line: 4}), "teardown-failed at e2e/a.spec.ts:4"},
		{"fixture file", global(&jstestprovider.Anchor{File: filepath.Join(tree, "e2e", "fixtures.ts"), Line: 9}), "teardown-failed at e2e/fixtures.ts:9"},
		{"unlocated", global(nil), "infrastructure:reporter-global-error"},
		{"outside the tree", global(&jstestprovider.Anchor{File: "/elsewhere/fixtures.ts", Line: 9}), "infrastructure:reporter-global-error"},
		{"node_modules", global(&jstestprovider.Anchor{File: filepath.Join(tree, "node_modules", "x.js"), Line: 9}), "infrastructure:reporter-global-error"},
		{"other infrastructure keeps its reason", jstestprovider.Receipt{Infrastructure: &jstestprovider.InfrastructureFailure{Reason: "server-not-ready",
			Location: &jstestprovider.Anchor{File: filepath.Join(tree, "a.spec.ts"), Line: 1}}}, "infrastructure:server-not-ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := qualify(tc.r, nil, 1, roots); got != tc.want {
				t.Fatalf("qualify reason %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCCEV0087ControlTeardownAttribution(t *testing.T) {
	root, base := fixtureRepo(t)
	head := commitBehaviourChange(t, root)
	sc := compiledScenario(t, root, base, head)
	specs := controlSpecs(sc)
	if len(specs) == 0 {
		t.Fatal("the compiled scenario has no controls")
	}
	hookAttempt := func(detail string) behaviorfalsify.AttemptResult {
		return behaviorfalsify.AttemptResult{Attempt: 1, Status: behaviorfalsify.StatusInfrastructureFailed, Reasons: []string{"playwright-infrastructure"},
			Receipt: &behaviorfalsify.HookReceipt{TestOutcome: "infrastructure_failed",
				TargetObservation: behaviorfalsify.CriterionObservation{State: "not_run"},
				Infrastructure:    &behaviorfalsify.InfrastructureFailure{Reason: "environment-unavailable", Detail: detail}}}
	}
	report := func(detail string) behaviorfalsify.Report {
		var results []behaviorfalsify.ControlResult
		for _, s := range specs {
			results = append(results, behaviorfalsify.ControlResult{ID: s.id, Kind: s.kind, Status: behaviorfalsify.StatusInfrastructureFailed,
				Reasons: []string{"one-or-more-attempts-lost-to-infrastructure"}, Attempts: []behaviorfalsify.AttemptResult{hookAttempt(detail)}})
		}
		return behaviorfalsify.Report{Results: results}
	}
	teardown := FailureTeardownFailed + " at e2e/fixtures.ts:9"
	mismatch := assertionMessage(sc, `"Pending"`)
	for _, tc := range []struct {
		name, detail, want string
	}{
		{"teardown located in a fixture", teardown, teardown + " (control " + specs[0].id + ")"},
		{"other infrastructure", "infrastructure:server-not-ready", "control-not-killed:" + specs[0].id},
	} {
		t.Run(tc.name, func(t *testing.T) {
			controls := controlResults(specs, "sha256:plan", report(tc.detail))
			if got := controls[0].Reasons; slices.Contains(got, teardown) != (tc.detail == teardown) {
				t.Fatalf("control reasons %q for detail %q", got, tc.detail)
			}
			f := &fakeRunner{sides: map[string]fakeSide{head: {states: repeat("passed")}, base: {states: repeat("failed"), message: mismatch, line: sc.AssertionLine}},
				controls: string(behaviorfalsify.StatusInfrastructureFailed), controlReasons: controls[0].Reasons}
			p := f.env(root).proveScenario(bg, base, head, sc, 3)
			if p.Verdict != VerdictInconclusive || p.Note != tc.want {
				t.Fatalf("verdict %s (%s), want %s (%s)", p.Verdict, p.Note, VerdictInconclusive, tc.want)
			}
		})
	}
	controls := controlResults(specs, "sha256:plan", report(teardown))
	controls = append([]controlProof{{ID: "control:other", Status: string(behaviorfalsify.StatusSurvived)}}, controls...)
	if v, note := verdict(sideProof{Qualified: true, Outcomes: []repeatOutcome{{State: "passed"}}}, sideProof{Qualified: true, Outcomes: []repeatOutcome{{State: "passed"}}}, controls); v != VerdictControlSurvived {
		t.Fatalf("survivor with a teardown control: %s (%s)", v, note)
	}
}
