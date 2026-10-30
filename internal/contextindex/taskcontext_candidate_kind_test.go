package contextindex

import (
	"context"
	"slices"
	"testing"
)

// candidateClassOfKind is TCP-V0-163's typed candidate union: every result
// kind the packet renders belongs to exactly one candidate class.
var candidateClassOfKind = map[string]string{
	governingRelation: "governing-instruction", specMentionedRelation: "governing-instruction",
	instructionRoutedRelation: "governing-instruction",
	"mentioned":               "named-subject", "definition": "named-subject", "reverse-import": "named-subject",
	"reference": "named-subject", "cochange": "named-subject", "sibling": "named-subject",
	contextGraphRelation: "named-subject",
	"pair":               "paired-test", "test": "paired-test",
	"documentation": "documentation",
	"lexical":       "lexical",
}

func candidateKindFixture(t *testing.T) *Index {
	t.Helper()
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":              "module example.test/kinds\n\ngo 1.27.0\n",
		"AGENTS.md":           "Standing instructions.\n",
		"cache/demux.go":      "package cache\n\nfunc Split(key string) string { return key }\n",
		"cache/demux_test.go": "package cache\n\nfunc TestSplit() { _ = Split(\"k\") }\n",
		"server/server.go":    "package server\n\nimport \"example.test/kinds/cache\"\n\nfunc Run() string { return cache.Split(\"k\") }\n",
		// An authority-shaped document the task does not concern: a spec that
		// shares only the common word "keys" with the task.
		"docs/specs/billing-v0.md": "# Billing\n\n- `BIL-V0-001`: invoices carry their own keys.\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// TestTaskContextResultsNameTheirCandidateKind_TCPV0163: every rendered
// result names a kind of the typed candidate union, the union covers every
// relation the packet orders, and an authority-shaped but irrelevant spec does
// not outrank the subject's paired test or importer.
func TestTaskContextResultsNameTheirCandidateKind_TCPV0163(t *testing.T) {
	t.Parallel()
	for _, relation := range append(slices.Clone(contextRelationOrder), contextGraphRelation) {
		if _, ok := candidateClassOfKind[relation]; !ok {
			t.Fatalf("relation %q has no candidate class", relation)
		}
	}
	index := candidateKindFixture(t)
	packet, err := TaskContext(context.Background(), index, "Reviewer asks whether `Split` handles empty keys", "cache/demux.go", 20)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	position := map[string]int{}
	for rank, result := range results {
		kind, _ := result["kind"].(string)
		if _, ok := candidateClassOfKind[kind]; !ok {
			t.Fatalf("result %v names kind %q, outside the candidate union", result["id"], kind)
		}
		position[result["id"].(string)] = rank
	}
	rank, ok := position["docs/specs/billing-v0.md"]
	if !ok {
		t.Fatalf("the irrelevant spec is not a candidate, so the fixture proves nothing: %v", position)
	}
	for _, relevant := range []string{"cache/demux_test.go", "server/server.go"} {
		at, ok := position[relevant]
		if !ok {
			t.Fatalf("relevant %s missing from %v", relevant, position)
		}
		if rank < at {
			t.Fatalf("irrelevant spec at %d outranks %s at %d", rank, relevant, at)
		}
	}
}

// TestTaskContextAuthorityLabelsDoNotOrderRows_TCPV0163: the ordering stages
// after slot admission (learned slot weights, corroboration) read the row's
// kind and corroboration, never its authority label. Relabelling the
// irrelevant row with the strongest authority leaves the order unchanged.
func TestTaskContextAuthorityLabelsDoNotOrderRows_TCPV0163(t *testing.T) {
	t.Parallel()
	index := candidateKindFixture(t)
	order := func(irrelevantAuthority, relevantAuthority string) []string {
		compiler := newTaskContextCompiler(index, "Reviewer asks whether `Split` handles empty keys", "cache/demux.go")
		compiler.support["server/server.go"] = []string{"lexical"}
		rows := []contextRow{
			{kind: "documentation", path: "docs/specs/billing-v0.md", score: 100, authority: irrelevantAuthority},
			{kind: "reverse-import", path: "server/server.go", score: 700, authority: relevantAuthority},
			{kind: "lexical", path: "cache/demux_test.go", score: 120, authority: relevantAuthority},
		}
		rows = orderBySlotWeight(rows, SlotWeights{"reverse-import": 3, "lexical": 2, "documentation": 1})
		rows = compiler.corroborate(rows)
		paths := make([]string, 0, len(rows))
		for _, row := range rows {
			paths = append(paths, row.path)
		}
		return paths
	}
	want := order("vocabulary", SyntaxAuthority)
	if want[len(want)-1] != "docs/specs/billing-v0.md" {
		t.Fatalf("baseline order %v does not rank the irrelevant spec last", want)
	}
	for _, authority := range []string{"project-instructions", "repository-spec", "instruction-reference"} {
		if got := order(authority, "vocabulary"); !slices.Equal(got, want) {
			t.Fatalf("authority %q reorders rows: got %v, want %v", authority, got, want)
		}
	}
}
