// SPDX-License-Identifier: Apache-2.0

package affected

import (
	"path"
	"strings"
)

// evaluatedCacheOnly reports whether the evaluated target of rule is a
// literal recipe apart from its Go build cache location (AFP-V0-3600), and
// returns the scans of that proof, whose edges the target keeps beside those
// of its default-invocation alternatives.
//
// The AFP-V0-3122 hedge keeps an evaluated target that runs an uncertified
// command a computed site, because a missed alternative would hide what it
// runs. The proof expands the target's prerequisites and recipe again with
// every make variable unknown: a name the file assigns or not, MAKE, every
// environment, command-line, target- or pattern-specific, override and
// define value, and every automatic variable except the target's own name.
// It holds only when no unknown part reaches a prerequisite or any shell
// word other than a GOCACHE value: every command, operand, directory,
// redirection, sourced file and make goal is then literal recipe text,
// whatever value make, the environment or the command line supplies.
func (registry *driverRegistry) evaluatedCacheOnly(parsed *driverMakeFile, rule *driverMakeRule, target string) ([]*driverScan, bool) {
	auto := map[string]string{}
	if !rule.pattern {
		auto = map[string]string{"@": target, "@D": path.Dir(target), "@F": path.Base(target)}
	}
	opaque := func() *driverExpander {
		expander := parsed.expander(auto)
		expander.opaque = true
		return expander
	}
	// exact reports an expansion that stayed within the modeled, effect-free
	// subset: no ambiguity, walk-bound overflow, unmodeled function or
	// effect, `$(shell ...)` command or file read.
	exact := func(expander *driverExpander) bool {
		return !expander.ambiguous && !expander.unmodeledEffects && !expander.shellAmbiguous &&
			len(expander.shells) == 0 && len(expander.readPaths) == 0
	}
	for _, line := range rule.prereqs {
		expander := opaque()
		for _, variant := range expander.expandAll(line) {
			if driverDynamic(variant) {
				return nil, false
			}
		}
		if !exact(expander) {
			return nil, false
		}
	}
	var scans []*driverScan
	for _, recipe := range rule.recipe {
		expander := opaque()
		variants := expander.expandAll(driverRecipeLine(recipe))
		if !exact(expander) {
			return nil, false
		}
		for _, variant := range variants {
			for _, cwd := range driverScanDirectories(parsed.dir) {
				scan := newDriverScan()
				scan.source = registry.source
				scan.strict = true
				scan.scanShell(variant, cwd, registry.module)
				if scan.computed || len(scan.leaks) != 0 {
					return nil, false
				}
				scans = append(scans, scan)
			}
		}
	}
	return scans, true
}

// driverScanDirectories are the working directories scanBoth scans a make
// file's recipe text with: the file's own and, when that is not the root,
// the root as well.
func driverScanDirectories(dir string) []string {
	if dir == "." {
		return []string{"."}
	}
	return []string{dir, "."}
}

// strictWords records, in an AFP-V0-3600 scan, each word of one simple
// command that holds an unknown make value, except the value of a GOCACHE
// assignment in the command's leading assignments and `env` operands, the
// one place AFP-V0-3128 admits any value.
func (s *driverShellScanner) strictWords(words []driverWord) {
	prefix, wrapped := true, false
	for _, word := range words {
		exempt := false
		if prefix {
			switch {
			case !word.quoted && driverAssignment(word.text):
				name, _, _ := strings.Cut(word.text, "=")
				exempt = name == "GOCACHE"
			case !word.quoted && !word.escaped && word.text == "env":
				wrapped = true
			case wrapped && strings.HasPrefix(word.text, "-"):
			default:
				prefix = false
			}
		}
		if !exempt && strings.Contains(word.text, driverComputed) {
			s.scan.leak(word.text)
		}
	}
}

// leak records a word of an AFP-V0-3600 scan that holds an unknown make value
// outside a GOCACHE value.
func (scan *driverScan) leak(word string) {
	if scan.leaks == nil {
		scan.leaks = map[string]bool{}
	}
	scan.leaks[driverReasonWord(strings.ReplaceAll(word, driverComputed, "$(?)"))] = true
}
