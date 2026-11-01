package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// spanModeFixture holds two adjacent named functions, one that fits under
// the per-span cap but not a small budget, and one past the cap that names
// zqmarker past its first contextSpanMaxLines lines (line 140).
func spanModeFixture(t *testing.T) *Index {
	t.Helper()
	work := "package work\n\nfunc AlphaStep() int {\n\treturn 1\n}\nfunc BetaStep() int {\n\treturn 2\n}\n\n" +
		"func MediumWork() int {\n" + strings.Repeat("\t_ = 0\n", 29) + "\treturn 0\n}\n\n" +
		"func HugeWork() int {\n" + strings.Repeat("\t_ = 0\n", 96) + "\t_ = zqmarker\n" + strings.Repeat("\t_ = 0\n", 2) + "\treturn 0\n}\n"
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":           "module example.test/work\n\ngo 1.27.0\n",
		"pkg/work/work.go": work,
		"docs/notes.md":    "# notes\n",
	})
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func spanModeRow(t *testing.T, index *Index, mode, task string) (map[string]any, []map[string]any) {
	t.Helper()
	block, rows := spanRowsOf(t, spanPacket(t, index, mode, task))
	found := make([]map[string]any, 0)
	for _, row := range rows {
		if row["path"] == "pkg/work/work.go" {
			found = append(found, row)
		}
	}
	return block, found
}

// TestContextSpanHitRange: under `hits` a row names its hit range beside the
// returned range, and stripping those fields gives back the `on` packet
// (CRK-V0-009).
func TestContextSpanHitRange(t *testing.T) {
	t.Run("CRK-V0-009", func(t *testing.T) {
		_, index := spanFixture(t)
		task := "Why does `RunEngine` return the wrong value?"
		on := spanPacket(t, index, "on", task)
		hits := spanPacket(t, index, "hits", task)
		_, rows := spanRowsOf(t, hits)
		core := findSpan(rows, "core", "pkg/core/engine.go")
		if core == nil || core["start_line"] != 4 || core["end_line"] != 6 || core["hit_start_line"] != 4 || core["hit_end_line"] != 4 {
			t.Fatalf("hits core = %#v", core)
		}
		for _, row := range rows {
			delete(row, "hit_start_line")
			delete(row, "hit_end_line")
		}
		onBytes, _ := json.Marshal(on)
		hitBytes, _ := json.Marshal(hits)
		if !bytes.Equal(onBytes, hitBytes) {
			t.Fatalf("hits changed more than the hit fields:\n%s\n%s", onBytes, hitBytes)
		}
	})
}

// TestContextSpanFallbacks: under `hits` a clipped declaration says so, a hit
// past the clip returns its hit window with the reason, and a span that would
// overrun the remaining budget falls back to its hit window when that fits;
// `on` keeps the same ranges silently and omits the overrunning span
// (CRK-V0-010).
func TestContextSpanFallbacks(t *testing.T) {
	index := spanModeFixture(t)
	t.Run("CRK-V0-010", func(t *testing.T) {
		task := "Why does `HugeWork` return zero?"
		_, onRows := spanModeRow(t, index, "on", task)
		if len(onRows) != 1 || onRows[0]["start_line"] != 43 || onRows[0]["end_line"] != 43+contextSpanMaxLines-1 {
			t.Fatalf("on rows = %#v", onRows)
		}
		_, hitRows := spanModeRow(t, index, "hits", task)
		if len(hitRows) != 1 || hitRows[0]["start_line"] != 43 || hitRows[0]["end_line"] != 43+contextSpanMaxLines-1 || hitRows[0]["hit_start_line"] != 43 {
			t.Fatalf("hits rows = %#v", hitRows)
		}
		if reason := hitRows[0]["reason"].(string); !strings.HasSuffix(reason, "; `HugeWork` runs 102 lines, so the span keeps its first 80") {
			t.Fatalf("clip reason = %q", reason)
		}
		past := "Why is `zqmarker` zero?"
		_, onPast := spanModeRow(t, index, "on", past)
		_, hitPast := spanModeRow(t, index, "hits", past)
		if len(onPast) != 1 || len(hitPast) != 1 || hitPast[0]["start_line"] != 135 || hitPast[0]["end_line"] != 144 ||
			onPast[0]["start_line"] != 135 || onPast[0]["end_line"] != 144 || hitPast[0]["hit_start_line"] != 140 {
			t.Fatalf("past-clip rows on %#v hits %#v", onPast, hitPast)
		}
		if reason := hitPast[0]["reason"].(string); !strings.HasSuffix(reason, "; the hit lies past the first 80 of the 102 lines of `HugeWork`, so the hit window is returned") ||
			strings.Contains(onPast[0]["reason"].(string), "hit window") {
			t.Fatalf("past-clip reasons on %q hits %q", onPast[0]["reason"], reason)
		}
	})
	t.Run("CRK-V0-010/budget", func(t *testing.T) {
		task := "Why does `MediumWork` return zero?"
		for _, mode := range []string{"on", "hits"} {
			t.Setenv("CORVINT_CONTEXT_SPANS", mode)
			compiler := newTaskContextCompiler(index, task, "")
			ranker := newSpanRanker(compiler, compiler.compile(20))
			all, omitted := ranker.rank(15)
			spans := make([]contextSpan, 0, 1)
			for _, span := range all {
				if span.path == "pkg/work/work.go" {
					spans = append(spans, span)
				}
			}
			if mode == "on" {
				if len(spans) != 0 || omitted == 0 {
					t.Fatalf("on: spans %#v omitted %d", spans, omitted)
				}
				continue
			}
			if len(spans) != 1 || spans[0].start != 5 || spans[0].end != 15 || spanLines(all) > 15 {
				t.Fatalf("hits: spans %#v", spans)
			}
			if !strings.Contains(spans[0].note, "would overrun the remaining line budget") {
				t.Fatalf("budget fallback note = %q", spans[0].note)
			}
		}
	})
}

// TestContextSpanMerge: under `merged` adjacent spans of one file become one
// row that is its first contributor and lists the other with its hit range,
// using no more lines than `on` (CRK-V0-011).
func TestContextSpanMerge(t *testing.T) {
	index := spanModeFixture(t)
	task := "Why do `AlphaStep` and `BetaStep` disagree?"
	t.Run("CRK-V0-011", func(t *testing.T) {
		onBlock, onRows := spanModeRow(t, index, "on", task)
		if len(onRows) != 2 {
			t.Fatalf("on rows = %#v", onRows)
		}
		block, rows := spanModeRow(t, index, "merged", task)
		if len(rows) != 1 || rows[0]["start_line"] != 3 || rows[0]["end_line"] != 8 {
			t.Fatalf("merged rows = %#v", rows)
		}
		parts := mapsFromAny(rows[0]["merged_from"])
		if len(parts) != 1 || parts[0]["symbol"] != "BetaStep" || parts[0]["hit_start_line"] != 6 || parts[0]["start_line"] != 6 {
			t.Fatalf("merged_from = %#v", parts)
		}
		if rows[0]["symbol"] != "AlphaStep" || rows[0]["hit_start_line"] != 3 || rows[0]["hit_end_line"] != 3 {
			t.Fatalf("merged hit range = %#v", rows[0])
		}
		if block["lines_used"].(int) > onBlock["lines_used"].(int) {
			t.Fatalf("merged used %v lines, on %v", block["lines_used"], onBlock["lines_used"])
		}
	})
}

// TestMergeSpansPure: merging is transitive, keeps the earliest position and
// the other rows' order, leaves its input alone, and refuses a union past the
// per-span cap or a span that touches nothing (CRK-V0-011).
func TestMergeSpansPure(t *testing.T) {
	t.Run("CRK-V0-011", func(t *testing.T) {
		selected := []contextSpan{
			{role: "core", path: "f", start: 10, end: 20, hitStart: 12, hitEnd: 12},
			{role: "core", path: "g", start: 1, end: 5},
			{role: "core", path: "f", start: 30, end: 40, hitStart: 35, hitEnd: 35},
			{role: "core", path: "f", start: 1, end: 8},
		}
		before := append([]contextSpan{}, selected...)
		merged, ok := mergeSpans(selected, contextSpan{role: "call-site", path: "f", start: 21, end: 29, hitStart: 25, hitEnd: 25})
		if !ok || !reflect.DeepEqual(selected, before) {
			t.Fatalf("ok %v, input changed %v", ok, !reflect.DeepEqual(selected, before))
		}
		if len(merged) != 3 || merged[0].path != "f" || merged[0].start != 10 || merged[0].end != 40 || merged[1].path != "g" || merged[2].start != 1 {
			t.Fatalf("merged = %#v", merged)
		}
		if len(merged[0].parts) != 3 || merged[0].parts[2].role != "call-site" || merged[0].hitStart != 12 || merged[0].hitEnd != 12 {
			t.Fatalf("parts = %#v", merged[0])
		}
		again, _ := mergeSpans(selected, contextSpan{role: "call-site", path: "f", start: 21, end: 29, hitStart: 25, hitEnd: 25})
		if !reflect.DeepEqual(merged, again) {
			t.Fatal("merge is not deterministic")
		}
		bridged, ok := mergeSpans(merged, contextSpan{role: "core", path: "f", start: 9, end: 9})
		if !ok || len(bridged) != 2 || bridged[0].start != 1 || bridged[0].end != 40 || len(bridged[0].parts) != 5 {
			t.Fatalf("bridged = %#v", bridged)
		}
		if _, ok := mergeSpans(selected, contextSpan{path: "h", start: 1, end: 3}); ok {
			t.Fatal("a span of another file merged")
		}
		if _, ok := mergeSpans(selected, contextSpan{path: "f", start: 41, end: 41 + contextSpanMaxLines}); ok {
			t.Fatal("a union past the span cap merged")
		}
	})
}
