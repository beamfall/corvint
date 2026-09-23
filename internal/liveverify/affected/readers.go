package affected

import "strings"

// This file mirrors the naming relation of tools/gate-affected-select
// (componentRuns, namesPath, matchesAt; AFP-V0-012 rule (c)). The gate tool is
// a standard-library-only package main that the trusted PR driver builds
// (AFP-V0-011, AFP-V0-013), so it cannot import this package without widening
// that trusted build, and this package cannot import a main package.

// readers selects, for every dirty path no plugin owns, each unit not already
// reached whose path tokens name it, with a PATH_LITERAL_READER witness
// (AFP-V0-021). Like rule (c) the reader is not traversed to its dependents.
// The path keeps its UNOWNED_DIRTY_PATH entry: a literal index cannot bound
// the reads a path assembled at run time makes, so the selection only adds.
// Unknown entries arrive in dirty-path order and units in id order, so the
// smallest naming dirty path is each reader's witness.
func (graph *Graph) readers(reached map[string]Witness, unknown []Unknown) {
	for _, entry := range unknown {
		if entry.Reason != UnknownUnownedDirtyPath {
			continue
		}
		for _, id := range graph.namers(entry.Detail) {
			if _, seen := reached[id]; !seen {
				reached[id] = Witness{Kind: WitnessPathLiteralReader, DirtyPath: entry.Detail, Via: []string{id}}
			}
		}
	}
}

// namers lists, in id order, the units carrying a path token that names
// dirtyPath.
func (graph *Graph) namers(dirtyPath string) []string {
	parts := strings.Split(dirtyPath, "/")
	ids := make([]string, 0)
	for _, id := range graph.order {
		if namesAny(graph.units[id].PathTokens, parts) {
			ids = append(ids, id)
		}
	}
	return ids
}

func namesAny(tokens, parts []string) bool {
	for _, value := range tokens {
		if namesPath(value, parts) {
			return true
		}
	}
	return false
}

type run struct {
	components                []string
	partialFirst, partialLast bool
}

// componentRuns splits a token at empty, `.`, and `..` components. A run of two
// or more components may begin or end mid-name where the token does (a literal
// concatenated with a variable), so its outer components match by suffix and
// prefix; a lone component must match a whole path component.
func componentRuns(value string) []run {
	components := strings.Split(value, "/")
	var runs []run
	start := 0
	for end := 0; end <= len(components); end++ {
		if end < len(components) && components[end] != "" && components[end] != "." && components[end] != ".." {
			continue
		}
		if end > start {
			long := end-start >= 2
			runs = append(runs, run{components: components[start:end], partialFirst: long && start == 0, partialLast: long && end == len(components)})
		}
		start = end + 1
	}
	return runs
}

// namesPath reports whether one of the token's component runs matches
// consecutive components of the split dirty path.
func namesPath(value string, parts []string) bool {
	for _, candidate := range componentRuns(value) {
		for offset := 0; offset+len(candidate.components) <= len(parts); offset++ {
			if candidate.matchesAt(parts[offset:]) {
				return true
			}
		}
	}
	return false
}

func (candidate run) matchesAt(parts []string) bool {
	last := len(candidate.components) - 1
	for position, component := range candidate.components {
		switch {
		case position == 0 && candidate.partialFirst && position != last:
			if !strings.HasSuffix(parts[position], component) {
				return false
			}
		case position == last && candidate.partialLast:
			if !strings.HasPrefix(parts[position], component) {
				return false
			}
		default:
			if parts[position] != component {
				return false
			}
		}
	}
	return true
}
