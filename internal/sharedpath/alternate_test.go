package sharedpath

import (
	"path/filepath"
	"testing"
)

// TestSingleAlternateRefusesListSyntax pins that the object store handed to
// GIT_ALTERNATE_OBJECT_DIRECTORIES is one entry: the platform list separator,
// a quote or a line break is refused (V1-2488).
func TestSingleAlternateRefusesListSyntax(t *testing.T) {
	store := filepath.Join(t.TempDir(), "repo", ".git", "objects")
	if !singleAlternate(store) {
		t.Fatalf("plain object store %q refused", store)
	}
	for _, suffix := range []string{string(filepath.ListSeparator) + "other", `"quoted`, "line\nbreak", "cr\rx"} {
		if singleAlternate(store + suffix) {
			t.Errorf("store with %q admitted", suffix)
		}
	}
}
