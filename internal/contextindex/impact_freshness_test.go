package contextindex

import (
	"context"
	"fmt"
	"strconv"
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

// DIRTY-CACHE-018: the MIXED warning names the dirty paths it is bound
// against, sorted and bounded, with an overflow count.
func TestImpactMixedWarningNamesDirtyPaths(t *testing.T) {
	root := impactRepository(t, "")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	target := "internal/token/token.go"
	index.DirtyPaths = []string{"notes.txt"}
	got, err := Impact(index, []string{target}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if warning := got["warning"].(string); !strings.HasSuffix(warning, `; dirty: "notes.txt".`) || strings.Contains(warning, "more") {
		t.Fatalf("single dirty path not named: %s", warning)
	}
	index.DirtyPaths = nil
	for i := 9; i >= 0; i-- {
		index.DirtyPaths = append(index.DirtyPaths, fmt.Sprintf("docs/n%02d.txt", i))
	}
	got, err = Impact(index, []string{target}, 10)
	if err != nil {
		t.Fatal(err)
	}
	warning := got["warning"].(string)
	want := `10 dirty paths; mutable content is excluded; dirty: "docs/n00.txt", "docs/n01.txt", "docs/n02.txt", "docs/n03.txt", "docs/n04.txt", "docs/n05.txt", "docs/n06.txt", "docs/n07.txt" (+2 more).`
	if !strings.HasSuffix(warning, want) || strings.Contains(warning, "n08") {
		t.Fatalf("warning=%s\nwant suffix %s", warning, want)
	}
}

// DIRTY-CACHE-018: under a packet budget the names spend only slack left
// after evidence selection; the count always stays.
func TestImpactMixedWarningNamesWithinBudget(t *testing.T) {
	root := impactRepository(t, "")
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	target := "internal/token/token.go"
	long := make([]string, 8)
	for i := range long {
		long[i] = fmt.Sprintf("docs/%02d-%s.txt", i, strings.Repeat("x", 200))
	}
	for _, budget := range []int{MinPacketBytes, MinPacketBytes + 600, MaxPacketBytes} {
		view := *index
		view.DirtyPaths = []string{"notes.txt"}
		short, err := EvalImpact(&view, []string{target}, 10, &budget)
		if err != nil {
			t.Fatal(err)
		}
		view.DirtyPaths = long
		got, err := EvalImpact(&view, []string{target}, 10, &budget)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := CanonicalJSON(got)
		warning := got["warning"].(string)
		if len(raw) > budget || got["coverage"].(map[string]any)["packet_bytes"] != len(raw) || !strings.Contains(warning, "8 dirty paths") {
			t.Fatalf("budget %d: %d bytes, warning %s", budget, len(raw), warning)
		}
		if len(got["results"].([]any)) != len(short["results"].([]any)) {
			t.Fatalf("budget %d: dirty-path names evicted evidence: %d vs %d results", budget, len(got["results"].([]any)), len(short["results"].([]any)))
		}
		switch budget {
		case MinPacketBytes:
			if !strings.HasSuffix(warning, "; dirty path names omitted.") {
				t.Fatalf("minimum budget kept names: %s", warning)
			}
		case MaxPacketBytes:
			if !strings.Contains(warning, strconv.Quote(long[7])) {
				t.Fatalf("roomy budget dropped names: %s", warning)
			}
		}
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

// V1-1141: a same-tree snapshot written at an earlier commit must not make the
// warning name the writer's commit; the opening observation wins.
func TestObservedCommitPrefersObservation(t *testing.T) {
	if got := observedCommit("b", "a"); got != "b" {
		t.Fatalf("got %q", got)
	}
	if got := observedCommit("", "a"); got != "a" {
		t.Fatalf("got %q", got)
	}
}
