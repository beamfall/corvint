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
	dir := regexp.QuoteMeta(directory)
	patterns := []string{
		// The separator may be in the next operand: "run" + "/" + name.
		`["'\x60](?:\./)?` + dir + `/?["'\x60]\s*\+`,
		`(?:[Jj]oin|joinpath)\s*\(\s*["'\x60](?:\./)?` + dir + `["'\x60]\s*,`,
		`["'\x60](?:\./)?` + dir + `/[^"'\x60\n]*[$%{]`,
	}
	if directory == "." {
		// A computed basename needs no directory literal. Without one we
		// cannot exclude a root callee from joins, concatenations or formats;
		// conservatively require these possible callers to be declared.
		patterns = []string{
			`(?:[Jj]oin|joinpath)\s*\(`,
			`["'\x60][^"'\x60\n]*["'\x60]\s*\+|\+\s*["'\x60]`,
			`["'\x60][^"'\x60\n]*[$%{]`,
		}
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
