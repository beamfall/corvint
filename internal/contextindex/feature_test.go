package contextindex

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func featureIndex() *Index {
	revision := strings.Repeat("1", 40)
	featureBlob := strings.Repeat("2", 40)
	scenarioBlob := strings.Repeat("3", 40)
	sourceBlob := strings.Repeat("4", 40)
	testBlob := strings.Repeat("5", 40)
	return &Index{
		Revision: revision, Module: "example.test/feature", ProfileID: "beamfall",
		Sources: map[string]Source{
			"go.mod":                 {Path: "go.mod", BlobHash: strings.Repeat("6", 40)},
			"pkg/sample.go":          {Path: "pkg/sample.go", BlobHash: sourceBlob},
			"pkg/sample_test.go":     {Path: "pkg/sample_test.go", BlobHash: testBlob},
			"testing/features.yaml":  {Path: "testing/features.yaml", BlobHash: featureBlob},
			"testing/scenarios.yaml": {Path: "testing/scenarios.yaml", BlobHash: scenarioBlob},
		},
		Features: map[string]Record{
			"a-first": {
				Kind: "feature", ID: "a-first", Path: "testing/features.yaml", BlobHash: featureBlob, Line: 2,
				Fields: map[string]any{"id": "a-first", "area": "auth", "summary": "Scalar fixture.", "adr": []any{pythonInteger("12")}, "applies": []any{"server"}, "status": "shipped"},
			},
		},
		Scenarios: map[string]Record{
			"z-last": {
				Kind: "scenario", ID: "z-last", Path: "testing/scenarios.yaml", BlobHash: scenarioBlob, Line: 2,
				Fields: map[string]any{"id": "z-last", "features": []any{"a-first"}},
			},
		},
		Markers: map[string][]Marker{
			"feature:a-first": {{Path: "pkg/sample_test.go", BlobHash: testBlob, Line: 3, Column: 20}},
		},
		Symbols: []Symbol{
			{Kind: "func", Name: "StableValue", Path: "pkg/sample.go", BlobHash: sourceBlob, Line: 3},
			{Kind: "func", Name: "TestStableValue", Path: "pkg/sample_test.go", BlobHash: testBlob, Line: 4},
		},
		Imports: map[string]map[string]struct{}{},
	}
}

func TestFeatureSelectsCanonicalRecordAndRankedGoImplementation(t *testing.T) {
	receipt, err := Feature(featureIndex(), "a-first", 10)
	if err != nil {
		t.Fatal(err)
	}
	if receipt["mode"] != "feature" || receipt["state"] != "READY" {
		t.Fatalf("receipt=%v", receipt)
	}
	results := mapsFromAny(receipt["results"])
	if len(results) != 2 || results[0]["kind"] != "feature" || results[0]["id"] != "a-first" || results[0]["score"] != 1000 {
		t.Fatalf("canonical results=%v", results)
	}
	if results[1]["kind"] != "symbol" || results[1]["id"] != "pkg/sample.go:StableValue" || results[1]["score"] != 680 {
		t.Fatalf("ranked implementation=%v", results[1])
	}
}

func TestFeaturePromotesImplementationsUsedByExactMarkerTest(t *testing.T) {
	index := featureIndex()
	sourceBlob := index.Sources["pkg/sample.go"].BlobHash
	testBlob := index.Sources["pkg/sample_test.go"].BlobHash
	index.Sources["pkg/sample.go"] = Source{
		Path: "pkg/sample.go", BlobHash: sourceBlob,
		Data: []byte("package pkg\n\nvar StableAttempt = 1\nvar StableWindow = 2\ntype StableRequest struct{}\nfunc ResolveStable() {}\n"),
	}
	index.Sources["pkg/sample_test.go"] = Source{
		Path: "pkg/sample_test.go", BlobHash: testBlob,
		Data: []byte("package pkg\n\nfunc TestStableValue() {\n\t// feature:a-first\n\t_ = StableRequest{}\n\tResolveStable()\n\t_ = \"StableAttempt\"\n}\n"),
	}
	index.Markers["feature:a-first"] = []Marker{{Path: "pkg/sample_test.go", BlobHash: testBlob, Line: 4, Column: 5}}
	index.Symbols = []Symbol{
		{Kind: "var", Name: "StableAttempt", Path: "pkg/sample.go", BlobHash: sourceBlob, Line: 3},
		{Kind: "var", Name: "StableWindow", Path: "pkg/sample.go", BlobHash: sourceBlob, Line: 4},
		{Kind: "type", Name: "StableRequest", Path: "pkg/sample.go", BlobHash: sourceBlob, Line: 5},
		{Kind: "func", Name: "ResolveStable", Path: "pkg/sample.go", BlobHash: sourceBlob, Line: 6},
		{Kind: "func", Name: "TestStableValue", Path: "pkg/sample_test.go", BlobHash: testBlob, Line: 3},
	}

	results := featureImplementationCandidates(index, index.Features["a-first"], 4)
	got := []string{results[0]["id"].(string), results[1]["id"].(string)}
	want := []string{"pkg/sample.go:StableRequest", "pkg/sample.go:ResolveStable"}
	if !slices.Equal(got, want) {
		t.Fatalf("promoted marker uses = %v, want %v", got, want)
	}
}

func TestFeatureMarkerUsePromotionRequiresUniqueDeclaration(t *testing.T) {
	ranked := []featureCandidate{
		{symbol: Symbol{Name: "Shared", Path: "a.go"}, markerUse: true},
		{symbol: Symbol{Name: "Shared", Path: "b.go"}, markerUse: true},
		{symbol: Symbol{Name: "Lexical", Path: "c.go"}},
		{symbol: Symbol{Name: "Direct", Path: "d.go"}, markerUse: true},
	}

	got := promoteFeatureMarkerUses(ranked)
	if got[0].symbol.Name != "Direct" || got[1].symbol.Path != "a.go" || got[2].symbol.Path != "b.go" {
		t.Fatalf("promoted marker uses = %v", got)
	}
}

func TestFeatureUnknownIsSuccessfulOutOfScopeAndBudgetable(t *testing.T) {
	for _, budget := range []*int{nil, intPointer(MinPacketBytes)} {
		receipt, err := FeatureBudget(featureIndex(), "no-such-feature", 10, budget)
		if err != nil {
			t.Fatal(err)
		}
		if receipt["state"] != "OUT_OF_SCOPE" || len(anySlice(receipt["results"])) != 0 {
			t.Fatalf("budget=%v receipt=%v", budget, receipt)
		}
		coverage := receipt["coverage"].(map[string]any)
		if coverage["requested_results"] != 0 || coverage["included_results"] != 0 {
			t.Fatalf("budget=%v coverage=%v", budget, coverage)
		}
	}
}

func TestFeatureValidatesLimitBeforeIdentifier(t *testing.T) {
	for _, test := range []struct {
		id      string
		limit   int
		message string
	}{
		{"Bad_ID", 0, "limit must be an integer from 1 to 50"},
		{"Bad_ID", 10, "feature_id must be a lowercase kebab-case identifier"},
		{"a-first", 51, "limit must be an integer from 1 to 50"},
	} {
		_, err := Feature(featureIndex(), test.id, test.limit)
		var featureError *Error
		if !errors.As(err, &featureError) || featureError.Code != "" || err.Error() != test.message {
			t.Fatalf("id=%q limit=%d err=%v", test.id, test.limit, err)
		}
	}
	for _, limit := range []int{1, 50} {
		if _, err := Feature(featureIndex(), "a-first", limit); err != nil {
			t.Fatalf("limit=%d err=%v", limit, err)
		}
	}
}

func TestFeatureBudgetPreservesCriticalSelection(t *testing.T) {
	for _, budget := range []int{MinPacketBytes, 1500, MaxPacketBytes} {
		receipt, err := FeatureBudget(featureIndex(), "a-first", 10, &budget)
		if err != nil {
			t.Fatalf("budget=%d err=%v", budget, err)
		}
		coverage := receipt["coverage"].(map[string]any)
		if coverage["budget_bytes"] != budget || coverage["within_budget"] != true || coverage["packet_bytes"].(int) > budget {
			t.Fatalf("budget=%d coverage=%v", budget, coverage)
		}
	}
}

func TestFeatureRefusesKnownNonGoCandidateRankingButNotUnknownOrLimitOne(t *testing.T) {
	index := featureIndex()
	index.Sources["pkg/candidate.py"] = Source{Path: "pkg/candidate.py", BlobHash: strings.Repeat("7", 40)}
	_, err := Feature(index, "a-first", 2)
	var featureError *Error
	if !errors.As(err, &featureError) || featureError.Code != "unsupported-feature-repository" {
		t.Fatalf("err=%v", err)
	}
	if _, err := Feature(index, "a-first", 1); err != nil {
		t.Fatalf("limit-one err=%v", err)
	}
	if _, err := Feature(index, "no-such-feature", 10); err != nil {
		t.Fatalf("unknown err=%v", err)
	}
}

func intPointer(value int) *int { return &value }
