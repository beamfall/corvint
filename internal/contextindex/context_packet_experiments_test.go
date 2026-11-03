// SPDX-License-Identifier: Apache-2.0
package contextindex

import (
	"encoding/json"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TCP-V0-2220: hit-only fallback fits even one remaining line, without
// crossing a file or returning a declaration unrelated to the hit.
func TestContextExactHitFallback_TCPV02220(t *testing.T) {
	index := spanModeFixture(t)
	t.Setenv("CORVINT_CONTEXT_SPANS", "hits")
	compiler := newTaskContextCompiler(index, "Why does `MediumWork` return zero?", "")
	ranker := newSpanRanker(compiler, compiler.compile(20))
	rows, omitted := ranker.rank(1)
	if len(rows) != 1 || rows[0].start != 10 || rows[0].end != 10 || !strings.Contains(rows[0].note, "remaining line budget") {
		t.Fatalf("one-line fallback: %#v omitted %d", rows, omitted)
	}
	rows, omitted = ranker.rank(0)
	if len(rows) != 0 || omitted == 0 {
		t.Fatalf("zero budget returned %#v omitted %d", rows, omitted)
	}
	path := "pkg/work/work.go"
	ranker.symbols[path] = []Symbol{
		{Name: "outer", Path: path, Line: 3, EndLine: 40},
		{Name: "inner", Path: path, Line: 6, EndLine: 8},
	}
	if span := ranker.enclosing(path, 12, 144); span.symbol != "outer" || span.start != 3 || span.end != 40 {
		t.Fatalf("nested declaration hid actual enclosure: %#v", span)
	}
	if span := ranker.enclosing(path, 1, 144); span.start != 1 || span.end != 1 || !strings.Contains(span.note, "no indexed enclosing") {
		t.Fatalf("unowned hit: %#v", span)
	}
}

// TCP-V0-2221: transitive normalization is pure even with already merged
// input, preserves rank order, and precedes line-budget selection.
func TestContextPrebudgetMerge_TCPV02221(t *testing.T) {
	a := contextSpan{path: "f", start: 1, end: 4, hitStart: 1, hitEnd: 1, slots: []string{"definition"}}
	b := contextSpan{path: "f", start: 8, end: 10, hitStart: 8, hitEnd: 8, slots: []string{"lexical"}}
	c := contextSpan{path: "f", start: 5, end: 7, hitStart: 5, hitEnd: 5, slots: []string{"reference"}}
	input := []contextSpan{a, {path: "g", start: 1, end: 2}, b, c}
	before, _ := json.Marshal(input)
	got := coalesceSpans(input)
	after, _ := json.Marshal(input)
	if string(before) != string(after) || len(got) != 2 || got[0].end != 10 || got[1].path != "g" || len(got[0].parts) != 3 {
		t.Fatalf("normalization: %#v, mutated %v", got, string(before) != string(after))
	}
	if !reflect.DeepEqual(got, coalesceSpans(input)) || !reflect.DeepEqual(got, coalesceSpans(got)) {
		t.Fatal("normalization is not deterministic and idempotent")
	}
	// Exercise a preexisting parts slice with spare capacity: appending another
	// contributor must not overwrite input backing storage.
	got[0].parts = append(make([]contextSpan, 0, 8), got[0].parts...)
	before, _ = json.Marshal(got)
	_ = coalesceSpans(append(slices.Clone(got), contextSpan{path: "f", start: 11, end: 12}))
	after, _ = json.Marshal(got)
	if string(before) != string(after) {
		t.Fatal("normalization mutated premerged input")
	}
	index := spanModeFixture(t)
	t.Setenv("CORVINT_CONTEXT_SPANS", "merged")
	compiler := newTaskContextCompiler(index, "Why do `AlphaStep` and `BetaStep` disagree?", "")
	ranker := newSpanRanker(compiler, compiler.compile(20))
	rows, omitted := ranker.rank(5)
	if len(rows) != 1 || rows[0].start != 3 || rows[0].end != 6 || len(rows[0].parts) != 2 || omitted != 0 || spanLines(rows) > 5 {
		t.Fatalf("merge did not precede fallback: %#v omitted %d", rows, omitted)
	}
}

// TCP-V0-2221/2223: retaining all slot/hit provenance should cost less than
// repeating a complete row for adjacent declarations, including full packets.
func TestContextMergeBytes_TCPV02221(t *testing.T) {
	index := spanModeFixture(t)
	task := "Why do `AlphaStep` and `BetaStep` disagree?"
	hits := spanPacket(t, index, "hits", task)
	merged := spanPacket(t, index, "merged", task)
	hitBytes, _ := json.Marshal(hits)
	mergedBytes, _ := json.Marshal(merged)
	if len(mergedBytes) > len(hitBytes) {
		t.Fatalf("merged packet grew: %d > %d", len(mergedBytes), len(hitBytes))
	}
	t.Logf("fixture packet bytes hits=%d merged=%d", len(hitBytes), len(mergedBytes))
}

// TCP-V0-2222: limit, reservations, pair and graph placement select exactly
// the same rows in both arms; only the ordinary order/reasons may change.
func TestContextRRFMembership_TCPV02222(t *testing.T) {
	index := recipeFixtureIndex(t)
	for _, graph := range []string{"", "on"} {
		t.Setenv("CORVINT_CONTEXT_GRAPH", graph)
		for limit := 1; limit <= 50; limit++ {
			t.Setenv("CORVINT_CONTEXT_RRF", "")
			off := newTaskContextCompiler(index, recipeFixtureTask, "").compile(limit)
			t.Setenv("CORVINT_CONTEXT_RRF", "on")
			on := newTaskContextCompiler(index, recipeFixtureTask, "").compile(limit)
			byPath := map[string]contextRow{}
			for _, row := range off {
				byPath[row.path] = row
			}
			if len(on) != len(off) {
				t.Fatalf("limit %d: membership size differs", limit)
			}
			for position, row := range on {
				original, exists := byPath[row.path]
				if !exists {
					t.Fatalf("limit %d: RRF admitted %s", limit, row.path)
				}
				if reservedRelation(row.kind) || row.kind == contextGraphRelation {
					if !reflect.DeepEqual(row, off[position]) {
						t.Fatalf("limit %d: reserved/graph row moved", limit)
					}
				} else {
					row.reason = strings.Split(row.reason, "; rrf ")[0]
					if !reflect.DeepEqual(row, original) {
						t.Fatalf("limit %d: RRF changed row beyond reason: %#v vs %#v", limit, row, original)
					}
				}
			}
		}
	}
}

func TestContextRRFTieBreak_TCPV02222(t *testing.T) {
	compiler := &taskContextCompiler{relationOrder: map[string]int{"definition": 0, "lexical": 1}, relationRank: map[string]map[string]int{
		"z": {"definition": 1}, "a": {"lexical": 1}, "b": {"definition": 1},
	}}
	rows := compiler.fuseRanks([]contextRow{{path: "z", kind: "definition"}, {path: "a", kind: "lexical"}, {path: "b", kind: "definition"}})
	if rows[0].path != "b" || rows[1].path != "z" || rows[2].path != "a" {
		t.Fatalf("slot then path tie-break: %#v", rows)
	}
}

// TCP-V0-2221: pre-budget coalescing never returns a row past the 80-line
// per-span cap. Two adjacent 50-line declarations fold into a 100-line union,
// which falls back to its contributors' hit range with a reason; a union whose
// hit range is itself past the cap is omitted.
func TestContextMergedRowCap_TCPV02221(t *testing.T) {
	body := func(name string) string {
		return "func " + name + "() int {\n" + strings.Repeat("\t_ = 0\n", 47) + "\treturn 0\n}\n"
	}
	root := impactRepositoryWithFiles(t, map[string]string{
		"go.mod":           "module example.test/work\n\ngo 1.27.0\n",
		"pkg/work/work.go": "package work\n\n" + body("AlphaLong") + body("BetaLong"),
	})
	index, err := Build(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CORVINT_CONTEXT_SPANS", "merged")
	compiler := newTaskContextCompiler(index, "Why do `AlphaLong` and `BetaLong` disagree?", "")
	ranker := newSpanRanker(compiler, compiler.compile(20))
	rows, _ := ranker.rank(contextSpanLineBudget)
	var work *contextSpan
	for index := range rows {
		if rows[index].lines() > contextSpanMaxLines {
			t.Fatalf("merged row past the cap: %#v", rows[index])
		}
		if rows[index].path == "pkg/work/work.go" {
			work = &rows[index]
		}
	}
	if work == nil || work.start != 3 || work.end != 53 || len(work.parts) != 2 ||
		!strings.Contains(work.note, "the merged union runs 100 lines, beyond the 80-line cap, so the hit range is returned") {
		t.Fatalf("capped merged row: %#v", work)
	}
	wide := contextSpan{path: "f", start: 1, end: 120, parts: []contextSpan{
		{path: "f", start: 1, end: 60, hitStart: 1, hitEnd: 1},
		{path: "f", start: 61, end: 120, hitStart: 100, hitEnd: 100},
	}}
	if _, ok := capMerged(wide); ok {
		t.Fatal("a union whose hit range exceeds the cap was kept")
	}
}

// TCP-V0-2220: enclosure lookup over many declarations without EndLine (Go)
// stays near-linear; the earlier per-symbol rescan visited ~n²/2 symbols.
func TestContextEnclosingManySymbols_TCPV02220(t *testing.T) {
	const count = 200000
	path := "big.go"
	symbols := make([]Symbol, count)
	lines := make([]string, count)
	for index := range symbols {
		symbols[index] = Symbol{Name: "f" + strconv.Itoa(index), Path: path, Line: index + 1}
		lines[index] = "x"
	}
	ranker := &spanRanker{hits: true, symbols: map[string][]Symbol{path: symbols}, split: map[string][]string{path: lines}}
	done := make(chan contextSpan, 1)
	go func() { done <- ranker.enclosing(path, count, count) }()
	select {
	case span := <-done:
		if span.symbol != "f"+strconv.Itoa(count-1) || span.start != count || span.end != count {
			t.Fatalf("enclosing = %#v", span)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("enclosure lookup over 200000 declarations took over 10s")
	}
}
