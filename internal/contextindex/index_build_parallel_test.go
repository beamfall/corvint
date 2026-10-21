package contextindex

import (
	"reflect"
	"testing"
)

// TestGPKV0088SymbolWindowsMatchEachSymbolsTerms pins GPK-V0-088 for the
// symbol windows: each worker reuses one cleared set across its symbols, so
// every symbol's posting set must still be exactly terms(symbolContextText)
// for that symbol alone. A set carried over between symbols adds the
// previous window's terms and fails here.
func TestGPKV0088SymbolWindowsMatchEachSymbolsTerms(t *testing.T) {
	index := evalIndexWithSymbolWindows(t, evalQueryRepository(t))
	table := &index.Vocabulary.SymbolWindows
	got := make([]map[string]struct{}, len(index.Symbols))
	for key := 0; key < table.keyCount(); key++ {
		term := table.key(key)
		if !table.walk(int(table.Offsets[key]), int(table.Offsets[key+1]), func(position uint32) {
			if got[position] == nil {
				got[position] = map[string]struct{}{}
			}
			got[position][term] = struct{}{}
		}) {
			t.Fatalf("posting list for %q does not decode", term)
		}
	}
	sources := evalSourceLines{index: index}
	compared := 0
	for position, symbol := range index.Symbols {
		want := map[string]struct{}{}
		if !isTestPath(symbol.Path) {
			want = terms(symbolContextText(symbol, sources.of(symbol.Path).lines))
		}
		actual := got[position]
		if actual == nil {
			actual = map[string]struct{}{}
		}
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("symbol %d %s %s: window terms %v, want %v", position, symbol.Path, symbol.Name, sortedStringKeys(actual), sortedStringKeys(want))
		}
		if len(want) > 0 {
			compared++
		}
	}
	if compared < 2 {
		t.Fatalf("only %d symbols carried window terms; the fixture cannot show a set leaking between symbols", compared)
	}
}

// TestGPKV0088ConcurrentCompileMatchesSequentialPhases pins GPK-V0-088 for
// compile: the vocabulary built beside the per-source tables, and the
// postings inverted in parallel, must equal the same phases run one after
// another.
func TestGPKV0088ConcurrentCompileMatchesSequentialPhases(t *testing.T) {
	index := evalIndexWithSymbolWindows(t, evalQueryRepository(t))
	index.compile()
	sequential := index.buildVocabulary()
	sequential.SymbolWindows = index.buildSymbolWindows()
	sequential.IdentGraph = index.buildIdentGraph(sequential)
	if !reflect.DeepEqual(index.Vocabulary, sequential) {
		t.Fatal("the concurrently compiled vocabulary differs from the sequential phases")
	}

	paths := sortedStringKeys(index.Sources)
	perSource := make([]sourceTerms, len(paths))
	tokeniser := newTokeniser()
	for position, path := range paths {
		perSource[position] = tokeniseSource(path, index.Sources[path], tokeniser)
	}
	counted, words, pathTerms := invertSourceTerms(perSource)
	if !reflect.DeepEqual(counted, invertCounts(perSource)) {
		t.Fatal("parallel term inversion differs from invertCounts")
	}
	if !reflect.DeepEqual(words, invertPresence(perSource, func(entry sourceTerms) map[string]struct{} { return entry.words })) {
		t.Fatal("parallel word inversion differs from invertPresence")
	}
	if !reflect.DeepEqual(pathTerms, invertPresence(perSource, func(entry sourceTerms) map[string]struct{} { return stringSetOf(entry.pathTerms) })) {
		t.Fatal("parallel path-term inversion differs from invertPresence")
	}
	if counted.keyCount() == 0 || words.keyCount() == 0 || pathTerms.keyCount() == 0 {
		t.Fatal("the fixture produced an empty table, so the comparison proves nothing")
	}
}
