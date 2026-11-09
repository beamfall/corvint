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
