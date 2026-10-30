package pythonsyntax

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/pythongrammar"
)

// FuzzSourceSyntaxValidNeverPanics feeds repository-controlled Python bytes to
// the validator. It must return a verdict rather than panic, and it must never
// be more permissive than the unvalidated subset parser it layers over.
func FuzzSourceSyntaxValidNeverPanics(f *testing.F) {
	seeds, _ := filepath.Glob(filepath.Join("testdata", "ast-parity", "*.py"))
	for _, seed := range seeds {
		if data, err := os.ReadFile(seed); err == nil {
			f.Add(data)
		}
	}
	f.Add([]byte("def f(x, /, *a, k=1, **kw):\n    return (y := x) if f\"{x!r:>{k}}\" else lambda: 0x_1\n"))
	f.Fuzz(func(t *testing.T, source []byte) {
		valid := SourceSyntaxValid("fuzz.py", source)
		if valid && pythongrammar.ParsePython312Subset("fuzz.py", source, discardFacts{}) != "" {
			t.Fatalf("validator accepted %q, which the subset parser rejects", source)
		}
	})
}
