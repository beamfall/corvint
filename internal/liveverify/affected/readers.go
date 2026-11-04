package affected

import (
	"path"
	"slices"
	"strings"
)

// This file mirrors the naming relation of tools/gate-affected-select
// (componentRuns, namesPath, matchesAt; AFP-V0-012 rule (c)). The gate tool is
// a standard-library-only package main that the trusted PR driver builds
// (AFP-V0-011, AFP-V0-013), so it cannot import this package without widening
// that trusted build, and this package cannot import a main package. One rule
// is narrower here than rule (c): an unanchored lone component names a file
// name only (namesPath, V1-0290). The CEM sidecar narrowing of rule (c)
// (resolvingReaders, resolvesWithin, rootAnchors) is mirrored by resolves. A
// second narrowing exists only here: a Go unit that names a dirty path by
// such a file name alone must also place the path (namesFileOnlyWithin,
// AFP-V0-072).

// ChangeEvidencePath is the CEM 0.2 sidecar (frontier.ExcludedPath). Its
// readers are narrowed to the units whose token resolves to it, as the gate's
// rule (c) does, because a dogfood change commits it on every change.
const ChangeEvidencePath = ".corvint/change.cem.json"

// SealedChangesDirectory holds the sealed CEMs a dogfood seal moves the
// sidecar to, one `<commit>.cem.json` per change (AFP-V0-152).
const SealedChangesDirectory = ".corvint/changes/"

// resolvedEvidence reports whether dirtyPath is the CEM sidecar or a sealed
// CEM directly under SealedChangesDirectory: a path whose Go readers must
// resolve to it (resolves), because every dogfooded change commits one and a
// fixture-root token naming the same components reads a temporary copy
// (AFP-V0-152, V1-1820).
func resolvedEvidence(dirtyPath string) bool {
	if dirtyPath == ChangeEvidencePath {
		return true
	}
	name, sealed := strings.CutPrefix(dirtyPath, SealedChangesDirectory)
	return sealed && strings.HasSuffix(name, ".cem.json") && len(name) > len(".cem.json") && !strings.Contains(name, "/")
}

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
// in id order, so the smallest naming dirty path that resolves is each
// reader's witness; each unit tries the dirty paths in that order and stops
// at its witness.
// It returns the namers resolves kept out of a path other than the CEM
// sidecar, each true when one such path is unowned: Select still names their
// unit frontier whose bearing condition holds, so the AFP-V0-072 narrowing
// removes no unknown entry (AFP-V0-072(4), AFP-V0-581). It also returns each
// already-reached unit with a frontierBears: DEPENDENTS frontier whose own
// path literal or glob names a dirty path, resolved or narrowed: its frontier
// bears whichever witness won (AFP-V0-581 (2)).
func (graph *Graph) readers(reached map[string]Witness, dirty []string) (narrowed, reads map[string]bool) {
	split := make([][]string, len(dirty))
	for i, dirtyPath := range dirty {
		split[i] = strings.Split(dirtyPath, "/")
	}
	narrowed = make(map[string]bool)
	reads = make(map[string]bool)
	tokens := graph.pathTokens()
	for i, id := range graph.order {
		_, seen := reached[id]
		if seen && graph.units[id].FrontierBears != FrontierBearsOnDependents {
			continue
		}
		for j, dirtyPath := range dirty {
			if !namesAny(tokens[i], split[j]) {
				continue
			}
			if graph.foreignTestFile(id, dirtyPath) && !namesFileName(tokens[i], split[j]) {
				continue
			}
			if graph.resolves(id, dirtyPath) {
				if seen {
					reads[id] = true
				} else {
					reached[id] = Witness{Kind: WitnessPathLiteralReader, DirtyPath: dirtyPath, Via: []string{id}}
				}
				break
			}
			if dirtyPath == ChangeEvidencePath || len(graph.units[id].Frontier) == 0 {
				continue
			}
			if seen {
				reads[id] = true
				break
			}
			_, owned := graph.owner[dirtyPath]
			narrowed[id] = narrowed[id] || !owned
		}
		if _, now := reached[id]; !now || (seen && !reads[id]) {
			if dirtyPath, glob, ok := graph.globRead(id, dirty); ok {
				if seen {
					reads[id] = true
				} else {
					reached[id] = Witness{Kind: WitnessPathGlobReader, DirtyPath: dirtyPath, Via: []string{id}, Glob: glob}
				}
			}
		}
	}
	return narrowed, reads
}

// foreignTestFile reports whether dirtyPath is a `_test.go` file that a Go
// unit other than id owns while id is a Go unit: Go compiles that file only
// into its owner's test binary, so id reads it only by naming the file
// (namesFileName), never by a literal of an ancestor directory (ACX-V0-852).
func (graph *Graph) foreignTestFile(id, dirtyPath string) bool {
	if !strings.HasPrefix(id, "go:") || !strings.HasSuffix(dirtyPath, "_test.go") {
		return false
	}
	owner, owned := graph.owner[dirtyPath]
	return owned && owner != id && strings.HasPrefix(owner, "go:")
}

// namesFileName reports whether a component run of one of the tokens matches
// the split path ending at its file name, so the token names the file itself
// (whole, or as a run's trailing fragment) rather than only an ancestor
// directory of it (ACX-V0-852).
func namesFileName(tokens []pathToken, parts []string) bool {
	for _, token := range tokens {
		for _, candidate := range token.runs {
			if offset := len(parts) - len(candidate.components); offset >= 0 && candidate.matchesAt(parts[offset:]) {
				return true
			}
		}
	}
	return false
}

// globRead reports whether one of unit id's literal glob patterns matches a
// dirty path, or an ancestor of it, that its declared read scope (if any)
// allows, with the smallest such path and the first pattern: readers selects
// the unit by it (AFP-V0-401). An anchored pattern matches from the repository
// root. An unanchored one (always two or more components) follows a root known
// only at run time, so it matches any run of the path's components, as a
// multi-component path token does (AFP-V0-021).
func (graph *Graph) globRead(id string, dirty []string) (dirtyPath, glob string, ok bool) {
	unit := graph.units[id]
	if len(unit.PathGlobs) == 0 {
		return "", "", false
	}
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	directory := ""
	if len(files) != 0 {
		directory = relativeDirectory(files[0])
	}
	for _, dirtyPath := range dirty {
		if unit.ReadScoped && !InReadScope(directory, unit.ReadScope, dirtyPath) {
			continue
		}
		parts := strings.Split(dirtyPath, "/")
		for _, glob := range unit.PathGlobs {
			if globMatches(glob, parts) {
				return dirtyPath, glob, true
			}
		}
	}
	return "", "", false
}

// globMatches reports whether glob matches the path parts or one of their
// ancestors, component by component (path.Match never crosses a "/").
func globMatches(glob string, parts []string) bool {
	anchored := strings.HasPrefix(glob, "/")
	components := strings.Split(strings.TrimPrefix(glob, "/"), "/")
	for offset := 0; offset+len(components) <= len(parts); offset++ {
		matched := true
		for index, component := range components {
			if ok, err := path.Match(component, parts[offset+index]); err != nil || !ok {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
		if anchored {
			return false
		}
	}
	return false
}

// testlessReaderUsers carries each path-literal or path-glob reader that has
// no test of its own, and that names a dirty path no unit owns, to the units
// that can exercise it (AFP-V0-495): its reverse dependents, and their
// test-only users, witnessed by the chain from the reader through the
// smallest such unowned path. A reader with its own tests stays untraversed
// (AFP-V0-021), as does one that names only owned paths, whose change already
// reaches its importers through their edges (a runner config's test-match
// literal, say). A test-less reader of an unowned path would otherwise drop it
// with no selection and no unknown, so this only adds; witnesses already in
// reached are kept. It returns every unit it traversed, which therefore is no
// terminal selection whatever witness it keeps (AFP-V0-560), and every unit the
// traversal expands joins followed (AFP-V0-581).
func (graph *Graph) testlessReaderUsers(reached map[string]Witness, dirty []string, followed map[string]bool) map[string]bool {
	testless := func(id string) bool {
		witness := reached[id]
		return (witness.Kind == WitnessPathLiteralReader || witness.Kind == WitnessPathGlobReader) && witness.ViaLength() == 1 && witness.viaHead() == id && len(graph.units[id].Tests) == 0
	}
	found := false
	for id := range reached {
		if found = testless(id); found {
			break
		}
	}
	unowned := make([]string, 0, len(dirty))
	for _, path := range dirty {
		if _, owned := graph.owner[path]; !owned {
			unowned = append(unowned, path)
		}
	}
	if !found || len(unowned) == 0 {
		return nil
	}
	named := make(map[string]Witness)
	graph.readers(named, unowned)
	start := make(map[string]Witness)
	for id, witness := range named {
		if testless(id) {
			start[id] = witness
		}
	}
	if len(start) == 0 {
		return nil
	}
	users := graph.traverse(start, nil)
	for id := range users {
		followed[id] = true
	}
	graph.testUsersOf(users, nil)
	carried := make(map[string]bool, len(users))
	for id := range users {
		carried[id] = true
	}
	mergeWitnesses(reached, users)
	return carried
}

// unboundedReadersOf selects, on any dirty path, each unit not already reached
// whose reads no literal bounds, witnessed by the smallest dirty path (rule
// (d)), with the cause that makes it one (AFP-V0-192). Like a reader it is not
// traversed: its unbounded dependents are already in the set.
func (graph *Graph) unboundedReadersOf(reached map[string]Witness, dirty []string) {
	if len(dirty) == 0 {
		return
	}
	for _, id := range graph.unbounded {
		if _, seen := reached[id]; !seen {
			cause := graph.causes[id]
			reached[id] = Witness{Kind: WitnessUnboundedReader, DirtyPath: dirty[0], Via: []string{id}, Cause: &UnboundedCause{Unit: cause, Reads: graph.units[cause].UnboundedReads}}
		}
	}
}

// ReadScopesPath is the repository's declaration of the paths each declared
// Go package's tests read outside the package directory (AFP-V0-023).
const ReadScopesPath = ".corvint/test-read-scopes.json"

// GatedRootLocatorsPath is the repository's declaration of the Go packages
// whose root locating runs only behind an absolute gate outside the
// repository (TSP-V0-003). The declaration is read when the graph is built,
// so a plan with the declaration dirty is UNKNOWN (gatedLocatorsChanged).
const GatedRootLocatorsPath = ".corvint/gated-root-locators.json"

// FrontierGatedLocatorsChanged is the unknown detail of a plan whose dirty
// paths hold GatedRootLocatorsPath (TSP-V0-003): the edited declaration may
// clear a root locator its own head graph trusts.
const FrontierGatedLocatorsChanged = "go:gated-root-locators-changed"

// ShellForeignCallersPath is the repository's declaration of the files that
// run a shell script by a path no plugin resolves (TSP-V0-005). The shell
// plugin reads and verifies it when the graph is built, so a plan with the
// declaration dirty is UNKNOWN (foreignCallersChanged).
const ShellForeignCallersPath = ".corvint/shell-foreign-callers.json"

// FrontierForeignCallersChanged is the unknown detail of a plan whose dirty
// paths hold ShellForeignCallersPath (TSP-V0-005).
const FrontierForeignCallersChanged = "shell:foreign-callers-changed"

// ShellReadScopesPath is the repository's declaration of the paths each
// declared toolchain-running shell test reads (TSP-V0-950). The shell plugin
// reads it when the graph is built, so a plan with the declaration dirty is
// UNKNOWN (shellReadScopesChanged).
const ShellReadScopesPath = ".corvint/shell-read-scopes.json"

// FrontierShellReadScopesChanged is the unknown detail of a plan whose dirty
// paths hold ShellReadScopesPath (TSP-V0-950).
const FrontierShellReadScopesChanged = "shell:read-scopes-changed"

// shellReadScopesChanged returns the TSP-V0-950 unknown when normalized holds
// the shell read-scope declaration.
func shellReadScopesChanged(normalized []string) []Unknown {
	if !slices.Contains(normalized, ShellReadScopesPath) {
		return nil
	}
	return []Unknown{{Reason: UnknownLanguageFrontier, Detail: FrontierShellReadScopesChanged}}
}

// foreignCallersChanged returns the TSP-V0-005 unknown when normalized holds
// the shell foreign-caller declaration.
func foreignCallersChanged(normalized []string) []Unknown {
	if !slices.Contains(normalized, ShellForeignCallersPath) {
		return nil
	}
	return []Unknown{{Reason: UnknownLanguageFrontier, Detail: FrontierForeignCallersChanged}}
}

// gatedLocatorsChanged returns the TSP-V0-003 unknown when normalized holds
// the gated root-locator declaration, whatever unit names it.
func gatedLocatorsChanged(normalized []string) []Unknown {
	if !slices.Contains(normalized, GatedRootLocatorsPath) {
		return nil
	}
	return []Unknown{{Reason: UnknownLanguageFrontier, Detail: FrontierGatedLocatorsChanged}}
}

// scopedReadersOf selects each unit not already reached whose declared read
// scope holds a dirty path, witnessed by the smallest such path (AFP-V0-023).
// Like a reader it is not traversed. When the declaration itself is dirty and
// the graph knows the declaration it is measured from, only a Go unit whose
// own declaration was added, changed or removed bears on that edit
// (AFP-V0-050); otherwise every declared unit does.
func (graph *Graph) scopedReadersOf(reached map[string]Witness, dirty []string) {
	for _, id := range graph.order {
		unit := graph.units[id]
		if _, seen := reached[id]; seen || (!unit.ReadScoped && graph.priorReadScopes == nil) {
			continue
		}
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		if len(files) == 0 {
			continue
		}
		directory := relativeDirectory(files[0])
		for _, dirtyPath := range dirty {
			bears := unit.ReadScoped && InReadScope(directory, unit.ReadScope, dirtyPath)
			if dirtyPath == ReadScopesPath && graph.priorReadScopes != nil {
				bears = graph.declarationChanged(id, unit, directory) || (unit.ReadScoped && inDeclaredScope(directory, unit.ReadScope, dirtyPath))
			}
			if bears {
				reached[id] = Witness{Kind: WitnessDeclaredReadScope, DirtyPath: dirtyPath, Via: []string{id}}
				break
			}
		}
	}
}

// declarationChanged reports whether a Go unit's read-scope declaration in
// the observed tree differs from the one the change is measured from: added,
// with different entries, or removed (AFP-V0-050).
func (graph *Graph) declarationChanged(id string, unit Unit, directory string) bool {
	if !strings.HasPrefix(id, "go:") {
		return false
	}
	prior, declared := graph.priorReadScopes[directory]
	if !unit.ReadScoped {
		return declared
	}
	return !declared || !slices.Equal(prior, unit.ReadScope)
}

// InReadScope reports whether a dirty path bears on a package in directory
// whose tests read only scope outside it: the declaration itself, a path in
// the package's subtree, or a path that is a declared entry, lies in a
// declared subtree, or is an ancestor of an entry (a directory record).
func InReadScope(directory string, scope []string, dirtyPath string) bool {
	return dirtyPath == ReadScopesPath || inDeclaredScope(directory, scope, dirtyPath)
}

// inDeclaredScope is InReadScope without the declaration itself: the package's
// subtree and its declared entries only.
func inDeclaredScope(directory string, scope []string, dirtyPath string) bool {
	if directory == "." || dirtyPath == directory || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	for _, entry := range scope {
		target := strings.TrimSuffix(entry, "/")
		if dirtyPath == target || strings.HasPrefix(target, dirtyPath+"/") || (target != entry && strings.HasPrefix(dirtyPath, entry)) {
			return true
		}
	}
	return false
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
// A non-Go unit its plugin certifies (Unit.PlacesFileNames, a shell script
// that changes to no literal directory and reads nothing recursively) places
// a lone file name as a Go unit does (namesFileOnlyWithin, AFP-V0-400): it runs
// from the repository root or below its own directory. Any other non-Go unit
// keeps the component-run relation: its tests' working directory is unknown. For the CEM sidecar and a sealed CEM
// (resolvedEvidence), a Go unit's token must resolve against the unit's
// directory, or against the repository root when a parent-only or anchored
// token of the unit could put it there, to that path or one of its ancestors. An anchored token always resolves: it is anchored at
// its module's directory, which this graph does not record, so narrowing it
// could drop a real reader. For any other path see namesFileOnlyWithin.
// A unit with a declared read scope (AFP-V0-023) reads no dirty path outside
// that scope, whatever its tokens name (AFP-V0-161): the declaration states
// it, and full CI confines the package's tests to it.
func (graph *Graph) resolves(id, dirtyPath string) bool {
	unit := graph.units[id]
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	if unit.ReadScoped && len(files) != 0 && !InReadScope(relativeDirectory(files[0]), unit.ReadScope, dirtyPath) {
		return false
	}
	if unit.PlacesFileNames && !strings.HasPrefix(id, "go:") && len(files) != 0 {
		return namesFileOnlyWithin(relativeDirectory(files[0]), unit.PathTokens, dirtyPath, false)
	}
	if unit.AnchorsPlaced && !strings.HasPrefix(id, "go:") && len(files) != 0 {
		return anchorsPlacedName(relativeDirectory(files[0]), unit, dirtyPath)
	}
	if !strings.HasPrefix(id, "go:") || len(files) == 0 {
		// A unit with no files, such as a dependent nested module
		// (AFP-V0-582), has no directory to narrow a token against.
		return true
	}
	directory := relativeDirectory(files[0])
	if !resolvedEvidence(dirtyPath) {
		return namesFileOnlyWithin(directory, unit.PathTokens, dirtyPath, true)
	}
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

// namesFileOnlyWithin reports whether a Go unit in directory whose tokens name
// dirtyPath can reach it (AFP-V0-072). A token that names it by a run that
// places a directory (a multi-component run, or any run of an anchored or
// climbing token) keeps the component-run relation. When only unanchored lone
// tokens name it by its file name (V1-0290), the unit reaches it only in its
// own subtree, where its tests run, or under a base its literals can reach:
// the repository root, or any ancestor of its directory once a token climbs
// (`filepath.Join("..", "model.go")`). Below that base every directory
// component of the path must be a whole component of one of its tokens, so a
// literal could join it, or a component the lexer cannot keep whole (a space,
// a non-ASCII letter), which no token can witness and so never narrows.
// Any other way to the repository goes through a root-locating call or a
// climbing literal, which select the unit as an unbounded reader (rule (d)),
// or through a root supplied at run time, which no literal index bounds.
//
// For a Go unit (module) an anchored token is decided by anchoredWithin
// instead of the component-run relation (AFP-V0-2203); a non-Go unit's
// anchored token follows a run-time root and keeps the relation.
func namesFileOnlyWithin(directory string, tokens []string, dirtyPath string, module bool) bool {
	if directory == "." || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	parts := strings.Split(dirtyPath, "/")
	climbs, named := false, false
	// prefixes holds, for each anchored token that does not resolve from a
	// base, the number of leading path components before a run of it
	// matches: the run then needs those components placed (AFP-V0-2203).
	var prefixes []int
	for _, value := range tokens {
		if strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
			if anchored, ok := anchoredWithin(directory, value, dirtyPath); module && ok {
				if anchored {
					return true
				}
				prefixes = append(prefixes, runOffsets(value, parts)...)
			} else if namesPath(value, parts) {
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
			for offset := 0; offset+len(candidate.components) <= len(parts); offset++ {
				if candidate.matchesAt(parts[offset:]) {
					return true
				}
			}
		}
	}
	if !named && len(prefixes) == 0 {
		return false
	}
	vocabulary := make(map[string]bool)
	for _, value := range tokens {
		for _, component := range strings.Split(value, "/") {
			vocabulary[component] = true
		}
	}
	// An anchored token joined to a root assembled at run time from the
	// unit's literals: every component before the run must be one a literal
	// could join, from the root or from the module's possible directories.
	anchorBases := []string{""}
	for base := directory; base != "."; base = path.Dir(base) {
		anchorBases = append(anchorBases, base)
	}
	for _, offset := range prefixes {
		before := strings.Join(parts[:offset], "/")
		for _, base := range anchorBases {
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
	bases := []string{""}
	for base := directory; climbs && base != "."; base = path.Dir(base) {
		bases = append(bases, base)
	}
	for _, base := range bases {
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

// anchorsPlacedName is resolves for a non-Go unit its plugin certifies
// (Unit.AnchorsPlaced, AFP-V0-2245): a token that does not start with "/"
// keeps the component-run relation (namesPath), and an anchored token names
// dirtyPath only as a Go unit's does under AFP-V0-2203, but from the bases a
// runner may join it to: the repository root, the unit's directory and its
// ancestors, and each recorded run directory (a relative path,
// `process.cwd()` or `path.resolve()` starts in the runner's directory; the
// plugin lists a run directory's ancestors too when the module can climb). A run of the token may also follow directory
// components below a base that are each a component of one of the unit's
// tokens or words. The unit's own subtree keeps every token.
func anchorsPlacedName(directory string, unit Unit, dirtyPath string) bool {
	if directory == "." || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	parts := strings.Split(dirtyPath, "/")
	seen := map[string]bool{"": true}
	bases := []string{""}
	for base := directory; base != "." && base != "" && !seen[base]; base = path.Dir(base) {
		seen[base] = true
		bases = append(bases, base)
	}
	for _, base := range unit.RunDirectories {
		if !seen[base] {
			seen[base] = true
			bases = append(bases, base)
		}
	}
	var vocabulary map[string]bool
	for _, value := range unit.PathTokens {
		if !strings.HasPrefix(value, "/") {
			if namesPath(value, parts) {
				return true
			}
			continue
		}
		if anchoredFrom(bases, value, dirtyPath) {
			return true
		}
		for _, offset := range runOffsets(value, parts) {
			if vocabulary == nil {
				vocabulary = make(map[string]bool)
				for _, token := range unit.PathTokens {
					for _, component := range strings.Split(token, "/") {
						vocabulary[component] = true
					}
				}
				for _, word := range unit.AnchorWords {
					vocabulary[word] = true
				}
			}
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
	}
	return false
}

// anchoredWithin decides an anchored token (one that starts with "/": a
// module-path literal the Go plugin anchored at its module's directory, or an
// absolute path) for a Go unit in directory (AFP-V0-2203). The module's
// directory is not recorded, but it is the unit's directory or one of its
// ancestors, so the token names dirtyPath only when, joined to one of those
// bases, it is dirtyPath or an ancestor of it; the last of two or more
// components may be a fragment the literal is concatenated with, as in
// componentRuns. A token that climbs out of the repository from every base
// names nothing in it, and so does one with no component run (`"/"`, `"/."`),
// as componentRuns finds none. The Go lexicon splits a literal at a printf
// verb, so an empty component inside one token is a literal double slash,
// which the join cleans. ok is false for a token that does not start with
// "/".
func anchoredWithin(directory, value, dirtyPath string) (names, ok bool) {
	if !strings.HasPrefix(value, "/") {
		return false, false
	}
	bases := []string{""}
	for base := directory; base != "." && base != ""; base = path.Dir(base) {
		bases = append(bases, base)
	}
	return anchoredFrom(bases, value, dirtyPath), true
}

// anchoredFrom is anchoredWithin's relation over an explicit list of bases.
func anchoredFrom(bases []string, value, dirtyPath string) bool {
	rest := strings.TrimLeft(value, "/")
	if len(componentRuns(rest)) == 0 {
		return false
	}
	parts := strings.Split(dirtyPath, "/")
	for _, base := range bases {
		resolved := path.Join(base, rest)
		if resolved == ".." || strings.HasPrefix(resolved, "../") {
			continue
		}
		if resolved == "." {
			// The token climbs back to its module's directory
			// (`"/x/.."`): it names every path below that base.
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
			last := position == len(components)-1
			if last && len(components) >= 2 && !strings.HasSuffix(value, "/") {
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

// runOffsets lists the offsets at which a run of the token matches
// consecutive components of parts, as namesPath tries them.
func runOffsets(value string, parts []string) []int {
	token := newPathToken(value)
	var offsets []int
	for _, candidate := range token.runs {
		first := 0
		if len(candidate.components) == 1 && token.nameOnly {
			first = len(parts) - 1
		}
		for offset := first; offset+len(candidate.components) <= len(parts); offset++ {
			if candidate.matchesAt(parts[offset:]) {
				offsets = append(offsets, offset)
			}
		}
	}
	return offsets
}

// placeable reports whether every directory component of relative is a whole
// token component or one the Go lexicon cannot keep whole.
func placeable(vocabulary map[string]bool, relative string) bool {
	components := strings.Split(relative, "/")
	for _, component := range components[:len(components)-1] {
		if !vocabulary[component] && lexable(component) {
			return false
		}
	}
	return true
}

// lexable reports whether component is made only of the characters the Go
// plugin's path-token lexicon keeps in one token (golang.pathToken,
// [A-Za-z0-9._~@+/-]); any other character splits the literal there.
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

// pathTokens holds, in graph.order, each unit's path tokens with their
// component runs split once, a token shared by several units split once.
func (graph *Graph) pathTokens() [][]pathToken {
	split := map[string]pathToken{}
	tokens := make([][]pathToken, len(graph.order))
	for i, id := range graph.order {
		values := graph.units[id].PathTokens
		tokens[i] = make([]pathToken, len(values))
		for j, value := range values {
			token, ok := split[value]
			if !ok {
				token = newPathToken(value)
				split[value] = token
			}
			tokens[i][j] = token
		}
	}
	return tokens
}

func namesAny(tokens []pathToken, parts []string) bool {
	for _, token := range tokens {
		if token.names(parts) {
			return true
		}
	}
	return false
}

// pathToken is a token's componentRuns with namesPath's file-name-only rule
// for a lone component decided once.
type pathToken struct {
	runs     []run
	nameOnly bool
}

func newPathToken(value string) pathToken {
	return pathToken{runs: componentRuns(value), nameOnly: !strings.HasPrefix(value, "/") && !strings.Contains(value, "..")}
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
	return newPathToken(value).names(parts)
}

func (token pathToken) names(parts []string) bool {
	for _, candidate := range token.runs {
		first := 0
		if len(candidate.components) == 1 && token.nameOnly {
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
