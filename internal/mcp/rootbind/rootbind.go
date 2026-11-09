// Package rootbind pins an MCP repository root's Git marker: a real .git
// directory, or a regular gitfile whose worktree and common directories exist.
// It is the root check of corvint-mcp (bridge.New) lifted for corvint-docs-mcp (V1-2816);
// bridge.New and corvint-docs-mcp both use it.
package rootbind

import (
	"os"
	"path/filepath"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/rootalias"
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

// The root failure reasons Resolve names on stderr (MCPV0-950). Each is fixed
// text: it never repeats the declared path or a Git or system error.
const (
	ReasonNotAbsolute  = "--root must be an absolute path"
	ReasonUnclean      = "--root must be a clean path of at most 4,096 bytes without control characters"
	ReasonMissing      = "--root does not exist"
	ReasonNotDirectory = "--root is not a directory"
	ReasonInGitDir     = "--root is inside a .git directory, not a worktree"
	ReasonNoWorktree   = "--root is not inside a Git worktree"
	ReasonMarker       = "the worktree's .git marker is a symlink, or a gitfile whose linked-worktree directories are missing"
)

// Resolve maps a declared absolute root to the Git worktree toplevel it lies
// in (MCPV0-950): the root itself when it holds the .git marker, else its
// nearest physical ancestor that does, as `git rev-parse --show-toplevel`
// would answer without consulting the environment. A root inside a .git
// directory, outside any worktree, or whose toplevel marker Bind refuses
// returns one of the Reason constants instead. A root that is already a
// toplevel is returned in its declared spelling.
func Resolve(root string) (toplevel, reason string) {
	if !filepath.IsAbs(root) {
		return "", ReasonNotAbsolute
	}
	if !rootalias.ValidRoot(root) {
		return "", ReasonUnclean
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", ReasonMissing
	}
	if info, err := os.Stat(resolved); err != nil || !info.IsDir() {
		return "", ReasonNotDirectory
	}
	for directory := resolved; ; directory = filepath.Dir(directory) {
		if filepath.Base(directory) == ".git" {
			return "", ReasonInGitDir
		}
		if _, err := os.Lstat(filepath.Join(directory, ".git")); err == nil {
			if _, ok := Bind(directory); !ok {
				return "", ReasonMarker
			}
			if directory == resolved {
				return root, ""
			}
			return directory, ""
		}
		if filepath.Dir(directory) == directory {
			return "", ReasonNoWorktree
		}
	}
}
