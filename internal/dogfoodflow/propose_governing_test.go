package dogfoodflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestProposeCitationsDCWV0805CitesTheDecisionGoverningAnAmendedRequirement
// reproduces V1-3380, the 2026-10-31 bar-lowering change: a hunk amending the
// AFP-V0-029 shadow bar in affected-plan-v0.md was drafted to unrelated lines
// that only list the spec's path (documentation-ci-v0.md 9:9 and decision 0131)
// instead of the "Shadow first" paragraph of decision 0431, which the
// requirement's definition cites as its intent. The draft now cites that
// paragraph and names the requirement row as a review hint; a hunk outside any
// requirement keeps the path-anchor draft. Fails on BASE, which drafted
// documentation-ci-v0.md for both hunks.
func TestProposeCitationsDCWV0805CitesTheDecisionGoverningAnAmendedRequirement(t *testing.T) {
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	spec := "# Affected plan v0\n\n## Notes\n\nPlain prose before the requirements.\n" + numbered("note", 6) +
		"\n## Requirements\n\n" +
		"- **AFP-V0-029:** (accepted, decision 0509; intent decision 0431) Every PR run computing a trusted selection SHALL\n" +
		"  retain S with tool source, plan and selection digests.\n" +
		"  Narrowing requires, among all complete shadow rows of the current epoch, at least 20 complete rows,\n" +
		"  at least 5 of them with failing packages, and no row with a miss other than `FLAKY`.\n" +
		"  The bar is met only on a listing that reached the start of the epoch.\n" +
		"- **AFP-V0-030:** (accepted, decision 0509) The kill switch returns PR runs to FULL.\n" +
		"  It reads the latest 20 rows of the epoch.\n  It counts INCOMPLETE rows.\n  It never counts INFRA rows.\n" +
		"  It is read twice.\n  The window is bounded.\n  The verdict is trusted.\n" + numbered("tail", 6)
	writeTree(t, root, map[string]string{
		"docs/specs/affected-plan-v0.md": spec,
		"docs/specs/documentation-ci-v0.md": "# Documentation CI\n\nOwner: x\n\nAuthoritative inputs: `AGENTS.md` invariants 1, 2 and 8; `docs/specs/affected-plan-v0.md` AFP-V0-013/014/016.\n" +
			numbered("doc", 4),
		"docs/decisions/0131-gate-affected-static-read-attribution-2026-09-12.md": "# Decision 0131\n\nThe gate reads `docs/specs/affected-plan-v0.md` AFP-V0-012.\n" + numbered("d131", 4),
		"docs/decisions/0509-afp-v0-028-030-accepted-2026-10-10.md":               "# Decision 0509\n\nThe owner accepts `AFP-V0-028`, `AFP-V0-029` and `AFP-V0-030` as written.\n" + numbered("d509", 4),
		"docs/decisions/0431-merge-queue-backstops-pr-test-selection-2026-10-05.md": "# Decision 0431\n\n" +
			"6. **Forward qualification rows (`AFP-V0-029`).** Every PR run that computes a trusted selection\n" +
			"   retains it for the queue.\n" +
			"7. **Shadow first.** Items 1, 2 and 6 go live with PR runs still FULL, while the driver computes\n" +
			"   and retains S before running the full shard. In shadow, a PR run produces its own row from\n" +
			"   the same commit's complete outcomes, so failing pushes give counterfactual evidence that a\n" +
			"   wholly green queue cannot. Narrowing (item 3) is enabled when 20 complete shadow rows exist,\n" +
			"   at least 5 of them with one or more failing packages, and none of them has a miss other than\n" +
			"   `FLAKY`.\n" +
			"8. **Kill switch (`AFP-V0-030`).** PR runs return to FULL.\n",
	})
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-q", "-m", "base")
	base := testGit(t, root, "rev-parse", "HEAD")
	amended := strings.Replace(spec, "at least 20 complete rows,\n  at least 5 of them", "at least 8 complete rows,\n  at least 1 of them", 1)
	// The real case also rewrote the definition line itself, which amends
	// AFP-V0-029 rather than adding a requirement.
	amended = strings.Replace(amended, "intent decision 0431) Every", "intent decision 0431; amended 2026-10-31) Every", 1)
	amended = strings.Replace(amended, "Plain prose before the requirements.", "Plain prose, edited, before the requirements.", 1)
	// A new requirement after AFP-V0-030 adds a requirement; it amends none,
	// so it is never drafted to a decision AFP-V0-030 cites.
	amended = strings.Replace(amended, "  The verdict is trusted.\n", "  The verdict is trusted.\n- **AFP-V0-031:** (proposed) A new bar SHALL hold.\n", 1)
	writeTree(t, root, map[string]string{"docs/specs/affected-plan-v0.md": amended})
	testGit(t, root, "commit", "-q", "-am", "lower the bar")

	var stdout, stderr bytes.Buffer
	if code, err := ProposeCitations(context.Background(), ProposeOptions{Root: root, Base: base}, &stdout, &stderr); code != 0 || err != nil {
		t.Fatalf("ProposeCitations = %d, %v\n%s", code, err, stderr.String())
	}
	rows := textLines(stdout.Bytes())
	if len(rows) != 3 {
		t.Fatalf("draft has %d rows; want 3:\n%s\n%s", len(rows), stdout.String(), stderr.String())
	}
	var bar, prose, added []string
	for _, row := range rows {
		fields := strings.Split(row, "\t")
		id := fields[0]
		switch {
		case strings.Contains(stderr.String(), "hunk "+id+" edits docs/specs/affected-plan-v0.md at BASE 2,"):
			prose = fields
		case strings.Contains(stderr.String(), "hunk "+id+" edits docs/specs/affected-plan-v0.md at BASE 24,"):
			added = fields
		default:
			bar = fields
		}
	}
	if added == nil || strings.Contains(added[1], "/0509-") || strings.Contains(added[1], "/0431-") || strings.Contains(stderr.String(), "amends requirement AFP-V0-030") {
		t.Errorf("the added-requirement hunk row = %q; want no decision of the requirement before it\n%s\n%s", added, stdout.String(), stderr.String())
	}
	// The bar hunk cites the "Shadow first" paragraph (lines 5-10) on its
	// line sharing most words with the amended text, the line naming the
	// 20 complete shadow rows.
	if bar == nil || bar[1] != "docs/decisions/0431-merge-queue-backstops-pr-test-selection-2026-10-05.md" || bar[2] != "9:9" || bar[3] != "decision" {
		t.Errorf("bar hunk row = %q; want decision 0431 9:9 (the Shadow first paragraph)\n%s", bar, stdout.String())
	}
	if !strings.Contains(stderr.String(), "the hunk amends requirement AFP-V0-029, defined at BASE docs/specs/affected-plan-v0.md:") ||
		!strings.Contains(stderr.String(), "(DCW-V0-805)") {
		t.Errorf("stderr does not name the amended requirement row:\n%s", stderr.String())
	}
	// A hunk outside every requirement keeps the DCW-V0-161 path anchor.
	if prose == nil || prose[1] != "docs/specs/documentation-ci-v0.md" {
		t.Errorf("prose hunk row = %q; want the unchanged path anchor\n%s", prose, stdout.String())
	}
	if strings.Contains(bar[1], "0131") || strings.Contains(prose[1], "0131") {
		t.Errorf("draft cites the unrelated decision 0131:\n%s", stdout.String())
	}
}
