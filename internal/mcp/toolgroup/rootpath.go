package toolgroup

import (
	"path/filepath"
	"strings"
)

// RootRelativePath rewrites an absolute --artifact or --map inside root to the equivalent
// root-relative path, so both spellings open the same file and report the same path
// (DCP-V1-205). Containment is decided lexically first and then on symlink-resolved paths,
// so a root reached through a symlinked ancestor (macOS /var -> /private/var) matches either
// spelling (DCP-V1-206). An absolute path that is not below root is refused with a reason
// instead of the generic repository-unavailable refusal (DCP-V1-207). A relative value is
// returned unchanged; the bounded reader still applies its path rules.
func RootRelativePath(root, artifact string) (string, string) {
	if !filepath.IsAbs(artifact) {
		return artifact, ""
	}
	lexicalRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "--root cannot be made absolute"
	}
	candidate := filepath.Clean(artifact)
	if relative, inside := containedPath(lexicalRoot, candidate); inside {
		return relative, ""
	}
	realRoot, rootErr := filepath.EvalSymlinks(lexicalRoot)
	realParent, parentErr := filepath.EvalSymlinks(filepath.Dir(candidate))
	if rootErr == nil && parentErr == nil {
		if relative, inside := containedPath(realRoot, filepath.Join(realParent, filepath.Base(candidate))); inside {
			return relative, ""
		}
	}
	return "", "absolute path is not below --root"
}

// containedPath reports the slash-separated path of target relative to root when target is
// strictly below root.
func containedPath(root, target string) (string, bool) {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return filepath.ToSlash(relative), true
}
