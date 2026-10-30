package workflow

import (
	"reflect"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/wire"
)

// TestTCQV0066DiscriminationCandidatesAdmitRubySpecClaims checks the TCQ-V0-066
// selection: a Ruby source hunk citing a _spec.rb file is a candidate with
// only its spec files, a Go hunk keeps its _test.go files, and the not-run
// reasons name the language rule a hunk missed.
func TestTCQV0066DiscriminationCandidatesAdmitRubySpecClaims(t *testing.T) {
	t.Parallel()
	document := &wire.Map{
		Evidence: []wire.Evidence{
			{ID: "e1", Path: "spec/models/scoring_spec.rb"},
			{ID: "e2", Path: "pkg/calc/calc_test.go"},
			{ID: "e3", Path: "spec/models/other_spec.rb"},
			{ID: "e4", Path: "test/test_scoring.py"},
		},
		Hunks: []wire.Hunk{
			{Path: "app/models/scoring.rb", Basis: []wire.Basis{{EvidenceID: "e2", Relation: "test-claim"}, {EvidenceID: "e3", Relation: "test-claim"}, {EvidenceID: "e1", Relation: "test-claim"}}},
			{Path: "pkg/calc/calc.go", Basis: []wire.Basis{{EvidenceID: "e1", Relation: "test-claim"}, {EvidenceID: "e2", Relation: "test-claim"}}},
			{Path: "app/models/plain.rb", Basis: []wire.Basis{{EvidenceID: "e1", Relation: "implements"}}},
			{Path: "spec/models/scoring_spec.rb", Basis: []wire.Basis{{EvidenceID: "e3", Relation: "test-claim"}}},
			{Path: "lib/scoring.py", Basis: []wire.Basis{{EvidenceID: "e4", Relation: "test-claim"}}},
			{Path: "pkg/calc/other.go", Basis: []wire.Basis{{EvidenceID: "e1", Relation: "test-claim"}}},
		},
	}
	candidates := discriminationCandidates(document, nil)
	want := []hunkCandidate{
		{index: 0, tests: []string{"spec/models/other_spec.rb", "spec/models/scoring_spec.rb"}},
		{index: 1, tests: []string{"pkg/calc/calc_test.go"}},
	}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates = %+v, want %+v", candidates, want)
	}
	reasons := map[int]string{
		2: "no test claim cites a _spec.rb file",
		3: "not a Go or Ruby source file",
		4: "not a Go or Ruby source file",
		5: "no test claim cites a _test.go file",
	}
	for index, reason := range reasons {
		if got := unjudgedReason(index, document.Hunks[index], candidates, 8); got != reason {
			t.Errorf("hunk %d reason %q, want %q", index, got, reason)
		}
	}
}
