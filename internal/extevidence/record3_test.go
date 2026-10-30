package extevidence

import (
	"encoding/json"
	"strings"
	"testing"
)

// record3 is a minimal path-to-path record whose `to` endpoint carries the
// given extra members; schema selects the version.
func record3(t *testing.T, schema string, to map[string]any) []byte {
	t.Helper()
	endpoint := map[string]any{"repository": "web", "path": "src/api.ts", "blob": strings.Repeat("a", 40)}
	for key, value := range to {
		if value == nil {
			delete(endpoint, key)
			continue
		}
		endpoint[key] = value
	}
	data, err := json.Marshal(map[string]any{
		"schema":       schema,
		"provider":     map[string]any{"id": "mockdocs", "revision": "r1"},
		"repositories": []any{map[string]any{"id": "web", "revision": strings.Repeat("b", 40)}},
		"entities":     []any{},
		"relations": []any{map[string]any{
			"from":      map[string]any{"repository": "web", "path": "tests/api.spec.ts"},
			"to":        endpoint,
			"type":      "verifies",
			"evidence":  "manifest",
			"rule":      "declared",
			"reference": "docs/rule.md",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// TestEndpointSpanSymbolV3 pins EEP-V2-014 and EEP-V2-015: /3 is a separate,
// additive version; /1 and /2 decoding is unchanged and refuses span and
// symbol; a /3 narrowing is bounded and needs a pinned single-file path.
func TestEndpointSpanSymbolV3(t *testing.T) {
	span := map[string]any{"start": 3, "end": 9}
	narrowed := map[string]any{"span": span, "symbol": "checkout"}
	record, err := Decode3(record3(t, Schema3, narrowed))
	if err != nil {
		t.Fatalf("V3 narrowed endpoint: %v", err)
	}
	to := record.Relations[0].To
	if to.Span == nil || to.Span.Start != 3 || to.Span.End != 9 || to.Symbol != "checkout" {
		t.Fatalf("decoded endpoint = %+v", to)
	}
	for _, schema := range []string{Schema1, Schema2} {
		if _, err := Decode1(record3(t, schema, nil)); err != nil {
			t.Fatalf("%s without narrowing must still decode: %v", schema, err)
		}
		if _, err := Decode1(record3(t, schema, narrowed)); err == nil || !strings.Contains(err.Error(), "require schema") {
			t.Fatalf("%s must refuse span and symbol: %v", schema, err)
		}
		if _, err := Decode3(record3(t, schema, nil)); err == nil {
			t.Fatalf("Decode3 must refuse %s", schema)
		}
	}
	if _, err := Decode1(record3(t, Schema3, nil)); err == nil {
		t.Fatal("Decode1 must keep refusing /3")
	}
	if _, err := Decode3(record3(t, Schema3, nil)); err != nil {
		t.Fatalf("V3 without narrowing: %v", err)
	}
	for name, edit := range map[string]map[string]any{
		"zero start":        {"span": map[string]any{"start": 0, "end": 2}},
		"inverted":          {"span": map[string]any{"start": 5, "end": 4}},
		"over bound":        {"span": map[string]any{"start": 1, "end": MaxSpanLine + 1}},
		"unknown span key":  {"span": map[string]any{"start": 1, "end": 2, "column": 4}},
		"unpinned":          {"blob": nil, "symbol": "checkout"},
		"directory scope":   {"path": "src/", "blob": nil, "span": span},
		"expression symbol": {"symbol": "checkout()"},
		"empty segment":     {"symbol": "Cart..total"},
		"long symbol":       {"symbol": strings.Repeat("a", maxSymbol+1)},
	} {
		if _, err := Decode3(record3(t, Schema3, edit)); err == nil {
			t.Errorf("%s: V3 record must be invalid", name)
		}
	}
	entity := record3(t, Schema3, nil)
	var raw map[string]any
	if err := json.Unmarshal(entity, &raw); err != nil {
		t.Fatal(err)
	}
	raw["relations"].([]any)[0].(map[string]any)["to"] = map[string]any{"provider": "mockdocs", "entity": "cap", "symbol": "checkout"}
	entity, _ = json.Marshal(raw)
	if _, err := Decode3(entity); err == nil {
		t.Fatal("an entity endpoint must not carry a symbol")
	}
}
