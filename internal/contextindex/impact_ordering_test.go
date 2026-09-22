package contextindex

import (
	"context"
	"reflect"
	"testing"
)

func TestImpactOrdersMarkerEvidenceByFirstOccurrence(t *testing.T) {
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":              "module example.test/fixture\n\ngo 1.27.0\n",
		"pkg/a.go":            "package pkg\n\n// scenario:z-last\nfunc A() {}\n",
		"pkg/b.go":            "package pkg\n\n// feature:a-first\nfunc B() {}\n",
		"pkg/subject_test.go": "package pkg\n\n// feature:a-first\n// scenario:z-last\nfunc TestSubject() {}\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}

	impact := func(paths []string) map[string]any {
		t.Helper()
		receipt, err := Impact(index, paths, maxLimit)
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	forward := impact([]string{"pkg/a.go", "pkg/b.go"})
	reversed := impact([]string{"pkg/b.go", "pkg/a.go"})
	if !reflect.DeepEqual(forward, reversed) {
		t.Fatalf("impact output changed with input order:\nforward=%v\nreversed=%v", forward, reversed)
	}

	var evidence []map[string]any
	for _, result := range mapsFromAny(forward["results"]) {
		if result["kind"] == "test" && result["id"] == "pkg/subject_test.go" {
			evidence = mapsFromAny(result["evidence"])
			break
		}
	}
	if len(evidence) < 2 {
		t.Fatalf("subject_test.go evidence = %v, want two marker relations", evidence)
	}
	want := []string{
		"same-package test carries scenario:z-last",
		"same-package test carries feature:a-first",
	}
	for position, reason := range want {
		if got := stringValue(evidence[position]["reason"]); got != reason {
			t.Fatalf("evidence[%d] reason = %q, want %q; evidence=%v", position, got, reason, evidence)
		}
	}
}
