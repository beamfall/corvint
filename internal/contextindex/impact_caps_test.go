package contextindex

import (
	"context"
	"fmt"
	"slices"
	"testing"
)

// TestMCPV0031ImpactCapsRowsWithVisibleOmissions: evidence and references per
// result are capped and the remainder is counted, never dropped silently.
func TestMCPV0031ImpactCapsRowsWithVisibleOmissions(t *testing.T) {
	t.Run("MCPV0-031", func(t *testing.T) {
		evidence := make([]any, maxImpactEvidenceRows+3)
		for position := range evidence {
			evidence[position] = map[string]any{"line": position}
		}
		over := map[string]any{"evidence": evidence, "references": []string{"a", "b", "c", "d", "e", "f"}}
		under := map[string]any{"evidence": evidence[:2], "references": []string{"a"}}
		capImpactRows([]map[string]any{over, under})
		if got := len(anySlice(over["evidence"])); got != maxImpactEvidenceRows || over["evidence_omitted"] != 3 {
			t.Fatalf("evidence=%d omitted=%v", got, over["evidence_omitted"])
		}
		if got := len(over["references"].([]string)); got != maxImpactReferenceRows || over["references_omitted"] != 6-maxImpactReferenceRows {
			t.Fatalf("references=%d omitted=%v", got, over["references_omitted"])
		}
		if _, ok := under["evidence_omitted"]; ok {
			t.Fatal("under-cap result gained evidence_omitted")
		}
		if _, ok := under["references_omitted"]; ok {
			t.Fatal("under-cap result gained references_omitted")
		}
	})
}

// TestMCPV0112ImpactCountsEvidenceCeilingSeparately covers MCPV0-112: a
// result whose builder met the maxEvidence ceiling counts the rows it dropped
// as evidence_truncated, apart from the MCPV0-031 display cut counted as
// evidence_omitted, and coverage names the ceiling's cut in its own line. A
// result under the ceiling carries no evidence_truncated member.
func TestMCPV0112ImpactCountsEvidenceCeilingSeparately(t *testing.T) {
	lines := "package p\n\nfunc B() int {\n\tx := 0\n"
	for range maxEvidence + 5 {
		lines += "\tx += Alpha()\n"
	}
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod": "module example.com/m\n\ngo 1.27\n",
		"p/a.go": "package p\n\nfunc Alpha() int { return 1 }\n",
		"p/b.go": lines + "\treturn x\n}\n",
		"p/c.go": "package p\n\nfunc C() int { return Alpha() }\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := ImpactSyntax(index, []string{"p/a.go"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var heavy, light map[string]any
	for _, result := range mapsFromAny(receipt["results"]) {
		switch result["id"] {
		case "p/b.go":
			heavy = result
		case "p/c.go":
			light = result
		}
	}
	if heavy == nil || light == nil {
		t.Fatalf("reference results missing: %v", resultKeys(receipt))
	}
	if heavy[evidenceTruncatedKey] != 5 || heavy["evidence_omitted"] != maxEvidence-maxImpactEvidenceRows ||
		len(anySlice(heavy["evidence"])) != maxImpactEvidenceRows {
		t.Fatalf("heavy reference truncated=%v omitted=%v evidence=%d", heavy[evidenceTruncatedKey], heavy["evidence_omitted"], len(anySlice(heavy["evidence"])))
	}
	if _, ok := light[evidenceTruncatedKey]; ok {
		t.Fatalf("under-ceiling result gained %s: %#v", evidenceTruncatedKey, light)
	}
	want := fmt.Sprintf("5 evidence rows in 1 results omitted by the per-result evidence ceiling of %d", maxEvidence)
	coverage, _ := receipt["coverage"].(map[string]any)
	if !slices.Contains(anySlice(coverage["uncertainty"]), any(want)) {
		t.Fatalf("coverage uncertainty lacks %q: %#v", want, coverage["uncertainty"])
	}
	// The line counts only results the packet budget keeps: a budget that
	// drops the truncated result drops its count with it (Codex review).
	for _, test := range []struct {
		budget int
		kept   bool
	}{{MaxPacketBytes, true}, {MinPacketBytes, false}} {
		budget := test.budget
		budgeted, err := EvalImpactSyntax(index, []string{"p/a.go"}, 10, &budget)
		if err != nil {
			t.Fatal(err)
		}
		carried := false
		for _, result := range mapsFromAny(budgeted["results"]) {
			if _, ok := result[evidenceTruncatedKey]; ok {
				carried = true
			}
		}
		budgetCoverage, _ := budgeted["coverage"].(map[string]any)
		if carried != test.kept || slices.Contains(anySlice(budgetCoverage["uncertainty"]), any(want)) != test.kept {
			t.Fatalf("budget %d: truncated result kept=%v, ceiling line=%v, want %v: %#v", budget, carried,
				slices.Contains(anySlice(budgetCoverage["uncertainty"]), any(want)), test.kept, budgetCoverage["uncertainty"])
		}
	}

	existing := map[string]any{"evidence": ceilingEvidenceRows("existing", maxEvidence-1)}
	candidate := map[string]any{"evidence": ceilingEvidenceRows("candidate", 3), evidenceTruncatedKey: 2}
	mergeEvidence(existing, candidate)
	if len(anySlice(existing["evidence"])) != maxEvidence || existing[evidenceTruncatedKey] != 4 {
		t.Fatalf("merge kept %d rows, truncated=%v; want %d and 4", len(anySlice(existing["evidence"])), existing[evidenceTruncatedKey], maxEvidence)
	}
}

func ceilingEvidenceRows(reason string, count int) []any {
	rows := make([]any, count)
	for position := range rows {
		rows[position] = evidence("p/x.go", position+1, "blob", reason, "medium", "syntax")
	}
	return rows
}
