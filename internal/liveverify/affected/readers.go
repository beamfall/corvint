package affected

import (
	"path"
	"strings"
)

// This file mirrors the naming relation of tools/gate-affected-select
// (componentRuns, namesPath, matchesAt; AFP-V0-012 rule (c)). The gate tool is
// a standard-library-only package main that the trusted PR driver builds
// (AFP-V0-011, AFP-V0-013), so it cannot import this package without widening
// that trusted build, and this package cannot import a main package. One rule
// is narrower here than rule (c): an unanchored lone component names a file
// name only (namesPath, V1-0290). The CEM sidecar narrowing of rule (c)
// (resolvingReaders, resolvesWithin, rootAnchors) is mirrored by resolves.

// ChangeEvidencePath is the CEM 0.2 sidecar (frontier.ExcludedPath). Its
// readers are narrowed to the units whose token resolves to it, as the gate's
// rule (c) does, because a dogfood change commits it on every change.
const ChangeEvidencePath = ".corvint/change.cem.json"

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
			if _, seen := reached[id]; !seen && graph.resolves(id, dirtyPath) {
				reached[id] = Witness{Kind: WitnessPathLiteralReader, DirtyPath: dirtyPath, Via: []string{id}}
			}
		}
	}
}

// unboundedReadersOf selects, on any dirty path, each unit not already reached
// whose reads no literal bounds, witnessed by the smallest dirty path (rule
// (d)). Like a reader it is not traversed: its unbounded dependents are
// already in the set.
func (graph *Graph) unboundedReadersOf(reached map[string]Witness, dirty []string) {
	if len(dirty) == 0 {
		return
	}
	for _, id := range graph.unbounded {
		if _, seen := reached[id]; !seen {
			reached[id] = Witness{Kind: WitnessUnboundedReader, DirtyPath: dirty[0], Via: []string{id}}
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

// resolves reports whether unit id can read dirtyPath by one of its tokens.
// Any path but the CEM sidecar keeps the component-run relation. For the
// sidecar, a Go unit's token must resolve against the unit's directory, or
// against the repository root when a parent-only or anchored token of the unit
// could put it there, to the sidecar or one of its ancestors. An anchored token
// always resolves: it is anchored at its module's directory, which this graph
// does not record, so narrowing it could drop a real reader.
func (graph *Graph) resolves(id, dirtyPath string) bool {
	unit := graph.units[id]
	if dirtyPath != ChangeEvidencePath || !strings.HasPrefix(id, "go:") {
		return true
	}
	directory := relativeDirectory(append(append([]string(nil), unit.Sources...), unit.Tests...)[0])
	anchored := rootAnchored(unit.PathTokens)
	parts := strings.Split(dirtyPath, "/")
	for _, value := range unit.PathTokens {
		if !namesPath(value, parts) {
			continue
		}
		if strings.HasPrefix(value, "/") || resolvesWithin(directory, value, dirtyPath) || (anchored && resolvesWithin("", value, dirtyPath)) {
			return true
		}
	}
	return false
}

// rootAnchored reports a token that could put an adjacent fragment at the
// repository root: an anchored token, or one made only of parent components.
func rootAnchored(tokens []string) bool {
	for _, value := range tokens {
		if strings.HasPrefix(value, "/") || parentOnly(value) {
			return true
		}
	}
	return false
}

func parentOnly(value string) bool {
	parent := false
	for _, component := range strings.Split(value, "/") {
		switch component {
		case "", ".":
		case "..":
			parent = true
		default:
			return false
		}
	}
	return parent
}

// resolvesWithin reports whether value, joined to directory, names dirtyPath
// or one of its ancestors, with a multi-component token's outer components
// allowed to be fragments at their resolved positions.
func resolvesWithin(directory, value, dirtyPath string) bool {
	resolved := path.Join(directory, value)
	if resolved == "." {
		resolved = ""
	}
	if resolved == "" || resolved == dirtyPath || strings.HasPrefix(dirtyPath, resolved+"/") {
		return true
	}
	resolvedParts := strings.Split(resolved, "/")
	dirtyParts := strings.Split(dirtyPath, "/")
	if len(resolvedParts) < 2 || len(resolvedParts) != len(dirtyParts) {
		return false
	}
	for position, component := range resolvedParts {
		switch {
		case position == 0:
			if !strings.HasSuffix(dirtyParts[position], component) {
				return false
			}
		case position == len(resolvedParts)-1:
			if !strings.HasPrefix(dirtyParts[position], component) {
				return false
			}
		case dirtyParts[position] != component:
			return false
		}
	}
	return true
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
// that is neither anchored at a module root nor climbs with `..` is tried
// against the file name alone, never a directory component: the `internal/`
// of `"internal/%03d.go"` would otherwise name every path below any
// `internal` directory (V1-0290), while `"../../.corvint"` is a path.
func namesPath(value string, parts []string) bool {
	for _, candidate := range componentRuns(value) {
		first := 0
		if len(candidate.components) == 1 && !strings.HasPrefix(value, "/") && !strings.Contains(value, "..") {
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
