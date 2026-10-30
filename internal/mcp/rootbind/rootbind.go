// Package rootbind pins an MCP repository root's Git marker: a real .git
// directory, or a regular gitfile whose worktree and common directories exist.
// It is the root check of corvint-mcp (bridge.New) lifted for corvint-docs-mcp (V1-2816);
// bridge.New and corvint-docs-mcp both use it.
package rootbind

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
)

// Binding is the observed Git identity of a resolved root.
type Binding struct {
	Marker      os.FileInfo
	Directories []string // worktree git dir and common dir; empty for a .git directory
	identities  []os.FileInfo
}

// Bind observes root/.git. A symlink, a missing marker, or a gitfile whose
// directories are not real directories is refused.
func Bind(resolved string) (*Binding, bool) {
	marker, err := os.Lstat(filepath.Join(resolved, ".git"))
	if err != nil || (!marker.IsDir() && !marker.Mode().IsRegular()) || marker.Mode()&os.ModeSymlink != 0 {
		return nil, false
	}
	binding := &Binding{Marker: marker}
	if marker.Mode().IsRegular() {
		gitDir, commonDir, err := gitauth.WorktreeDirectories(resolved)
		if err != nil {
			return nil, false
		}
		binding.Directories = []string{gitDir, commonDir}
		for _, path := range binding.Directories {
			info, err := os.Lstat(path)
			if err != nil || !info.IsDir() {
				return nil, false
			}
			binding.identities = append(binding.identities, info)
		}
	}
	return binding, true
}

// Same reports whether root still has the bound Git identity.
func (binding *Binding) Same(root string) bool {
	marker, err := os.Lstat(filepath.Join(root, ".git"))
	if err != nil || marker.Mode()&os.ModeSymlink != 0 || !os.SameFile(binding.Marker, marker) {
		return false
	}
	if len(binding.Directories) == 0 {
		return marker.IsDir()
	}
	gitDir, commonDir, err := gitauth.WorktreeDirectories(root)
	if err != nil || gitDir != binding.Directories[0] || commonDir != binding.Directories[1] {
		return false
	}
	for i, path := range binding.Directories {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || !os.SameFile(info, binding.identities[i]) {
			return false
		}
	}
	return true
}
