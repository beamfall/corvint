package contextindex

import (
	"path"
	"sort"
)

// ExpandImpactDirectories replaces each operand named in directories with the
// indexed files directly inside it whose suffix ImpactPathAdmitted admits, in
// path order, so a package directory reads exactly as naming those files does
// (GPK-V0-883). Subdirectories are not descended: one operand stays one
// package, and the result stays within the path bound a recursive walk of a
// source tree would exceed. Every other operand is kept in place. A directory
// that expands to nothing is returned as empty, with no paths, so the caller
// refuses it by name rather than dropping it.
func ExpandImpactDirectories(index *Index, paths []string, directories map[string]bool) (expanded []string, empty string) {
	if len(directories) == 0 {
		return paths, ""
	}
	members := make(map[string][]string, len(directories))
	for sourcePath := range index.Sources {
		if parent := path.Dir(sourcePath); directories[parent] && ImpactPathAdmitted(sourcePath) {
			members[parent] = append(members[parent], sourcePath)
		}
	}
	for _, value := range paths {
		if !directories[value] {
			expanded = append(expanded, value)
			continue
		}
		if len(members[value]) == 0 {
			return nil, value
		}
		sort.Strings(members[value])
		expanded = append(expanded, members[value]...)
	}
	return expanded, ""
}
