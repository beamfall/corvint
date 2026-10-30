package localcompletion

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Beamfall/corvint/internal/testsupport"
)

// TestMain points TMPDIR at its resolved path: local state refuses symlinked
// ancestors, and the macOS default temp root sits under /var -> /private/var.
// It also detaches every git the tests spawn from the account's global and
// system configuration (testsupport.IsolateGit, V1-2633).
func TestMain(m *testing.M) {
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		os.Setenv("TMPDIR", dir)
	}
	testsupport.IsolateGit()
	os.Exit(m.Run())
}
