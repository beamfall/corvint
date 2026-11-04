// SPDX-License-Identifier: Apache-2.0

package gateselect

// Rule (c)'s naming relation for a dirty path other than the CEM sidecar and
// a sealed CEM, which resolvingReaders decides. It is the planner's Go-unit
// relation (internal/liveverify/affected: namesPath with the V1-0290
// file-name-only rule, then namesFileOnlyWithin with AFP-V0-072's placement
// and AFP-V0-2203's anchored resolution), duplicated here because this
// package is a stdlib-only trusted build:
//
//   - AFP-V0-2240: an anchored token (one that starts with "/") names a path
//     outside its holder's subtree only when, joined to the repository root,
//     the holder's directory or one of its ancestors, it is the path or an
//     ancestor of it, when a run of two or more of its components matches
//     anywhere, when a lone-component run is the path's file name (its root
//     may come from the environment), or when a lone-component run matches
//     a directory component after components a literal of the package
//     could join.
//   - AFP-V0-2241: a lone component of an unanchored, non-climbing token
//     names only the file name, and a package that only such tokens name
//     reaches the path in its own subtree or, from the root (or an ancestor
//     once a token climbs), through directory components its literals hold.
//
// This index's lexicon differs from the planner's (AFP-V0-765,
// TestFastTierLiteralLexiconDiffers_AFPV0765) only where it keeps more or
// broader tokens, with one exception: a `..\x` literal yields `..` and `x`
// (or `a/..` and `x`) here and the climbing `../x` there. A package holding a
// token that ends in `..` therefore treats every token as climbing
// (parentClimbs), so the
// relation here stays a superset of the planner's.
// TestRuleCMatchesThePlanner_AFPV02240 holds the two equal on one fixture.

import (
	"path"
	"slices"
	"strings"
)

// keepsReader reports whether directory, which holds value, a token that
// names dirtyPath by this index's namesPath, is a rule (c) reader of
// dirtyPath: value names it by the planner's relation (with the lone-token
// rule relaxed in a package holding a parent-only token), and the package
// reaches it (namesWithin). memo caches namesWithin per directory and path.
func (index *repositoryIndex) keepsReader(directory, value, dirtyPath string, memo map[string]bool) bool {
	climbing := index.parentClimbs(directory)
	if !climbing && !namesResolvedPath(value, dirtyPath) {
		return false
	}
	key := directory + "\x00" + dirtyPath
	kept, ok := memo[key]
	if !ok {
		kept = index.namesWithin(directory, dirtyPath, climbing)
		memo[key] = kept
	}
	return kept
}

// parentClimbs reports whether directory holds a token that a `..\x`
// literal may have split from the component it names: a parent-only token
// (`..`) or one that ends in a parent component (`a/..` of `a/..\notes`).
func (index *repositoryIndex) parentClimbs(directory string) bool {
	return slices.ContainsFunc(index.tokens[directory], func(value string) bool {
		return parentOnly(value) || strings.HasSuffix(value, "/..")
	})
}

// namesWithin is the planner's namesFileOnlyWithin for the Go package in
// directory over every token it holds ("" is the root package, which reaches
// every path). climbing makes every lone token a climbing one.
func (index *repositoryIndex) namesWithin(directory, dirtyPath string, climbing bool) bool {
	if directory == "" || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	tokens := index.tokens[directory]
	parts := strings.Split(dirtyPath, "/")
	bases := []string{""}
	for base := directory; base != "." && base != ""; base = path.Dir(base) {
		bases = append(bases, base)
	}
	climbs, named := climbing, false
	var prefixes []int
	for _, value := range tokens {
		if strings.HasPrefix(value, "/") {
			if anchoredWithin(bases, value, parts) || longRunNames(value, parts) {
				return true
			}
			offsets := runOffsets(value, parts)
			if slices.Contains(offsets, len(parts)-1) {
				return true
			}
			prefixes = append(prefixes, offsets...)
			climbs = climbs || slices.Contains(strings.Split(value, "/"), "..")
			continue
		}
		if strings.Contains(value, "..") || climbing {
			if namesPath(value, dirtyPath) {
				return true
			}
			climbs = climbs || slices.Contains(strings.Split(value, "/"), "..")
			continue
		}
		for _, candidate := range componentRuns(value) {
			if len(candidate.components) < 2 {
				named = named || candidate.components[0] == parts[len(parts)-1]
				continue
			}
			if candidate.matchesAnywhere(parts) {
				return true
			}
		}
	}
	if !named && len(prefixes) == 0 {
		return false
	}
	vocabulary := map[string]bool{}
	for _, value := range tokens {
		for _, component := range strings.Split(value, "/") {
			vocabulary[component] = true
		}
	}
	// An anchored token's lone run that matches a directory component,
	// joined to a root assembled at run time from the package's literals:
	// every component before the run must be one a literal could join, from
	// the root or from the package's possible module directories. A lone run
	// that is the file name was decided above.
	for _, offset := range prefixes {
		before := strings.Join(parts[:offset], "/")
		for _, base := range bases {
			rest, under := before, true
			if base != "" {
				rest, under = strings.CutPrefix(before+"/", base+"/")
				rest = strings.TrimSuffix(rest, "/")
			}
			if under && (rest == "" || placeable(vocabulary, rest+"/x")) {
				return true
			}
		}
	}
	if !named {
		return false
	}
	for _, base := range bases {
		if base != "" && !climbs {
			break
		}
		rest, under := dirtyPath, true
		if base != "" {
			rest, under = strings.CutPrefix(dirtyPath, base+"/")
		}
		if under && placeable(vocabulary, rest) {
			return true
		}
	}
	return false
}

// anchoredWithin is the planner's anchoredWithin: joined to one of bases the
// token is the path or an ancestor of it, its last of two or more components
// allowed to be a fragment unless the token ends in "/". A token that climbs
// out of the repository from a base names nothing from it; one that climbs
// back to the base names every path below it; one with no component run
// (`"/"`, `"/."`) names nothing.
func anchoredWithin(bases []string, value string, parts []string) bool {
	rest := strings.TrimLeft(value, "/")
	if len(componentRuns(rest)) == 0 {
		return false
	}
	dirtyPath := strings.Join(parts, "/")
	for _, base := range bases {
		resolved := path.Join(base, rest)
		if resolved == ".." || strings.HasPrefix(resolved, "../") {
			continue
		}
		if resolved == "." {
			if base == "" || strings.HasPrefix(dirtyPath, base+"/") {
				return true
			}
			continue
		}
		components := strings.Split(resolved, "/")
		if len(components) > len(parts) {
			continue
		}
		matched := true
		for position, component := range components {
			if position == len(components)-1 && len(components) >= 2 && !strings.HasSuffix(value, "/") {
				matched = strings.HasPrefix(parts[position], component)
			} else {
				matched = parts[position] == component
			}
			if !matched {
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// longRunNames reports whether a run of two or more components of an
// anchored token matches consecutive components of parts anywhere: such a
// run keeps the component-run relation, because the root it is joined to may
// come from the environment or another value no literal spells
// (`os.Getenv("FIXTURES") + "/repo/lib.js"`), so no placement bounds it.
func longRunNames(value string, parts []string) bool {
	for _, candidate := range componentRuns(value) {
		if len(candidate.components) >= 2 && candidate.matchesAnywhere(parts) {
			return true
		}
	}
	return false
}

// runOffsets lists the offsets at which a lone-component run of an anchored
// token matches a component of parts (at every offset: an anchored token's
// lone component is not limited to the file name); longRunNames decides a
// longer run. An offset at the file name keeps the reader without placement
// (round 7: `os.ReadFile(os.Getenv("FIXTURES") + "/lib.js")` reads
// app/x/repo/lib.js), as a run of two or more components does.
func runOffsets(value string, parts []string) []int {
	var offsets []int
	for _, candidate := range componentRuns(value) {
		if len(candidate.components) >= 2 {
			continue
		}
		for offset := 0; offset+len(candidate.components) <= len(parts); offset++ {
			if candidate.matchesAt(parts[offset:]) {
				offsets = append(offsets, offset)
			}
		}
	}
	return offsets
}

// placeable reports whether every directory component of relative is a whole
// component of one of the package's tokens or one the lexicon (pathToken)
// cannot keep whole.
func placeable(vocabulary map[string]bool, relative string) bool {
	components := strings.Split(relative, "/")
	for _, component := range components[:len(components)-1] {
		if !vocabulary[component] && lexable(component) {
			return false
		}
	}
	return true
}

// lexable reports whether component is made only of the characters pathToken
// keeps in one token, [A-Za-z0-9._~@+/-].
func lexable(component string) bool {
	for _, char := range component {
		switch {
		case 'a' <= char && char <= 'z', 'A' <= char && char <= 'Z', '0' <= char && char <= '9':
		case strings.ContainsRune("._~@+-", char):
		default:
			return false
		}
	}
	return true
}
