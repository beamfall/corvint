package affected

import "strings"

// This file mirrors the naming relation of tools/gate-affected-select
// (componentRuns, namesPath, matchesAt; AFP-V0-012 rule (c)). The gate tool is
// a standard-library-only package main that the trusted PR driver builds
// (AFP-V0-011, AFP-V0-013), so it cannot import this package without widening
// that trusted build, and this package cannot import a main package. One rule
// is narrower here than rule (c): an unanchored lone component names a file
// name only (namesPath, V1-0290).

// PathTokenBound names, in a LANGUAGE_FRONTIER detail
// "<namespace>:path-token-bound:<unit>", a unit whose plugin dropped its path
// tokens at the bound (AFP-V0-021).
const PathTokenBound = "path-token-bound"

// readers selects, for every dirty path, as rule (c) does whether or not a
// plugin owns the path, each unit not already reached whose path tokens name
// it, with a PATH_LITERAL_READER witness (AFP-V0-021). Like rule (c) the reader
// is not traversed to its dependents, and no unknown entry is added or removed
// for a match: a literal index cannot bound the reads a path assembled at run
// time makes, so the selection only adds. Dirty paths arrive sorted and units
// in id order, so the smallest naming dirty path is each reader's witness.
func (graph *Graph) readers(reached map[string]Witness, dirty []string) {
	for _, dirtyPath := range dirty {
		for _, id := range graph.namers(dirtyPath) {
			if _, seen := reached[id]; !seen {
				reached[id] = Witness{Kind: WitnessPathLiteralReader, DirtyPath: dirtyPath, Via: []string{id}}
			}
		}
	}
}

// tokenBounds names, once a reader match was attempted, every unit whose
// tokens were dropped at the plugin's bound and that nothing else reached: it
// may read a dirty path the plan cannot see.
func (graph *Graph) tokenBounds(reached map[string]Witness, dirty []string) []Unknown {
	unknown := make([]Unknown, 0)
	for _, id := range graph.order {
		if len(dirty) == 0 || !graph.units[id].PathTokensBounded {
			continue
		}
		if _, seen := reached[id]; !seen {
			namespace := strings.SplitN(id, ":", 2)[0]
			unknown = append(unknown, Unknown{Reason: UnknownLanguageFrontier, Detail: namespace + ":" + PathTokenBound + ":" + id})
		}
	}
	return unknown
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
// consecutive components of the split dirty path. A lone component of a token
// that is not anchored at a module root is tried against the file name alone,
// never a directory component: the `internal/` of `"internal/%03d.go"` would
// otherwise name every path below any `internal` directory (V1-0290).
func namesPath(value string, parts []string) bool {
	for _, candidate := range componentRuns(value) {
		first := 0
		if len(candidate.components) == 1 && !strings.HasPrefix(value, "/") {
			first = len(parts) - 1
		}
		for offset := first; offset+len(candidate.components) <= len(parts); offset++ {
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
