package contextindex

import (
	"runtime"
	"strings"
	"testing"
)

// V1-3016: the capped join equals truncateRunes over the full join for
// windows under, at and over the cap, with multi-byte and invalid UTF-8 on
// either side of the boundary and a newline landing on it.
func TestJoinLinesCappedMatchesTruncatedJoin(t *testing.T) {
	long := strings.Repeat("x", 1999)
	cases := [][]string{
		nil,
		{""},
		{"", ""},
		{"func F() {}", "", "\treturn"},
		{long},
		{long, ""},
		{long + "y", ""},
		{long, "z"},
		{strings.Repeat("é", 2001)},
		{strings.Repeat("a", 1998), "\xff\xfe", "b"},
		{strings.Repeat("a", 1990), "\xffinvalid tail beyond the cap"},
		{"\xff" + strings.Repeat("日本", 1500)},
		{strings.Repeat("a", 2000)},
		{strings.Repeat("a", 2001)},
	}
	for index, lines := range cases {
		want := truncateRunes(strings.Join(lines, "\n"), symbolContextRunes)
		if got := joinLinesCapped(lines, symbolContextRunes); got != want {
			t.Fatalf("case %d: capped join differs (%d vs %d bytes)", index, len(got), len(want))
		}
	}
}

// V1-3016: a window of one very long line is capped before it is joined or
// decoded, so its text costs O(cap) bytes, not O(line).
func TestSymbolContextTextAllocatesWithinTheCap(t *testing.T) {
	line := "var " + strings.Repeat("name, ", 150000) + "last int // " + strings.Repeat("comment ", 20000)
	lines := []string{"package p", "", line, ""}
	symbol := Symbol{Path: "p.go", Line: 3}
	want := truncateRunes(strings.Join(lines[:4], "\n"), symbolContextRunes)
	if got := symbolContextText(symbol, lines); got != want {
		t.Fatal("capped text differs from the truncated join")
	}
	const calls = 50
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for range calls {
		_ = symbolContextText(symbol, lines)
	}
	runtime.ReadMemStats(&after)
	perCall := (after.TotalAlloc - before.TotalAlloc) / calls
	if perCall > 4*utfMaxBytes*symbolContextRunes {
		t.Fatalf("symbolContextText allocated %d bytes per call over a %d-byte line", perCall, len(line))
	}
}

const utfMaxBytes = 4

// V1-3016: symbols that share one window (names declared on one line) post
// the same terms as a per-symbol scan, with the window computed once.
func TestBuildSymbolWindowsSharesIdenticalWindows(t *testing.T) {
	source := "package p\n\nvar alpha, beta, gamma = 1, 2, 3 // shared window text\n\nfunc delta() {}\n"
	index := &Index{
		Sources: map[string]Source{"p.go": {Path: "p.go", Data: []byte(source)}},
		Symbols: []Symbol{
			{Path: "p.go", Name: "alpha", Line: 3}, {Path: "p.go", Name: "beta", Line: 3},
			{Path: "p.go", Name: "gamma", Line: 3}, {Path: "p.go", Name: "delta", Line: 5},
		},
	}
	table := index.buildSymbolWindows()
	lines := strings.Split(source, "\n")
	for term := range terms(symbolContextText(index.Symbols[0], lines)) {
		low, high, ok := table.find(term)
		if !ok {
			t.Fatalf("term %q has no postings", term)
		}
		var positions []uint32
		table.walk(low, high, func(position uint32) { positions = append(positions, position) })
		if len(positions) < 3 || positions[0] != 0 || positions[1] != 1 || positions[2] != 2 {
			t.Fatalf("term %q postings %v, want every shared-window symbol", term, positions)
		}
	}
}
