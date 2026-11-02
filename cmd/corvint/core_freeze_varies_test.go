package main

import (
	"reflect"
	"testing"
)

// TestCCFV1620GoldenRegenKeepsAPreviousVariesLeaf (V1-3342): two runs that happen to agree on a
// varying value (build_ms 12 in both) must not freeze it over the golden's placeholder, while a
// placeholder whose type no longer matches, and every pinned value, follow the new runs.
func TestCCFV1620GoldenRegenKeepsAPreviousVariesLeaf(t *testing.T) {
	previous := map[string]any{
		"build_ms": "<varies:number>",
		"commit":   "<varies:string>",
		"bytes":    float64(6447),
		"rows":     []any{map[string]any{"ms": "<varies:number>"}},
		"kind":     "<varies:number>",
	}
	merged := map[string]any{
		"build_ms": float64(12),
		"commit":   "<varies:string>",
		"bytes":    float64(6500),
		"rows":     []any{map[string]any{"ms": float64(3)}},
		"kind":     "text",
		"added":    float64(1),
	}
	want := map[string]any{
		"build_ms": "<varies:number>",
		"commit":   "<varies:string>",
		"bytes":    float64(6500),
		"rows":     []any{map[string]any{"ms": "<varies:number>"}},
		"kind":     "text",
		"added":    float64(1),
	}
	if got := keepCoreVaries(previous, merged); !reflect.DeepEqual(got, want) {
		t.Fatalf("keepCoreVaries = %#v, want %#v", got, want)
	}
}
