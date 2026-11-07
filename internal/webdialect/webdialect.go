// SPDX-License-Identifier: Apache-2.0

// Package webdialect is the TypeScript language plugin's impact profile
// (docs/specs/language-plugins-v0.md, LGP-V0-027; AFP-V0-2247): the bounded
// syntax profile of every impact dialect whose rule reads ECMAScript module
// imports. The web lexer that masks a module's comments and literals stays in
// internal/contextindex, whose flow, setup and registry readers share it, so
// core passes it in and registers the profile (LGP-V0-027). The package has no
// package-level variable with a computed initializer: its patterns are compiled
// when Impact builds the profile (ADN-V0-004), and it imports no core package
// (LGP-V0-020).
package webdialect

import (
	"regexp"
	"strings"

	"github.com/Beamfall/corvint/internal/langplugin/dialect"
)

// Impact returns the profile, masking a module's text with codeLines: the
// reflective and computed-dispatch probes, the dynamic import and require
// call pattern, and StaticLoad.
func Impact(codeLines func(text string) []string) dialect.Impact {
	return dialect.Impact{
		CodeLines: codeLines,
		Probes: []dialect.Probe{
			{Code: "observed-reflective-dispatch", Pattern: dialect.Reflection},
			{Code: "observed-computed-dispatch", Pattern: regexp.MustCompile(`\]\s*(?:\?\.)?\s*\(`)},
		},
		Loads:      regexp.MustCompile(`\b(?:import|require)\s*\(`),
		StaticLoad: StaticLoad,
	}
}

// StaticLoad recognizes only a complete quoted argument inside the call's
// parentheses. A quote-first expression can still concatenate a dynamic name.
func StaticLoad(tail string) bool {
	tail = strings.TrimSpace(tail)
	if len(tail) < 2 || (tail[0] != '\'' && tail[0] != '"') {
		return false
	}
	quote := tail[0]
	for i := 1; i < len(tail); i++ {
		if tail[i] == '\\' {
			i++
			continue
		}
		if tail[i] != quote {
			continue
		}
		return strings.HasPrefix(strings.TrimSpace(tail[i+1:]), ")")
	}
	return false
}
