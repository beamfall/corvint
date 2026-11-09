package workflow

import (
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/cem/verify"
)

// tierFixture is a two-hunk stable result. selfCited makes both hunks cite
// only a span of their own file with no anchor (the 03a8ca10 shape); otherwise
// the first hunk cites a spec span that anchors it.
func tierFixture(selfCited bool) verify.StableResult {
	accept := true
	hunk := func(id, path, evidence string) map[string]any {
		return map[string]any{"id": id, "path": path, "disposition": "supported", "reason": "evidence-backed",
			"basis": []any{map[string]any{"evidenceId": evidence, "relation": "implements"}}}
	}
	result := verify.StableResult{Outcome: "ACCEPT", Accept: &accept, Stage: "complete", Spec: "cem/1.0",
		Hunks: []any{hunk("h-code", "internal/a.go", "e-a"), hunk("h-docs", "docs/b.md", "e-b")},
		Drift: []verify.StableDrift{
			{EvidenceID: "e-a", Path: "internal/a.go", Status: verify.DriftStable, TargetSpan: map[string]int64{"start": 10, "end": 40}},
			{EvidenceID: "e-b", Path: "docs/b.md", Status: verify.DriftStable, TargetSpan: map[string]int64{"start": 0, "end": 12}},
		},
		Citation: &verify.Citation{Hunks: []verify.CitationHunk{
			{HunkID: "h-code", Tier: verify.TierCited, Anchors: []string{}},
			{HunkID: "h-docs", Tier: verify.TierCited, Anchors: []string{}},
		}, HunksByClass: verify.CitationClasses{Cited: 2}, SamePathCitations: 2},
	}
	if !selfCited {
		result.Drift[0] = verify.StableDrift{EvidenceID: "e-a", Path: "docs/specs/a.md", Status: verify.DriftStable, TargetSpan: map[string]int64{"start": 5, "end": 25}}
		result.Citation.Hunks[0] = verify.CitationHunk{HunkID: "h-code", Tier: verify.TierAnchored, Anchors: []string{"id:A-001"}}
		result.Citation.HunksByClass = verify.CitationClasses{Anchored: 1, Cited: 1}
		result.Citation.SamePathCitations = 1
	}
	return result
}

func TestCEMV1368StableRowsCarryTierAnchorsAndLocation(t *testing.T) {
	render := func(t *testing.T, result verify.StableResult) (map[string]any, string) {
		t.Helper()
		projection, err := stableReviewProjection(result)
		if err != nil {
			t.Fatal(err)
		}
		return projection, renderStableReport(result, projection)
	}
	t.Run("self-cited map is distinguishable from an anchored map", func(t *testing.T) {
		selfProjection, self := render(t, tierFixture(true))
		anchoredProjection, anchored := render(t, tierFixture(false))
		for _, want := range []string{
			"- Citation tiers: held 0, mechanical-proved 0, anchored 0, cited 2, unknown 0 of 2 hunks; same-path citations 2\n",
			"- 1. `internal/a.go` `h-code`: `supported`; reason `evidence-backed`\n  - Tier `cited`; anchors none; self-only citation\n  - Evidence `e-a` at `internal/a.go` bytes 10-40; relation `implements`; drift `stable`\n",
		} {
			if !strings.Contains(self, want) {
				t.Errorf("self-cited report lacks %q:\n%s", want, self)
			}
		}
		for _, want := range []string{
			"- Citation tiers: held 0, mechanical-proved 0, anchored 1, cited 1, unknown 0 of 2 hunks; same-path citations 1\n",
			"  - Tier `anchored`; anchors `id:A-001`\n  - Evidence `e-a` at `docs/specs/a.md` bytes 5-25;",
		} {
			if !strings.Contains(anchored, want) {
				t.Errorf("anchored report lacks %q:\n%s", want, anchored)
			}
		}
		if selfProjection["recordSetSha256"] == anchoredProjection["recordSetSha256"] {
			t.Error("self-cited and anchored maps bind the same record set")
		}
	})
	t.Run("weakest support first, map ordinal kept", func(t *testing.T) {
		projection, report := render(t, tierFixture(false))
		rows := projection["hunks"].([]any)
		if first := rows[0].(map[string]any); first["id"] != "h-docs" || first["ordinal"] != 2 || first["selfOnly"] != true {
			t.Fatalf("first row %v, want the self-only h-docs at map ordinal 2", first)
		}
		if strings.Index(report, "`h-docs`") > strings.Index(report, "`h-code`") {
			t.Errorf("report does not list the self-only hunk first:\n%s", report)
		}
	})
	t.Run("refused result without citation says NOT_PRODUCED", func(t *testing.T) {
		result := tierFixture(true)
		result.Citation, result.Drift = nil, nil
		_, report := render(t, result)
		for _, want := range []string{"- Citation tiers: NOT_PRODUCED\n", "  - Tier `NOT_PRODUCED`; anchors none\n", "  - Evidence `e-a` at NOT_PRODUCED;"} {
			if !strings.Contains(report, want) {
				t.Errorf("report lacks %q:\n%s", want, report)
			}
		}
	})
}
