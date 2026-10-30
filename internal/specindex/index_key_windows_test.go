package specindex

import "testing"

// TestIndexKeyAcceptsWindowsGlobMatches pins the literal form filepath.Glob
// returns on Windows.
func TestIndexKeyAcceptsWindowsGlobMatches(t *testing.T) {
	if got := indexKey(`..\..\docs\specs\example-v0.md`); got != "docs/specs/example-v0.md" {
		t.Fatalf("indexKey = %q", got)
	}
}
