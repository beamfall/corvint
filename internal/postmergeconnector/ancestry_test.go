package postmergeconnector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestChangedFilesRefusesRewrittenAncestry_PMCV0010 pins that a grafted or
// shallow checkout yields ancestry-rewritten instead of trusting the rewritten
// merge-base answer, and that removing the rewrite restores the read.
func TestChangedFilesRefusesRewrittenAncestry_PMCV0010(t *testing.T) {
	for _, tc := range []struct{ name, file string }{
		{"grafts", filepath.Join(".git", "info", "grafts")},
		{"shallow", filepath.Join(".git", "shallow")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, f, _, _ := fixture(t)
			b := f.Forge.Change.Binding
			path := filepath.Join(root, tc.file)
			// The base is the boundary in both files: Git still answers that base
			// is an ancestor of merge, so only the guard refuses.
			if err := os.WriteFile(path, []byte(b.Base+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ChangedFiles(context.Background(), root, b); err == nil || err.Error() != "ancestry-rewritten" {
				t.Fatalf("ChangedFiles = %v, want ancestry-rewritten", err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if files, err := ChangedFiles(context.Background(), root, b); err != nil || len(files) != 1 {
				t.Fatalf("plain checkout: %v %v", files, err)
			}
		})
	}
}
