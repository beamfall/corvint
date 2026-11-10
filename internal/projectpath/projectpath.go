// Package projectpath normalizes existing and verified-absent project paths.
package projectpath

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// LocalInputRule names the rule a local input path breaks, or returns "" for a canonical
// slash path relative to the input root (DCP-V1-054): never empty, over 1024 bytes, non-UTF-8,
// carrying a backslash or control byte, absolute, a parent traversal, the root itself or
// non-canonical. The corpus reader and the MCP corpus-artifact option apply the same rule.
func LocalInputRule(value string) string {
	switch {
	case value == "":
		return "got an empty path"
	case len(value) > 1024:
		return "got a path over 1024 bytes"
	case !utf8.ValidString(value):
		return "got a path that is not UTF-8"
	case strings.ContainsAny(value, "\\\x00\r\n\t"):
		return "got a backslash, NUL, tab, CR or LF"
	case strings.HasPrefix(value, "/"):
		return "got an absolute path"
	case value == ".." || strings.HasPrefix(value, "../"):
		return "got a parent traversal"
	case value == ".":
		return "got the root itself"
	case path.Clean(value) != value:
		return "got a non-canonical path"
	}
	return ""
}

// Relative returns a slash path strictly below the resolved root. Missing paths
// are admitted only below a contained ancestor with an entirely absent suffix.
// Unresolvable paths abstain. This is classification, not a race-free file opener.
func Relative(root, path string) (string, bool) {
	if path == "" {
		return "", false
	}
	// Callers pass slash paths; Windows splitting below trims only the native separator.
	path = filepath.FromSlash(path)
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
				target = filepath.FromSlash(target)
				switch {
				case filepath.IsAbs(target):
				case filepath.VolumeName(target) == "" && strings.HasPrefix(target, string(filepath.Separator)):
					// Windows resolves a rooted \x on the link's own volume, not below its parent.
					target = filepath.VolumeName(path) + target
				default:
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
			if len(suffix) > 0 {
				// Windows reports a path below a regular file as ERROR_PATH_NOT_FOUND,
				// which os.IsNotExist accepts, so the walk can stop at a file. An
				// existing ancestor of absent components must be a directory; a
				// dangling link's absent target was already proven absent above.
				if dir, err := os.Stat(resolved); err == nil && !dir.IsDir() || err != nil && !os.IsNotExist(err) {
					return "", false
				}
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
