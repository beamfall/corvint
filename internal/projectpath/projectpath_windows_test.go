package projectpath

import (
	"os"
	"path/filepath"
	"testing"
)

// TestRelativeRootedDanglingTargetUsesTheLinkVolume pins that a dangling link to
// a rooted target (\x, no volume) is resolved on the link's volume, as Windows
// does, so a target outside the root is not classified as contained (V1-2488).
func TestRelativeRootedDanglingTargetUsesTheLinkVolume(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink(`\corvint-v1-2488-absent\new`, filepath.Join(root, "rooted")); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if got, ok := Relative(root, "rooted"); ok {
		t.Fatalf("Relative(root, rooted) = %q, true; the target is outside the root", got)
	}
}
