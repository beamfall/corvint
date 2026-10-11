package contextindex

import "fmt"

// mixedImpactWarning describes the immutable answer binding, never mutable content.
func mixedImpactWarning(index *Index) string {
	commit := index.CommitRevision
	if commit == "" {
		commit = "UNKNOWN"
	}
	return fmt.Sprintf("MIXED worktree: impact is bound to commit %s; %d dirty paths; mutable content is excluded.", commit, len(index.DirtyPaths))
}

func impactRequestOutsideDirtySet(index *Index, request map[string]any) bool {
	if len(index.DirtyPaths) == 0 {
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
// The caller must have observed a nonempty DirtyPaths set and validated CLI options.
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
