//go:build !darwin && !freebsd && !linux && !windows

package worktreeimpact

import (
	"os"
	"path/filepath"
	"testing"
)

// PRS-V1-019 / PRS-V1-004 (V1-0393): an unqualified platform refuses even a
// regular working-tree target.
func TestReadTargetOnceOtherRefusesRegularFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err := readTargetOnce(root, "a.go"); err == nil {
		t.Fatal("unqualified platform read a working-tree target")
	}
}
