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
			delete(row, "slots")
		}
		onBytes, _ := json.Marshal(on["results"])
		hitBytes, _ := json.Marshal(hits["results"])
		if !bytes.Equal(onBytes, hitBytes) {
			t.Fatalf("hits changed more than the hit fields:\n%s\n%s", onBytes, hitBytes)
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
		parts := mapsFromAny(rows[0]["contributors"])
		if len(parts) != 2 || parts[0]["hit_start_line"] != 3 || parts[1]["hit_start_line"] != 6 || !reflect.DeepEqual(parts[1]["slots"], []string{"definition", "lexical"}) {
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
