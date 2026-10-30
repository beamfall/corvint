package affected

import (
	"io/fs"
	"os"
	"path/filepath"
)

// String formats an unknown as its reason and detail, with the hint only when
// there is one, so a plan without hints prints exactly as before AFP-V0-494.
func (unknown Unknown) String() string {
	if unknown.Hint == "" {
		return "{" + unknown.Reason + " " + unknown.Detail + "}"
	}
	return "{" + unknown.Reason + " " + unknown.Detail + " " + unknown.Hint + "}"
}

// symlinkUnownedHint is AFP-V0-494's guidance for an unowned dirty path that
// is a symbolic link, such as a node_modules link into another checkout.
const symlinkUnownedHint = "this path is a symbolic link; a Git ignore pattern ending in `/` matches only a directory, never a symlink, so such a pattern does not ignore it (drop the trailing `/` to ignore the link)"

// symlinkHints annotates, in place, each UNOWNED_DIRTY_PATH whose path in the
// observed checkout is a symbolic link (AFP-V0-494). It reads only Lstat of
// that one path, never follows the link, and changes neither selection nor
// scope; a graph with no disk root, an invalid path, or a failed Lstat gets
// no hint.
func (graph *Graph) symlinkHints(unknown []Unknown) {
	if graph.root == "" {
		return
	}
	for index := range unknown {
		path := unknown[index].Detail
		if unknown[index].Reason != UnknownUnownedDirtyPath || !fs.ValidPath(path) || path == "." {
			continue
		}
		info, err := os.Lstat(filepath.Join(graph.root, filepath.FromSlash(path)))
		if err == nil && info.Mode()&fs.ModeSymlink != 0 {
			unknown[index].Hint = symlinkUnownedHint
		}
	}
}
