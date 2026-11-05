// SPDX-License-Identifier: Apache-2.0

package shell

import (
	"bytes"
	"path"
	"regexp"
)

// ForeignCallerReference compiles the literal and constructed reference check
// for a declared callee. The repository gate applies it to every tracked
// executable-source candidate, including files outside the plugin's walk.
func ForeignCallerReference(script string) func([]byte) bool {
	base := []byte(path.Base(script))
	constructed := constructedForeignMatcher(script)
	return func(body []byte) bool { return bytes.Contains(body, base) || constructed(body) }
}

// constructedForeignMatcher is deliberately conservative. In addition to
// shell command expansion, a foreign caller can assemble a callee with a
// directory concatenation or a path-join call. Every script below that
// directory is a possible target; the project must list that caller for each.
// AZU-V0-004, AZU-V0-8901. This is a completeness guard, not an edge inference.
func constructedForeignMatcher(script string) func([]byte) bool {
	directory := path.Dir(script)
	if directory == "." {
		return func([]byte) bool { return false }
	}
	dir := regexp.QuoteMeta(directory)
	patterns := []string{
		`["'\x60](?:\./)?` + dir + `/["'\x60]\s*\+`,
		`(?:[Jj]oin|joinpath)\s*\(\s*["'](?:\./)?` + dir + `["']\s*,`,
		`["'](?:\./)?` + dir + `/[^"'\n]*[$%{]`,
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		compiled = append(compiled, regexp.MustCompile(pattern))
	}
	return func(body []byte) bool {
		for _, pattern := range compiled {
			if pattern.Match(body) {
				return true
			}
		}
		return false
	}
}

func constructedForeignReference(body []byte, script string) bool {
	return constructedForeignMatcher(script)(body)
}
