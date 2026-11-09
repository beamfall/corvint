package contextindex

import (
	"strings"
	"testing"
)

// The three fixtures below are declared line by line so a window's expected
// bounds can be read as line numbers rather than counted out of an escaped
// blob. Each is joined with "\n" and carries no trailing newline, so
// strings.Split and the oracle's str.splitlines() produce the same slice and
// an expected window can be quoted from the oracle byte for byte.

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
