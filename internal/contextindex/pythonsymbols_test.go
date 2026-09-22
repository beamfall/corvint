package contextindex

import "testing"

func pythonTestSource(text string) (Source, string) {
	return Source{Path: "pkg/module.py", BlobHash: "b1"}, text
}

// TestPythonSymbolsReportsNestedDefinitions pins the oracle's walk semantics:
// every function and class is reported at any nesting depth, a coroutine is a
// plain function, and the line is the definition's own line.
func TestPythonSymbolsReportsNestedDefinitions(t *testing.T) {
	source, text := pythonTestSource("class Outer:\n    def method(self):\n        pass\n\n    async def fetch(self):\n        pass\n\n\ndef free():\n    def inner():\n        pass\n    return inner\n")
	symbols := pythonSymbols(source, text)
	// EndLine is the oracle's ast end_lineno for the same node, taken from
	// CPython for this exact source: Outer 1-6, method 2-3, fetch 5-6,
	// free 9-12, inner 10-11.
	want := []Symbol{
		{Kind: "class", Name: "Outer", Path: "pkg/module.py", BlobHash: "b1", Line: 1, EndLine: 6},
		{Kind: "func", Name: "method", Path: "pkg/module.py", BlobHash: "b1", Line: 2, EndLine: 3},
		{Kind: "func", Name: "fetch", Path: "pkg/module.py", BlobHash: "b1", Line: 5, EndLine: 6},
		{Kind: "func", Name: "free", Path: "pkg/module.py", BlobHash: "b1", Line: 9, EndLine: 12},
		{Kind: "func", Name: "inner", Path: "pkg/module.py", BlobHash: "b1", Line: 10, EndLine: 11},
	}
	if len(symbols) != len(want) {
		t.Fatalf("symbols = %d (%v), want %d", len(symbols), symbols, len(want))
	}
	for index, expected := range want {
		if symbols[index] != expected {
			t.Errorf("symbol %d = %+v, want %+v", index, symbols[index], expected)
		}
	}
}

// TestPythonSymbolsReportsNothingForRejectedSource matches the oracle, which
// yields no symbols at all when its parser rejects the file. A partial walk
// would name a subset of the file's definitions as if it were all of them.
func TestPythonSymbolsReportsNothingForRejectedSource(t *testing.T) {
	// A malformed parameter list is outside the closed 3.12 subset, so the
	// grammar rejects the whole source. CPython rejects it too, which is what
	// makes it a sound fixture: the invariant under test is that a rejected file
	// yields nothing, never that this particular syntax is unsupported.
	source, text := pythonSymbolsRejected()
	if symbols := pythonSymbols(source, text); len(symbols) != 0 {
		t.Fatalf("symbols = %v, want none for a rejected source", symbols)
	}
}

func pythonSymbolsRejected() (Source, string) {
	return pythonTestSource("def kept():\n    pass\n\n\ndef broken(:\n    pass\n")
}

// TestPythonSymbolsAcceptsSourceWithoutTrailingNewline covers a file whose last
// line is unterminated. Both the grammar and the oracle's parser accept it, and
// the definitions are the same either way.
func TestPythonSymbolsAcceptsSourceWithoutTrailingNewline(t *testing.T) {
	source, text := pythonTestSource("def only():\n    return 1")
	symbols := pythonSymbols(source, text)
	if len(symbols) != 1 || symbols[0].Name != "only" || symbols[0].Line != 1 {
		t.Fatalf("symbols = %+v, want one func only at line 1", symbols)
	}
}
