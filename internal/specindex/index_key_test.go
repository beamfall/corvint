package specindex

import (
	"path/filepath"
	"testing"
)

// TestIndexKeyTrimsTheRepositoryPrefix pins the INDEX key for a glob match in
// the platform's own separator, so the Windows run cannot report a spec as a
// missing "../../docs/specs/..." entry again.
func TestIndexKeyTrimsTheRepositoryPrefix(t *testing.T) {
	file := filepath.Join("..", "..", "docs", "specs", "example-v0.md")
	if got := indexKey(file); got != "docs/specs/example-v0.md" {
		t.Fatalf("indexKey(%q) = %q", file, got)
	}
}
