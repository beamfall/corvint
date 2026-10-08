package contextindex

import "testing"

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
