package contextindex

import (
	"context"
	"slices"
	"testing"
)

// namedPathFixture mirrors the V1-0148 orientation miss (V1-0179): a task
// names two documents, a small gate spec cites both, a long spec repeats the
// task's words but cites one, and a decision cites only the changed document.
func namedPathFixture(t *testing.T) *Index {
	t.Helper()
	index, err := Build(context.Background(), impactRepositoryWithFiles(t, map[string]string{
		"docs/GUIDE.md":   "# Guide\n\nThe runbook steps are at `docs/RUNBOOK.md` lines 3-5.\n",
		"docs/RUNBOOK.md": "# Runbook\n\nStep one.\n",
		"docs/specs/citation-gate-v0.md": "# Citation gate\n\nStatus: accepted\n\n" +
			"- `CG-V0-001`: the gate scans `docs/GUIDE.md` and `docs/RUNBOOK.md`.\n",
		"docs/specs/kernel-v0.md": "# Kernel docs spec\n\nStatus: accepted\n\n" +
			"A stale line range in docs cites the runbook; see `docs/RUNBOOK.md`.\n\n## Docs\n\n## Line range\n\n## Stale citations\n",
		"docs/decisions/0001-guide-2026-09-01.md": "# Guide layout\n\nStatus: accepted\n\nThe guide lives at `docs/GUIDE.md`.\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	return index
}

// TestEvalQueryRanksTheSpecCitingEveryNamedPathFirst_GPKV0975: the document
// whose references cite every path the task names outranks a longer one that
// shares more of the task's words but cites only one of them.
func TestEvalQueryRanksTheSpecCitingEveryNamedPathFirst_GPKV0975(t *testing.T) {
	t.Parallel()
	index := namedPathFixture(t)
	packet, err := EvalQuery(context.Background(), index, "docs: GUIDE.md:3 cites a stale RUNBOOK.md line range", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	results := mapsFromAny(packet["results"])
	ids := make([]string, 0, len(results))
	for _, result := range results {
		ids = append(ids, stringValue(result["id"]))
	}
	if len(ids) == 0 || ids[0] != "docs/specs/citation-gate-v0.md" {
		t.Fatalf("results = %v, want the citation gate spec first", ids)
	}
	if named := evalNamedPaths(index, "docs: guide.md:3 cites a stale runbook.md line range"); !slices.Equal(named, []string{"docs/GUIDE.md", "docs/RUNBOOK.md"}) {
		t.Fatalf("named paths = %v", named)
	}
	// A bare word is never a path, and a document never cites itself.
	if named := evalNamedPaths(index, "guide runbook"); len(named) != 0 {
		t.Fatalf("bare words named %v", named)
	}
	for _, record := range index.Documents {
		if record.Path == "docs/GUIDE.md" && evalCitedNamedPaths(record, []string{"docs/GUIDE.md"}) != 0 {
			t.Fatal("the named document cites itself")
		}
	}
}

// TestImpactOrdersAReferencingSpecBeforeAReferencingDecision_GPKV0976: at the
// shared 825 score the spec that cites the changed document comes before the
// decision that cites it, so a packet budget that keeps one keeps the spec.
func TestImpactOrdersAReferencingSpecBeforeAReferencingDecision_GPKV0976(t *testing.T) {
	t.Parallel()
	packet, err := Impact(namedPathFixture(t), []string{"docs/GUIDE.md"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0)
	for _, result := range mapsFromAny(packet["results"]) {
		ids = append(ids, stringValue(result["id"]))
	}
	spec, decision := slices.Index(ids, "docs/specs/citation-gate-v0.md"), slices.Index(ids, "docs/decisions/0001-guide-2026-09-01.md")
	if spec < 0 || decision < 0 || spec > decision {
		t.Fatalf("results = %v, want the referencing spec before the referencing decision", ids)
	}
}

// TestEvalNamedPathsKeepsEveryCaseFoldedMatch_GPKV0975: two tracked paths that
// differ only in case both answer a lowercased full-path mention, in sorted
// order, on every ranking, and a basename they share names neither.
func TestEvalNamedPathsKeepsEveryCaseFoldedMatch_GPKV0975(t *testing.T) {
	t.Parallel()
	root, _ := caseFoldRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"internal/token/Token.go", "internal/token/token.go"}
	const task = "Why does internal/token/Token.go mint the wrong token?"
	first, err := EvalQuery(context.Background(), index, task, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	firstBytes, err := CanonicalJSON(first)
	if err != nil {
		t.Fatal(err)
	}
	for range 32 {
		if named := evalNamedPaths(index, pythonLower(task)); !slices.Equal(named, want) {
			t.Fatalf("named paths = %v, want %v", named, want)
		}
		packet, err := EvalQuery(context.Background(), index, task, 10, nil)
		if err != nil {
			t.Fatal(err)
		}
		if data, err := CanonicalJSON(packet); err != nil || string(data) != string(firstBytes) {
			t.Fatalf("repeated ranking differs (err %v)", err)
		}
	}
	if named := evalNamedPaths(index, "see token.go"); len(named) != 0 {
		t.Fatalf("an ambiguous basename named %v", named)
	}
}
