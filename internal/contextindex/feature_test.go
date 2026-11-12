package contextindex

import (
	"strings"
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

func intPointer(value int) *int { return &value }
