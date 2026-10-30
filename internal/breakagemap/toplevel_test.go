package breakagemap

import (
	"path/filepath"
	"testing"
)

// TestSameTopLevelComparesCleanNativePaths pins the `--show-toplevel` check:
// the trailing newline and a slash-separated spelling of the same checkout
// match, another directory does not (V1-2488).
func TestSameTopLevelComparesCleanNativePaths(t *testing.T) {
	canonical := filepath.Join(t.TempDir(), "checkout")
	if !sameTopLevel(filepath.ToSlash(canonical)+"\n", canonical) {
		t.Fatalf("top level %q does not match itself", canonical)
	}
	if sameTopLevel(filepath.ToSlash(filepath.Join(canonical, "sub"))+"\n", canonical) {
		t.Fatal("a subdirectory matched the checkout")
	}
}
