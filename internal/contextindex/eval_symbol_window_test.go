package contextindex

import (
	"context"
	"strings"
	"testing"
)

// The three fixtures below are declared line by line so a window's expected
// bounds can be read as line numbers rather than counted out of an escaped
// blob. Each is joined with "\n" and carries no trailing newline, so
// strings.Split and the oracle's str.splitlines() produce the same slice and
// an expected window can be quoted from the oracle byte for byte.

var pythonWindowFixture = []string{
	/*  1 */ `"""Roadmap helper commands."""`,
	/*  2 */ ``,
	/*  3 */ ``,
	/*  4 */ `def cmd_roadmap_ticket(args):`,
	/*  5 */ `    """Report one roadmap ticket."""`,
	/*  6 */ `    ticket = load_ticket(args.ticket)`,
	/*  7 */ `    emit({"tool": "roadmap_ticket", "ticket": ticket})`,
	/*  8 */ ``,
	/*  9 */ ``,
	/* 10 */ `def cmd_gate_summary(args):`,
	/* 11 */ `    rows = collect_gate_rows(args)`,
	/* 12 */ `    emit({"tool": "gate_summary", "rows": rows})`,
	/* 13 */ ``,
	/* 14 */ ``,
	/* 15 */ `def cmd_context_packet(args):`,
	/* 16 */ `    ticket = args.ticket`,
	/* 17 */ `    packet = build_context_packet(ticket)`,
	/* 18 */ `    emit({"tool": "context_packet", "packet": packet})`,
}

var webWindowFixture = []string{
	/*  1 */ `import { load } from "./loader";`,
	/*  2 */ ``,
	/*  3 */ `export const GATE_ROWS = 3;`,
	/*  4 */ ``,
	/*  5 */ `export function renderGatePanel(rows: GateRow[]): string {`,
	/*  6 */ "  const summary = rows.map((row) => row.label).join(\", \");",
	/*  7 */ "  return `gate: ${summary}`;",
	/*  8 */ `}`,
	/*  9 */ ``,
	/* 10 */ `export function renderContextPanel(packet: Packet): string {`,
	/* 11 */ "  return `context: ${packet.id}`;",
	/* 12 */ `}`,
}

var goWindowFixture = []string{
	/*  1 */ `package gate`,
	/*  2 */ ``,
	/*  3 */ `import "fmt"`,
	/*  4 */ ``,
	/*  5 */ `const Rows = 3`,
	/*  6 */ ``,
	/*  7 */ `func Summary(rows []Row) string {`,
	/*  8 */ "\treturn fmt.Sprintf(\"gate: %d\", len(rows))",
	/*  9 */ `}`,
	/* 10 */ ``,
	/* 11 */ `func ContextPacket(id string) string {`,
	/* 12 */ "\treturn \"context: \" + id",
	/* 13 */ `}`,
}

// symbolWindowText is the text symbolContextText yields for one symbol,
// which is the only thing the ranking reads the window for.
func symbolWindowText(t *testing.T, symbol Symbol, fixture []string) string {
	t.Helper()
	start, end := symbolContextWindow(symbol, len(fixture))
	if start >= end {
		t.Fatalf("window for %s:%d is empty: [%d,%d)", symbol.Path, symbol.Line, start, end)
	}
	return strings.Join(fixture[start:end], "\n")
}

// linesOf names an expected window by its inclusive 1-based line range, which
// is how the oracle's own slice reads once its 0-based start is translated.
func linesOf(fixture []string, first, last int) string {
	return strings.Join(fixture[first-1:last], "\n")
}

// TestPythonSymbolContextWindowMatchesTheOracle pins _python_symbols' window:
// it opens at lineno-3 (0-based), so three lines above the declaration, and
// closes at min(len(lines), node.end_lineno, lineno+20) -- the declaration's
// own last line. Before the DR-0005 repair every language shared the Go
// window, which opened one line higher and never closed early, so
// cmd_gate_summary's window absorbed the whole of cmd_context_packet below it.
// That borrowed text is what lifted its context overlap over the admission
// filter the oracle drops it with, and what re-scored cmd_context_packet from
// 108 to 120.
//
// The expected windows are the exact strings src/context_corvint_index.py
// _python_symbols yields for this fixture; EndLine comes from the grammar
// through pythonSymbols, so this covers the fact and the window together.
func TestPythonSymbolContextWindowMatchesTheOracle(t *testing.T) {
	text := strings.Join(pythonWindowFixture, "\n")
	symbols := pythonSymbols(Source{Path: "script/tools.py", BlobHash: "b1"}, text)
	if len(symbols) != 3 {
		t.Fatalf("symbols = %+v, want the fixture's three definitions", symbols)
	}
	// Oracle windows, first and last line inclusive: each stops at the
	// declaration's own end rather than 20 lines below it.
	want := []struct {
		name        string
		first, last int
	}{
		{"cmd_roadmap_ticket", 2, 7},
		{"cmd_gate_summary", 8, 12},
		{"cmd_context_packet", 13, 18},
	}
	for index, expected := range want {
		symbol := symbols[index]
		if symbol.Name != expected.name {
			t.Fatalf("symbol %d = %q, want %q", index, symbol.Name, expected.name)
		}
		got := symbolWindowText(t, symbol, pythonWindowFixture)
		if wanted := linesOf(pythonWindowFixture, expected.first, expected.last); got != wanted {
			t.Errorf("%s window =\n%q\nwant lines %d-%d:\n%q", symbol.Name, got, expected.first, expected.last, wanted)
		}
	}
}

// TestWebSymbolContextWindowOpensThreeLinesAbove pins _web_symbols' window,
// which shares the Python start offset and, unlike it, has no end clamp. Web
// symbols carried the identical start-offset defect and were never measured;
// decision 0007 D4 as amended moves them with Python. Lines are the oracle's
// own for this fixture.
func TestWebSymbolContextWindowOpensThreeLinesAbove(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		line        int
		first, last int
	}{
		{"GATE_ROWS", 3, 1, 12},
		{"renderGatePanel", 5, 3, 12},
		{"summary", 6, 4, 12},
		{"renderContextPanel", 10, 8, 12},
	} {
		symbol := Symbol{Kind: "func", Name: testCase.name, Path: "app/panel.ts", Line: testCase.line}
		got := symbolWindowText(t, symbol, webWindowFixture)
		if wanted := linesOf(webWindowFixture, testCase.first, testCase.last); got != wanted {
			t.Errorf("%s window =\n%q\nwant lines %d-%d:\n%q", testCase.name, got, testCase.first, testCase.last, wanted)
		}
	}
}

// TestGoSymbolContextWindowIsUnchanged holds the one window the candidate
// already matched. _go_symbols opens at lineno-4 and never clamps, and the
// DR-0005 repair moves only the other two, so a regression that gave every
// language the Python window would be as much a divergence as the one window
// it replaced.
func TestGoSymbolContextWindowIsUnchanged(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		line        int
		first, last int
	}{
		{"Rows", 5, 2, 13},
		{"Summary", 7, 4, 13},
		{"ContextPacket", 11, 8, 13},
	} {
		symbol := Symbol{Kind: "func", Name: testCase.name, Path: "internal/gate/gate.go", Line: testCase.line}
		got := symbolWindowText(t, symbol, goWindowFixture)
		if wanted := linesOf(goWindowFixture, testCase.first, testCase.last); got != wanted {
			t.Errorf("%s window =\n%q\nwant lines %d-%d:\n%q", testCase.name, got, testCase.first, testCase.last, wanted)
		}
	}
}

// evalSymbolWindowRepository is the DR-0005 shape as a repository: a Python
// declaration whose body ends immediately above the next declaration, which is
// the only adjacency the window difference exploits, plus a web module with
// the same adjacency. `script/tools.py` mirrors the Beamfall corpus file the
// divergence was measured on, where cmd_gate_summary's unclamped window
// absorbed cmd_context_packet below it.
func evalSymbolWindowRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testGit(t, root, "init", "-q")
	testGit(t, root, "config", "user.email", "corvint@example.test")
	testGit(t, root, "config", "user.name", "Corvint Test")
	files := map[string]string{
		".gitignore":      ".context-corvint/\n",
		"go.mod":          "module example.test/window\n\ngo 1.27.0\n",
		"script/tools.py": strings.Join(pythonWindowFixture, "\n") + "\n",
		"web/panel.ts":    strings.Join(webWindowFixture, "\n") + "\n",
	}
	for path, content := range files {
		writeTestFile(t, root, path, content)
	}
	testGit(t, root, "add", ".")
	testGit(t, root, "commit", "-qm", "roadmap ticket gate summary and context packet helpers")
	return root
}

// The exact window fixtures above and admission boundary pin the DR-0005 repair.
func TestEvalQueryAdjacentDeclarationsPreserveRecordedWindowBoundary(t *testing.T) {
	root := evalSymbolWindowRepository(t)
	index, err := Build(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	packet, err := EvalQuery(context.Background(), index, "roadmap ticket gate summary context packet emit", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, result := range mapsFromAny(packet["results"]) {
		if result["id"] == "script/tools.py:cmd_context_packet" {
			found = true
			if len(result["evidence"].([]any)) == 0 {
				t.Fatal("declaration has no original evidence")
			}
		}
	}
	if !found {
		t.Fatalf("context packet declaration missing: %#v", packet["results"])
	}
	web, err := EvalQuery(context.Background(), index, "render gate panel rows context packet", 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, result := range mapsFromAny(web["results"]) {
		if result["id"] == "web/panel.ts:renderGatePanel" {
			found = true
		}
	}
	if !found {
		t.Fatalf("web gate declaration missing: %#v", web["results"])
	}
}
