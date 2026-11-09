package bridge

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

// TestMCPV0033TextSummaryProjectsRowsAndKeepsEnvelopeFields: the MCP text
// block carries the full result object except that each receipt row is
// projected to kind, id and score, and the summary names its profile.
func TestMCPV0033TextSummaryProjectsRowsAndKeepsEnvelopeFields(t *testing.T) {
	t.Run("MCPV0-033", func(t *testing.T) {
		registry, err := New(makeRepository(t))
		if err != nil {
			t.Fatal(err)
		}
		result, callErr := registry.Call(context.Background(), ToolImpact, []byte(`{"paths":["internal/widget/widget.go"]}`))
		if callErr != nil {
			t.Fatal(callErr)
		}
		full, fullErr := result.CanonicalJSON()
		if fullErr != nil {
			t.Fatal(fullErr)
		}
		text, textErr := result.TextJSON()
		if textErr != nil {
			t.Fatal(textErr)
		}
		var fullObject, summary map[string]any
		if json.Unmarshal(full, &fullObject) != nil || json.Unmarshal(text, &summary) != nil {
			t.Fatal("result does not decode")
		}
		if summary["textProfile"] != TextSummaryProfile || len(text) >= len(full) {
			t.Fatalf("summary=%s", text)
		}
		rows := fullObject["receipt"].(map[string]any)["results"].([]any)
		projected := summary["receipt"].(map[string]any)["results"].([]any)
		if len(rows) == 0 || len(rows) != len(projected) {
			t.Fatalf("rows=%d projected=%d", len(rows), len(projected))
		}
		for position, raw := range rows {
			row, want := raw.(map[string]any), map[string]any{}
			for _, key := range []string{"kind", "id", "score"} {
				if value, ok := row[key]; ok {
					want[key] = value
				}
			}
			if !reflect.DeepEqual(projected[position], want) {
				t.Fatalf("row %d = %#v, want %#v", position, projected[position], want)
			}
		}
		delete(summary, "textProfile")
		delete(summary["receipt"].(map[string]any), "results")
		delete(fullObject["receipt"].(map[string]any), "results")
		if !reflect.DeepEqual(summary, fullObject) {
			t.Fatalf("summary changed fields outside receipt.results")
		}
	})
}
