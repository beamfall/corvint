//go:build darwin || linux

package taskman

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// cemVerdictDocumentMembers is the closed top-level member set of a gate
// document before CEM-V1-634; uncertainty joins it only on a runner-input
// satisfaction.
var cemVerdictDocumentMembers = []string{"candidateTree", "citation", "code", "governed", "inputs", "map", "outcome", "paths", "portable", "profile", "reason", "sealedMaps", "verdict"}

var runnerInputNotObserved = cemVerdictUncertainty{RunnerSelectionCompleteness: "NOT_OBSERVED", SourceInventoryCompleteness: "NOT_OBSERVED"}

func documentMembers(t *testing.T, document []byte) []string {
	t.Helper()
	var members map[string]json.RawMessage
	if err := json.Unmarshal(document, &members); err != nil {
		t.Fatal(err)
	}
	var got []string
	for k := range members {
		got = append(got, k)
	}
	slices.Sort(got)
	return got
}

// TestCEMV1634RunnerInputGrantStatesSelectionAndInventoryNotObserved replaces
// the V1-2599 pin of the runner-input basis (V1-2658). A receipt that records
// a FAILED selected test, or a FAILED rerun the link names, still refuses
// runner-not-passed. The same change still GRANTs once the failing test is
// left out of the plan's expectedTests or the failed rerun is left out of the
// map whole, but the GRANT now states runner selection and source inventory
// completeness NOT_OBSERVED. The default fixture is both cases: its plan
// names only lib::TestHelper, and a map with a failed run left out whole is
// the same map as one that never had it.
func TestCEMV1634RunnerInputGrantStatesSelectionAndInventoryNotObserved(t *testing.T) {
	for _, c := range []struct {
		name string
		opt  runnerFixtureOptions
	}{
		{"failing selected test", runnerFixtureOptions{brokenTest: true}},
		{"failed rerun linked", runnerFixtureOptions{failedFirst: true}},
	} {
		f := newRunnerFixture(t, c.opt)
		view := requireVerdictRefusal(t, f.input(PredicateTouchedAnchored, runnerTouch...), CEMVerdictPredicateFailed, "tier-below-anchored")
		if h := governedByPath(t, view)["lib/util.txt"]; h.Satisfied || h.Reason != runnerReasonNotPassed {
			t.Fatalf("%s: lib/util.txt must refuse runner-not-passed: %+v", c.name, h)
		}
		if view.Uncertainty != nil {
			t.Fatalf("%s: no hunk is satisfied on runner-input, so no uncertainty is stated: %+v", c.name, view.Uncertainty)
		}
	}
	f := newRunnerFixture(t, runnerFixtureOptions{})
	in := f.input(PredicateTouchedAnchored, runnerTouch...)
	result, view := evaluateVerdict(t, in)
	if !result.Granted || view.Outcome != "GRANT" {
		t.Fatalf("narrowed selection still grants:\n%s", result.Document)
	}
	for _, path := range []string{"lib/util.txt", "lib/fresh.txt"} {
		if h := governedByPath(t, view)[path]; !h.Satisfied || h.Anchor != runnerAnchorInput || h.Reason != "" {
			t.Fatalf("%s must be satisfied on runner-input: %+v", path, h)
		}
	}
	if view.Uncertainty == nil || *view.Uncertainty != runnerInputNotObserved {
		t.Fatalf("a runner-input GRANT must state both completeness axes NOT_OBSERVED:\n%s", result.Document)
	}
	if !bytes.Contains(result.Document, []byte(`"uncertainty":{"runnerSelectionCompleteness":"NOT_OBSERVED","sourceInventoryCompleteness":"NOT_OBSERVED"}`)) {
		t.Fatalf("the uncertainty member is one canonical object:\n%s", result.Document)
	}
	if got, want := documentMembers(t, result.Document), append(slices.Clone(cemVerdictDocumentMembers), "uncertainty"); !slices.Equal(got, slices.Sorted(slices.Values(want))) {
		t.Fatalf("gate document members %v, want %v", got, want)
	}
	if strings.Contains(string(result.Document), "TestHelper") {
		t.Fatalf("the gate document names no selected test:\n%s", result.Document)
	}
	// The recomputed document reverifies; a document pinned before the
	// member existed is a producer mismatch (TCB-V0-013).
	again, err := ReverifyCEMVerdictGate(context.Background(), in, result.Document)
	if err != nil || !again.Granted {
		t.Fatalf("the runner-input grant must reverify: %v %+v", err, again)
	}
	var members map[string]json.RawMessage
	if err := json.Unmarshal(result.Document, &members); err != nil {
		t.Fatal(err)
	}
	delete(members, "uncertainty")
	old, err := encode(members)
	if err != nil {
		t.Fatal(err)
	}
	stale, err := ReverifyCEMVerdictGate(context.Background(), in, old)
	if err != nil || stale.Granted || stale.Code != CEMVerdictProducerMismatch || !bytes.Equal(stale.Document, result.Document) {
		t.Fatalf("a pre-CEM-V1-634 runner-input grant must refuse on reverify: %v %+v", err, stale)
	}
}

// TestCEMV1635OnlyRunnerInputSatisfactionStatesUncertainty: the member follows
// the runner-input basis, not the outcome. A refusal that still satisfies a
// hunk on runner-input states it; a GRANT satisfied only by tier or on the
// runner-artifact basis encodes without it, byte for byte as before.
func TestCEMV1635OnlyRunnerInputSatisfactionStatesUncertainty(t *testing.T) {
	f := newRunnerFixture(t, runnerFixtureOptions{})
	// all-anchored also governs the deletion, which refuses no-post-image,
	// while lib/util.txt and lib/fresh.txt stay satisfied on runner-input.
	view := requireVerdictRefusal(t, f.input(PredicateAllAnchored), CEMVerdictPredicateFailed, "tier-below-anchored")
	if h := governedByPath(t, view)["lib/util.txt"]; !h.Satisfied || h.Anchor != runnerAnchorInput {
		t.Fatalf("lib/util.txt must stay satisfied on runner-input: %+v", h)
	}
	if view.Uncertainty == nil || *view.Uncertainty != runnerInputNotObserved {
		t.Fatalf("a refusal with a runner-input satisfaction states the uncertainty too: %+v", view.Uncertainty)
	}
	result, view := evaluateVerdict(t, f.input(PredicateTouchedAnchored, "artifacts/"))
	if !result.Granted || view.Uncertainty != nil || !slices.Equal(documentMembers(t, result.Document), cemVerdictDocumentMembers) {
		t.Fatalf("a runner-artifact grant carries no completeness claim:\n%s", result.Document)
	}
	for _, h := range view.Governed {
		if h.Anchor != runnerAnchorArtifact {
			t.Fatalf("fixture expects runner-artifact hunks only: %+v", h)
		}
	}
	tier := newVerdictFixture(t, nil)
	result, _ = evaluateVerdict(t, tier.input(PredicateTouchedAnchored, "src/"))
	if !result.Granted || !slices.Equal(documentMembers(t, result.Document), cemVerdictDocumentMembers) {
		t.Fatalf("a tier-satisfied grant encodes exactly as before:\n%s", result.Document)
	}
}
