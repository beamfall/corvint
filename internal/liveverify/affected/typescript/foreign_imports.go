// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"encoding/json"
	"path"
	"strings"

	"github.com/Beamfall/corvint/internal/liveverify/affected"
)

// packageImportTargets retains local data reads through package.json imports
// (AZU-V1-960). All matching patterns and conditional alternatives are kept;
// this is a conservative read set, not executable module resolution. An
// unavailable table or unsupported target keeps the import's reads unknown.
func packageImportTargets(owner, ref string, bodies map[string]string) ([]string, bool) {
	if !strings.HasPrefix(ref, "#") {
		return nil, false
	}
	directory := path.Dir(owner)
	for {
		if body, found := bodies[path.Join(directory, "package.json")]; found {
			var manifest struct {
				Imports map[string]json.RawMessage `json:"imports"`
			}
			if json.Unmarshal([]byte(body), &manifest) != nil {
				return nil, true
			}
			var targets []string
			matched, bounded := false, false
			var visit func(json.RawMessage, string, int)
			visit = func(raw json.RawMessage, capture string, depth int) {
				if depth > 16 || len(targets) >= maxPathTokens {
					bounded = true
					return
				}
				var value string
				if json.Unmarshal(raw, &value) == nil {
					value = strings.ReplaceAll(value, "*", capture)
					target := path.Join(directory, value)
					if !strings.HasPrefix(value, "./") || !affected.ValidRelativePath(target) || strings.Contains(value, "\\") {
						bounded = true
						return
					}
					targets = append(targets, target)
					if path.Ext(target) == "" {
						targets = append(targets, target+".json")
					}
					return
				}
				var conditions map[string]json.RawMessage
				if json.Unmarshal(raw, &conditions) == nil && len(conditions) > 0 {
					for _, key := range sortedKeys(conditions) {
						visit(conditions[key], capture, depth+1)
					}
					return
				}
				// Null, arrays and external package targets are deliberately unproved.
				bounded = true
			}
			for _, pattern := range sortedKeys(manifest.Imports) {
				if capture, ok := typeScriptAliasMatch(pattern, ref); ok {
					matched = true
					visit(manifest.Imports[pattern], capture, 0)
				}
			}
			return targets, bounded || !matched
		}
		if directory == "." {
			return nil, true
		}
		directory = path.Dir(directory)
	}
}
