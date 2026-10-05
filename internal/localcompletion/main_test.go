package localcompletion

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain points TMPDIR at its resolved path: local state refuses symlinked
// ancestors, and the macOS default temp root sits under /var -> /private/var.
func TestMain(m *testing.M) {
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		os.Setenv("TMPDIR", dir)
	}
	os.Exit(m.Run())
}
