// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"io/fs"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// literalLinks preserves every alias and its target as read dependencies. Resolve
// components before cleaning: alias/../file follows alias before climbing. A
// missing leaf is still a dependency; unknown links, cycles and escapes fail closed.
func literalLinks(root *affected.Source, directory, literal string, listings map[string][]fs.DirEntry) ([]string, bool) {
	if strings.ContainsAny(literal, "\\:") || strings.HasPrefix(literal, "/") {
		return nil, false
	}
	pending := strings.Split(literal, "/")
	current := directory
	var links []string
	for len(pending) > 0 {
		component := pending[0]
		pending = pending[1:]
		if component == "" || component == "." {
			continue
		}
		if component == ".." {
			if current == "." {
				return links, len(links) > 0
			}
			current = path.Dir(current)
			continue
		}
		entries, known := listings[current]
		if !known {
			var err error
			entries, err = root.ReadDir(current)
			if err != nil {
				return links, true
			}
			listings[current] = entries
		}
		next := path.Join(current, component)
		var entry fs.DirEntry
		for _, candidate := range entries {
			if candidate.Name() == component {
				entry = candidate
				break
			}
		}
		if entry == nil {
			if len(links) > 0 {
				target := path.Join(append([]string{next}, pending...)...)
				if !fs.ValidPath(target) {
					return links, true
				}
				links = append(links, target)
			}
			return links, false
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			links = append(links, next)
			if len(links) > 40 {
				return links, true
			}
			target, err := root.ReadLink(next)
			if err != nil || target == "" || strings.HasPrefix(target, "/") || strings.ContainsAny(target, "\\:") {
				return links, true
			}
			pending = append(strings.Split(target, "/"), pending...)
			continue
		}
		current = next
		if len(pending) > 0 && !entry.IsDir() {
			return links, len(links) > 0
		}
	}
	if len(links) > 0 {
		links = append(links, current)
	}
	return links, false
}

func literalLinkGlob(relative string) string {
	return "/" + strings.NewReplacer("\\", "\\\\", "*", "\\*", "?", "\\?", "[", "\\[", "]", "\\]").Replace(relative)
}
