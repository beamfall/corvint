// Package tracerepopaths defines the current-tree path authority shared by trace adapters.
package tracerepopaths

import (
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// Paths admits index sources and tracked ignore rules without indexing the latter.
func Paths(index *contextindex.Index) []string {
	paths := make([]string, 0, len(index.Sources))
	for path := range index.Sources {
		paths = append(paths, path)
	}
	for path := range index.Tracked {
		if path == ".gitignore" || strings.HasSuffix(path, "/.gitignore") {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}
