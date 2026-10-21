// Package taskprojection recognises the Corvint Tasks store projection
// layout (AFP-V0-088, DIRTY-CACHE-017). It is a leaf package so that both the
// affected planner and the read-path freshness receipts classify a dirty
// `.taskman/` record the same way.
package taskprojection

import "strings"

// Class is the non-source class of a Tasks store projection file.
const Class = "TASK_STORE_PROJECTION"

// Dir is the repository-relative directory a Corvint Tasks store projects
// its intent into.
const Dir = ".taskman"

// IsPath reports whether a canonical repository-relative path is a file of
// the recognised Tasks store projection layout: `.taskman/policy.json`,
// `.taskman/queue.json`, or one `.taskman/tickets/<name>.json` or
// `.taskman/releases/<name>.json` record. Any other path under `.taskman/`
// is not recognised.
func IsPath(path string) bool {
	rest, inside := strings.CutPrefix(path, Dir+"/")
	if !inside {
		return false
	}
	if rest == "policy.json" || rest == "queue.json" {
		return true
	}
	for _, records := range []string{"tickets/", "releases/"} {
		name, record := strings.CutPrefix(rest, records)
		if record && strings.HasSuffix(name, ".json") && len(name) > len(".json") && !strings.Contains(name, "/") {
			return true
		}
	}
	return false
}

// OnlyProjection reports whether paths is non-empty and every path is a
// recognised projection file (IsPath).
func OnlyProjection(paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if !IsPath(path) {
			return false
		}
	}
	return true
}
