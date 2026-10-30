package contextindex

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// impactDirtyPathsShown bounds the dirty paths a MIXED impact warning names
// (DIRTY-CACHE-018); the rest are counted, matching affected's detail.
const impactDirtyPathsShown = 8

// mixedImpactWarning describes the immutable answer binding, never mutable
// content. It names up to impactDirtyPathsShown dirty paths in sorted order
// and counts the rest.
func mixedImpactWarning(index *Index) string {
	return mixedImpactWarningNaming(index, impactDirtyPathsShown)
}

// mixedImpactWarningNaming names at most shown dirty paths. A budgeted
// receipt starts from shown=0 and names only what fits after evidence is
// selected, so the disclosure never evicts a result (DIRTY-CACHE-018).
func mixedImpactWarningNaming(index *Index, shown int) string {
	commit := index.CommitRevision
	if commit == "" {
		commit = "UNKNOWN"
	}
	paths := append([]string(nil), index.DirtyPaths...)
	sort.Strings(paths)
	shown = min(max(shown, 0), len(paths))
	named := make([]string, 0, shown)
	for _, path := range paths[:shown] {
		named = append(named, strconv.Quote(path))
	}
	detail := "dirty: " + strings.Join(named, ", ")
	switch {
	case shown == 0:
		detail = "dirty path names omitted"
	case shown < len(paths):
		detail += fmt.Sprintf(" (+%d more)", len(paths)-shown)
	}
	return fmt.Sprintf("MIXED worktree: impact is bound to commit %s; %d dirty paths; mutable content is excluded; %s.", commit, len(paths), detail)
}

func impactRequestOutsideDirtySet(index *Index, request map[string]any) bool {
	if !index.SourceDirty() {
		return false
	}
	dirty := make(map[string]struct{}, len(index.DirtyPaths))
	for _, value := range index.DirtyPaths {
		dirty[value] = struct{}{}
	}
	paths, _ := request["paths"].([]string)
	for _, value := range paths {
		if _, ok := dirty[value]; !ok {
			return true
		}
	}
	return false
}

// ImpactCleanAbstention returns a bounded refusal before ranking or provider reads.
// The caller must have observed SourceDirty and validated CLI options.
func ImpactCleanAbstention(index *Index, paths []string, limit int, budget *int) (map[string]any, error) {
	cleaned, err := cleanImpactRequest(paths, limit)
	if err != nil {
		return nil, err
	}
	result, err := receipt(index, "impact", map[string]any{"paths": cleaned, "limit": limit}, nil, limit, "NEEDS_WIDENING")
	if err != nil {
		return nil, err
	}
	result["warning"] = mixedImpactWarning(index)
	result["abstention"] = map[string]any{"active": true, "reason": "mixed-worktree"}
	return compileReceipt(result, budget, index)
}

func cleanImpactRequest(paths []string, limit int) ([]string, error) {
	if limit < 1 || limit > maxLimit {
		return nil, &Error{Message: fmt.Sprintf("limit must be an integer from 1 to %d", maxLimit)}
	}
	if len(paths) == 0 {
		return nil, &Error{Message: "impact paths must be a non-empty list"}
	}
	if len(paths) > maxImpactPaths {
		return nil, &Error{Message: fmt.Sprintf("impact paths exceed %d-path bound", maxImpactPaths)}
	}
	cleanedSet := make(map[string]struct{}, len(paths))
	for _, value := range paths {
		cleaned, err := cleanImpactPath(value)
		if err != nil {
			return nil, err
		}
		cleanedSet[cleaned] = struct{}{}
	}
	return keys(cleanedSet), nil
}

// observedCommit prefers the opening observation's commit over the snapshot
// writer's, which a same-tree snapshot may carry from an earlier commit.
func observedCommit(observed, stored string) string {
	if observed != "" {
		return observed
	}
	return stored
}
