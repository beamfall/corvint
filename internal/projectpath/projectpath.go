// Package projectpath normalizes existing and verified-absent project paths.
package projectpath

import (
	"os"
	"path/filepath"
	"strings"
)

// Relative returns a slash path strictly below the resolved root. Missing paths
// are admitted only below a contained ancestor with an entirely absent suffix.
// Unresolvable paths abstain. This is classification, not a race-free file opener.
func Relative(root, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", false
	}
	resolvedRoot, err = filepath.Abs(resolvedRoot)
	if err != nil {
		return "", false
	}
	if !filepath.IsAbs(path) {
		// Join would erase '..' before the filesystem can resolve preceding links.
		path = resolvedRoot + string(filepath.Separator) + path
	}
	resolved, ok := resolve(resolvedRoot, path, 40)
	if !ok {
		return "", false
	}
	relative, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || relative == "." {
		return "", false
	}
	return filepath.ToSlash(relative), true
}

func contained(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func resolve(root, path string, linksLeft int) (string, bool) {
	var suffix []string
	for {
		info, err := os.Lstat(path)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(path)
			if err != nil {
				if !os.IsNotExist(err) || info.Mode()&os.ModeSymlink == 0 || linksLeft == 0 {
					return "", false
				}
				// A dangling link exists; verify its target instead of treating the
				// link name as an absent component. Bound chains and cycles.
				target, err := os.Readlink(path)
				if err != nil {
					return "", false
				}
				if !filepath.IsAbs(target) {
					parent, _ := filepath.Split(path)
					target = parent + target
				}
				var ok bool
				resolved, ok = resolve(root, target, linksLeft-1)
				if !ok {
					return "", false
				}
			}
			if !contained(root, resolved) {
				return "", false
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
				if _, err := os.Lstat(resolved); !os.IsNotExist(err) {
					return "", false
				}
			}
			return resolved, true
		}
		if !os.IsNotExist(err) {
			return "", false
		}
		parent, component := filepath.Split(strings.TrimRight(path, string(filepath.Separator)))
		if component == "" || component == "." || component == ".." {
			return "", false
		}
		suffix = append(suffix, component)
		path = strings.TrimRight(parent, string(filepath.Separator))
		if path == "" {
			path = string(filepath.Separator)
		}
	}
}
