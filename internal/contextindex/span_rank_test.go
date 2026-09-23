package contextindex

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// spanFixtureFiles define RunEngine in one file and call it from another, so
// the span ranker has a core definition and a call site to emit.
var spanFixtureFiles = map[string]string{
	"go.mod": "module example.test/spans\n\ngo 1.27.0\n",
	"pkg/core/engine.go": "package core\n\n// RunEngine turns the crank once.\nfunc RunEngine(x int) int {\n\treturn crankOnce(x)\n}\n\n" +
		"func crankOnce(x int) int {\n\treturn x + 1\n}\n",
	"pkg/app/main.go":  "package app\n\nimport \"example.test/spans/pkg/core\"\n\n// Start runs the engine.\nfunc Start() int {\n\treturn core.RunEngine(1)\n}\n",
	"pkg/util/text.go": "package util\n\nfunc Trim(value string) string { return value }\n",
	"docs/engine.md":   "# engine\n\nThe engine turns a crank.\n",
}

func spanFixture(t *testing.T) (string, *Index) {
	t.Helper()
	root := impactRepositoryWithFiles(t, spanFixtureFiles)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, index
}

func spanPacket(t *testing.T, index *Index, flag, task string) map[string]any {
	t.Helper()
	t.Setenv("CORVINT_CONTEXT_SPANS", flag)
	packet, err := TaskContext(context.Background(), index, task, "", 20)
	if err != nil {
		t.Fatal(err)
	}
	return packet
}

func spanRowsOf(t *testing.T, packet map[string]any) (map[string]any, []map[string]any) {
	t.Helper()
	block, ok := packet["spans"].(map[string]any)
	if !ok {
		t.Fatalf("packet has no spans block: %#v", packet["spans"])
	}
	return block, mapsFromAny(block["rows"])
}

// TestContextSpansDefaultBytes: with the flag unset or not exactly "on" the
// packet carries no spans member and no sufficiency block, byte-identical to
// the recipe golden.
func TestContextSpansDefaultBytes(t *testing.T) {
	t.Run("TCP-V0-025", func(t *testing.T) {
		index := recipeFixtureIndex(t)
		golden, err := os.ReadFile("testdata/context-recipe-default-golden.json")
		if err != nil {
			t.Fatal(err)
		}
		for _, flag := range []string{"", "off", "ON", "unknown"} {
			t.Setenv("CORVINT_CONTEXT_SPANS", flag)
			packet := recipePacket(t, index, "", recipeFixtureTask)
			if _, present := packet["spans"]; present {
				t.Fatalf("flag %q added spans", flag)
			}
			if _, present := packet["coverage"].(map[string]any)["sufficiency"]; present {
				t.Fatalf("flag %q added sufficiency", flag)
			}
			encoded, err := json.MarshalIndent(packet, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(append(encoded, '\n'), golden) {
				t.Fatalf("flag %q changed golden packet bytes", flag)
			}
		}
	})
}

// TestContextSpansCoreAndCallSite: the definition the task names is a core
// span with its explicit line range, and the file that calls it contributes a
// call-site span whose reason names the call and the core it serves.
func TestContextSpansCoreAndCallSite(t *testing.T) {
	t.Run("TCP-V0-025", func(t *testing.T) {
		_, index := spanFixture(t)
		_, rows := spanRowsOf(t, spanPacket(t, index, "on", "Why does `RunEngine` return the wrong value?"))
		core := findSpan(rows, "core", "pkg/core/engine.go")
		if core == nil || core["start_line"] != 4 || core["end_line"] != 6 || core["symbol"] != "RunEngine" {
			t.Fatalf("core span = %#v in %#v", core, rows)
		}
		if !strings.Contains(core["reason"].(string), "defines `RunEngine`") || core["authority"] != SyntaxAuthority {
			t.Fatalf("core reason/authority = %#v", core)
		}
		if core["blob_hash"] != index.Sources["pkg/core/engine.go"].BlobHash {
			t.Fatalf("core blob_hash = %v", core["blob_hash"])
		}
	})
	t.Run("TCP-V0-026", func(t *testing.T) {
		// Here main.go is itself a result row, so its line is a core span;
		// ranking the call sites of the definition alone shows the rule.
		_, index := spanFixture(t)
		compiler := newTaskContextCompiler(index, "Why does `RunEngine` return the wrong value?", "")
		ranker := newSpanRanker(compiler, compiler.compile(20))
		core := contextSpan{role: "core", path: "pkg/core/engine.go", symbol: "RunEngine", start: 4, end: 6}
		calls := ranker.callSites([]contextSpan{core})
		var call *contextSpan
		for index := range calls {
			if calls[index].path == "pkg/app/main.go" {
				call = &calls[index]
			}
		}
		if call == nil || call.role != "call-site" || call.start > 7 || call.end < 7 || call.overlaps(core) {
			t.Fatalf("call sites = %#v", calls)
		}
		if !strings.Contains(call.reason, "names `RunEngine` at line 7") || !strings.Contains(call.reason, "pkg/core/engine.go:4-6") {
			t.Fatalf("call-site reason = %q", call.reason)
		}
	})
}

func findSpan(rows []map[string]any, role, path string) map[string]any {
	for _, row := range rows {
		if row["role"] == role && row["path"] == path {
			return row
		}
	}
	return nil
}

// TestContextSpansBudgetAndBounds: selected spans never exceed the declared
// line budget, each span stays within its cap, an overrunning span is
// counted as omitted, and the flag leaves results, the result bound and the
// repository untouched.
func TestContextSpansBudgetAndBounds(t *testing.T) {
	t.Run("TCP-V0-027", func(t *testing.T) {
		root, index := spanFixture(t)
		task := "Why does `RunEngine` call `crankOnce` in pkg/core/engine.go?"
		off := spanPacket(t, index, "", task)
		on := spanPacket(t, index, "on", task)
		block, rows := spanRowsOf(t, on)
		used := 0
		for _, row := range rows {
			lines := row["end_line"].(int) - row["start_line"].(int) + 1
			if lines < 1 || lines > contextSpanMaxLines {
				t.Fatalf("span size %d out of bounds: %#v", lines, row)
			}
			used += lines
		}
		if block["line_budget"] != contextSpanLineBudget || block["lines_used"] != used || used > contextSpanLineBudget {
			t.Fatalf("budget block = %#v, summed %d", block, used)
		}
		if sufficiency, ok := on["coverage"].(map[string]any)["sufficiency"].(map[string]any); !ok || sufficiency["verdict"] != sufficiencySatisfied {
			t.Fatalf("flag-on sufficiency = %#v", on["coverage"])
		}
		delete(on, "spans")
		delete(on["coverage"].(map[string]any), "sufficiency")
		offBytes, _ := json.Marshal(off)
		onBytes, _ := json.Marshal(on)
		if !bytes.Equal(offBytes, onBytes) {
			t.Fatalf("flag changed the packet beyond spans and sufficiency:\n%s\n%s", offBytes, onBytes)
		}
		if status := testGit(t, root, "status", "--porcelain", "--ignored"); status != "" {
			t.Fatalf("context with spans wrote to the repository: %q", status)
		}

		compiler := newTaskContextCompiler(index, task, "")
		ranker := newSpanRanker(compiler, compiler.compile(20))
		spans, omitted := ranker.rank(3)
		total := 0
		for _, span := range spans {
			total += span.lines()
		}
		if total > 3 || omitted == 0 {
			t.Fatalf("budget 3 selected %d lines, omitted %d: %#v", total, omitted, spans)
		}
	})
}
