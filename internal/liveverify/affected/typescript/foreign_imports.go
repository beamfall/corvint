// SPDX-License-Identifier: Apache-2.0
package typescript

import (
	"encoding/json"
	"net/url"
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
					target, understood := packageImportFileTarget(directory, value)
					if !understood {
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

// packageImportFileTarget admits only relative file URLs with nonempty path
// components made of ASCII letters, digits, space, dot, underscore or hyphen.
// Decode each component separately: queries/fragments do not name file bytes,
// and encoded separators, dot segments and all other forms remain unproved.
func packageImportFileTarget(directory, value string) (string, bool) {
	if !strings.HasPrefix(value, "./") {
		return "", false
	}
	// URL preprocessing differs for raw whitespace/control bytes. Require
	// printable ASCII input; filenames needing other bytes use percent escapes.
	for _, c := range value {
		if c < '!' || c > '~' {
			return "", false
		}
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Opaque != "" || u.User != nil {
		return "", false
	}
	components := strings.Split(strings.TrimPrefix(u.EscapedPath(), "./"), "/")
	for i, component := range components {
		decoded, err := url.PathUnescape(component)
		if err != nil || decoded == "" || decoded == "." || decoded == ".." {
			return "", false
		}
		for _, c := range decoded {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == ' ' || c == '.' || c == '_' || c == '-') {
				return "", false
			}
		}
		components[i] = decoded
	}
	target := path.Join(directory, strings.Join(components, "/"))
	return target, affected.ValidRelativePath(target)
}
