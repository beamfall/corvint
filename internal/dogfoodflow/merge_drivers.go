// SPDX-License-Identifier: Apache-2.0
package dogfoodflow

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/mergedrivers"
)

// registryMergePath is deliberately narrower than .corvint/*.json: evidence maps,
// policies and receipts are not registries and acquire no new derivation.
func registryMergePath(path string) bool {
	return path == ".corvint/shell-foreign-callers.json" || path == ".corvint/test-read-scopes.json" || path == ".corvint/shell-read-scopes.json"
}

type driverStage struct{ mode, object string }

// driverMerge resolves only recognized content conflicts inside the attribute-free
// scratch repository. The full tree, including modes, must equal the candidate;
// no candidate bytes are inputs to a rule or to regeneration (DWC-V0-7704).
func (b *batch) driverMerge(tree, conflicted, first, second string) string {
	fields := strings.Split(conflicted, "\x00")
	if !fullRevision.MatchString(fields[0]) {
		return ""
	}
	stages := map[string]map[int]driverStage{}
	for _, entry := range fields[1:] {
		if entry == "" {
			break
		}
		meta, path, ok := strings.Cut(entry, "\t")
		parts := strings.Fields(meta)
		if !ok || len(parts) != 3 || parts[0] != "100644" || !fullRevision.MatchString(parts[1]) {
			return ""
		}
		stage, err := strconv.Atoi(parts[2])
		if err != nil || stage < 1 || stage > 3 {
			return ""
		}
		if stages[path] == nil {
			stages[path] = map[int]driverStage{}
		}
		if _, duplicate := stages[path][stage]; duplicate {
			return ""
		}
		stages[path][stage] = driverStage{parts[0], parts[1]}
	}
	// Git calls a custom driver whenever both blobs changed, even if a text
	// merge would be clean. In particular keep-ours must also be replayed for
	// disjoint projection edits. Discover those paths from the parents alone.
	base, status := b.scratchGit("merge-base", "--all", first, second)
	base = chomp(base)
	if status != 0 || !fullRevision.MatchString(base) {
		return ""
	}
	changes := map[string]int{}
	for _, parent := range []string{first, second} {
		changed, status := b.scratchGit("diff-tree", "-r", "--no-commit-id", "--name-only", "--no-renames", "-z", base, parent)
		if status != 0 {
			return ""
		}
		for _, path := range strings.Split(changed, "\x00") {
			if driverPath(path) {
				changes[path]++
			}
		}
	}
	for path, count := range changes {
		if count != 2 {
			continue
		}
		s := map[int]driverStage{}
		for i, revision := range []string{base, first, second} {
			entry, status := b.scratchGit("ls-tree", "-z", revision, "--", ":(literal)"+path)
			if status != 0 {
				return ""
			}
			if entry == "" {
				continue
			}
			meta, foundPath, ok := strings.Cut(strings.TrimSuffix(entry, "\x00"), "\t")
			parts := strings.Fields(meta)
			if !ok || foundPath != path || len(parts) != 3 || parts[0] != "100644" || parts[1] != "blob" || !fullRevision.MatchString(parts[2]) {
				return ""
			}
			s[i+1] = driverStage{parts[0], parts[2]}
		}
		if s[2].object == "" || s[3].object == "" {
			return ""
		}
		if s[2].object != s[3].object {
			stages[path] = s
		}
	}
	if len(stages) == 0 {
		return ""
	}
	if _, status := b.scratchGit("read-tree", fields[0]); status != 0 {
		return ""
	}
	appendUsed, jsonUsed, generated := false, false, false
	paths := make([]string, 0, len(stages))
	for path := range stages {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		s := stages[path]
		// Missing sides are deletes, not content conflicts. Add/add has no base.
		if s[2].object == "" || s[3].object == "" {
			return ""
		}
		var data [3][]byte
		for i := range data {
			if s[i+1].object == "" {
				continue
			}
			blob, status := b.scratchGit("cat-file", "blob", s[i+1].object)
			if status != 0 {
				return ""
			}
			data[i] = []byte(blob)
		}
		var resolved []byte
		switch {
		case slices.Contains(generatedSpecFiles, path):
			resolved = data[1]
			generated = true
		case strings.HasPrefix(path, "docs/specs/") && filepath.Dir(path) == "docs/specs" && strings.HasSuffix(path, ".md"):
			var clean bool
			var err error
			resolved, clean, err = mergedrivers.Append(b.ctx, b.scratch, data[0], data[1], data[2], 7)
			if err != nil || !clean {
				return ""
			}
			appendUsed = true
		case registryMergePath(path):
			if data[0] == nil {
				data[0] = []byte("{}")
			}
			var err error
			resolved, err = mergedrivers.JSON(data[0], data[1], data[2])
			if err != nil {
				return ""
			}
			jsonUsed = true
		default:
			return ""
		}
		file, err := os.CreateTemp(b.scratch, "driver-blob-")
		if err != nil {
			return ""
		}
		name := file.Name()
		_, err = file.Write(resolved)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			os.Remove(name)
			return ""
		}
		object, status := b.scratchGit("hash-object", "-w", "--", name)
		os.Remove(name)
		if status != 0 || !fullRevision.MatchString(chomp(object)) {
			return ""
		}
		if _, status := b.scratchGit("update-index", "--add", "--cacheinfo", "100644", chomp(object), path); status != 0 {
			return ""
		}
	}
	written, status := b.scratchGit("write-tree")
	if status != 0 {
		return ""
	}
	written = chomp(written)
	if generated || written != tree {
		// Only generated projections may differ after the driver rules. Replay
		// preserves the parent-derived README/INDEX fields, never the candidate's.
		diff, status := b.scratchGit("diff-tree", "-r", "--name-only", "--no-renames", written, tree)
		if status != 0 {
			return ""
		}
		for _, path := range nonEmpty(strings.Split(chomp(diff), "\n")) {
			if !slices.Contains(generatedSpecFiles, path) {
				return ""
			}
		}
		if !b.replaySpecs(written, tree) {
			return ""
		}
		generated = true
	}
	var names []string
	if appendUsed {
		names = append(names, "append-union")
	}
	if jsonUsed {
		names = append(names, "json-union")
	}
	if generated {
		names = append(names, "regenerated-specs")
	}
	return strings.Join(names, "+")
}

func driverPath(path string) bool {
	return slices.Contains(generatedSpecFiles, path) || registryMergePath(path) ||
		(filepath.Dir(path) == "docs/specs" && strings.HasSuffix(path, ".md"))
}
