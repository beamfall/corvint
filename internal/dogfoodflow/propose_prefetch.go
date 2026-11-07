package dogfoodflow

import "github.com/Beamfall/corvint/internal/cem/patch"

// patchHunkPaths names each parsed hunk display path, the BASE read set the
// draft prefetches through prefetchBlobs before it walks the hunks (V1-4095).
func patchHunkPaths(hunks []*patch.Hunk) []string {
	paths := make([]string, 0, len(hunks))
	for _, hunk := range hunks {
		paths = append(paths, hunk.DisplayPath)
	}
	return paths
}

// matchedPaths names every path a candidate span line is on.
func matchedPaths(matches map[string][]spanLine) []string {
	paths := []string{}
	for _, found := range matches {
		for _, line := range found {
			paths = append(paths, line.path)
		}
	}
	return paths
}
