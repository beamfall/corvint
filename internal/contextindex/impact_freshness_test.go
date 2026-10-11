package contextindex

import (
	"context"
	"strings"
	"testing"
)

// DIRTY-CACHE-014: compare the complete set, not the displayed freshness sample.
func TestImpactMixedWarning(t *testing.T) {
	root := impactRepository(t, "")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	target := "internal/token/token.go"
	for _, tc := range []struct {
		name         string
		dirty, paths []string
		warn         bool
	}{
		{"clean", nil, []string{target}, false},
		{"disjoint", []string{"notes.txt"}, []string{target}, true},
		{"all dirty duplicate", []string{target}, []string{target, target}, false},
		{"partial overlap", []string{target}, []string{target, "internal/token/token_test.go"}, true},
		{"rename endpoints", []string{target, "internal/token/renamed.go"}, []string{target}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			view := *index
			view.DirtyPaths = tc.dirty
			for _, compile := range []func(*Index, []string, int) (map[string]any, error){Impact, ImpactSyntax} {
				got, err := compile(&view, tc.paths, 10)
				if err != nil {
					t.Fatal(err)
				}
				warning, present := got["warning"].(string)
				if present != tc.warn {
					t.Fatalf("warning=%q want present %v", warning, tc.warn)
				}
				if present && (!strings.Contains(warning, index.CommitRevision) || !strings.Contains(warning, "1 dirty paths") || strings.Contains(warning, index.Revision)) {
					t.Fatalf("wrong binding: %s", warning)
				}
				encoded, _ := CanonicalJSON(got)
				if got["coverage"].(map[string]any)["packet_bytes"] != len(encoded) {
					t.Fatalf("wrong byte accounting: %v", got["coverage"])
				}
			}
		})
	}
	index.DirtyPaths = []string{"notes.txt"}
	index.CommitRevision = ""
	got, err := Impact(index, []string{target}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got["warning"].(string), "commit UNKNOWN") {
		t.Fatal(got["warning"])
	}
	index.DirtyPaths = make([]string, maxExclusionSamples+1)
	for i := range index.DirtyPaths {
		index.DirtyPaths[i] = "notes.txt"
	}
	index.DirtyPaths[maxExclusionSamples] = target
	got, err = Impact(index, []string{target}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, present := got["warning"]; present {
		t.Fatal("used truncated dirty-path sample")
	}
}

func TestImpactMixedWarningBudget(t *testing.T) {
	root := impactRepository(t, "")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	index.DirtyPaths = []string{"notes.txt"}
	budget := MinPacketBytes
	got, err := EvalImpact(index, []string{"internal/token/token.go"}, 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := CanonicalJSON(got)
	if len(raw) > budget || !strings.Contains(got["warning"].(string), index.CommitRevision) {
		t.Fatalf("budget/warning: %s", raw)
	}
}

// DIRTY-CACHE-015: strict abstention remains explicit even at the minimum budget.
func TestImpactRequireCleanBudget(t *testing.T) {
	index := &Index{CommitRevision: strings.Repeat("a", 64), Revision: strings.Repeat("b", 64), DirtyPaths: []string{"pkg/a.go"}}
	budget := MinPacketBytes
	got, err := ImpactCleanAbstention(index, []string{"pkg/a.go"}, 10, &budget)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := CanonicalJSON(got)
	if len(raw) > budget || got["state"] != "NEEDS_WIDENING" || len(got["results"].([]any)) != 0 || got["abstention"].(map[string]any)["active"] != true {
		t.Fatalf("abstention: %s", raw)
	}
}

func TestImpactRequireCleanValidatesRequest(t *testing.T) {
	index := &Index{DirtyPaths: []string{"pkg/a.go"}}
	for _, limit := range []int{-1, 0, maxLimit + 1} {
		if _, err := ImpactCleanAbstention(index, []string{"pkg/a.go"}, limit, nil); err == nil {
			t.Fatalf("accepted limit %d", limit)
		}
	}
	for _, paths := range [][]string{nil, {"../a.go"}, {"./pkg/a.go"}, make([]string, maxImpactPaths+1)} {
		if _, err := ImpactCleanAbstention(index, paths, 10, nil); err == nil {
			t.Fatalf("accepted paths %v", paths)
		}
	}
}
