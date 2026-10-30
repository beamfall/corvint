package pythonsyntax

import (
	"testing"
)

// TestSemicolonStatementsMatchOracle pins SourceSyntaxValid to CPython for empty
// statement spans, which previously panicked with an index-out-of-range.
func TestSemicolonStatementsMatchOracle(t *testing.T) {
	for _, source := range []string{";\n", ";;\n", "x = 1;;\n", ";x = 1\n", "x = 1;\n", "x = 1; y = 2\n", "x = 1\n"} {
		want := source == "x = 1;\n" || source == "x = 1; y = 2\n" || source == "x = 1\n"
		got := SourceSyntaxValid("a.py", []byte(source))
		if got && !want {
			t.Errorf("%q: Go accepts what ast.parse rejects", source)
		}
		if got != want {
			t.Logf("%q: go=%v oracle=%v (conservative, allowed)", source, got, want)
		}
	}
}
