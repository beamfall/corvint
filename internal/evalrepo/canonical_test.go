package evalrepo

import (
	"strings"
	"testing"
)

func TestCanonicalJSONPreservesPythonFloatSpelling(t *testing.T) {
	encoded, err := canonicalJSON(map[string]any{"one": 1.0, "zero": 0.0, "ratio": 0.9})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"one":1.0,"ratio":0.9,"zero":0.0}`; got != want {
		t.Fatalf("encoded=%s want=%s", got, want)
	}
}

func TestWeightedJSONUsesPythonDefaultSeparators(t *testing.T) {
	encoded, err := weightedJSON(map[string]any{"a": []any{1, 2}, "b": true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(encoded), `{"a": [1, 2], "b": true}`; got != want {
		t.Fatalf("encoded=%s want=%s", got, want)
	}
}

func TestScoreDeltasDistinguishesOnlyAtTwoChangedCases(t *testing.T) {
	baseline := counters{caseScores: []caseScore{{}, {}}}
	fixture := counters{caseScores: []caseScore{{criticalMisses: 1}, {}}}
	one := scoreDeltas(baseline, fixture)[0].(map[string]any)
	if one["delta"] != "not distinguished" || one["classification"] != "not distinguished" {
		t.Fatalf("one changed case=%v", one)
	}
	fixture.caseScores[1].criticalMisses = 1
	fixture.criticalTotal = 2
	two := scoreDeltas(baseline, fixture)[0].(map[string]any)
	if two["delta"] != 2 || two["classification"] != "distinguished" {
		t.Fatalf("two changed cases=%v", two)
	}
}

func TestScoredEnvelopesRequireIdenticalCaseAndReceiptInputs(t *testing.T) {
	item := goldenCase{ID: "case-a", Mode: "query"}
	receipt := func() map[string]any {
		return map[string]any{
			"request":  map[string]any{"text": "task", "limit": 5},
			"revision": "tree",
			"intent":   map[string]any{"id": "repository", "confidence": "default"},
		}
	}
	baseline, err := newScoredEnvelope(item, receipt())
	if err != nil {
		t.Fatal(err)
	}
	identical, err := newScoredEnvelope(item, receipt())
	if err != nil {
		t.Fatal(err)
	}
	if err := requireIdenticalScoredEnvelopes([]scoredEnvelope{baseline}, []scoredEnvelope{identical}); err != nil {
		t.Fatalf("identical envelopes: %v", err)
	}

	tests := map[string]func() (goldenCase, map[string]any){
		"case identity": func() (goldenCase, map[string]any) {
			changed := item
			changed.ID = "case-b"
			return changed, receipt()
		},
		"case mode": func() (goldenCase, map[string]any) {
			changed := item
			changed.Mode = "feature"
			return changed, receipt()
		},
		"request": func() (goldenCase, map[string]any) {
			changed := receipt()
			changed["request"].(map[string]any)["limit"] = 6
			return item, changed
		},
		"revision": func() (goldenCase, map[string]any) {
			changed := receipt()
			changed["revision"] = "other-tree"
			return item, changed
		},
		"intent": func() (goldenCase, map[string]any) {
			changed := receipt()
			changed["intent"].(map[string]any)["id"] = "feature"
			return item, changed
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			changedItem, changedReceipt := mutate()
			learned, envelopeErr := newScoredEnvelope(changedItem, changedReceipt)
			if envelopeErr != nil {
				t.Fatal(envelopeErr)
			}
			err := requireIdenticalScoredEnvelopes([]scoredEnvelope{baseline}, []scoredEnvelope{learned})
			if err == nil || !strings.Contains(err.Error(), "scored envelope") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
