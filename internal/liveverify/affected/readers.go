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
// that trusted build, and this package cannot import a main package. For a Go
// unit and a path other than a CEM, the relation is rule (c)'s: any run of a
// token, a lone component included, names the path at any depth (AFP-V0-2203),
// because a literal may be joined to a root no literal index bounds (the
// environment, a flag, a walked directory, any non-constant prefix). For a
// non-Go unit one rule is narrower here than rule (c): an unanchored lone
// component names a file name only (namesPath, V1-0290), and a unit whose
// plugin certifies its roots (Unit.PlacesFileNames) must also place the path
// (namesFileOnlyWithin, AFP-V0-072, AFP-V0-400). The CEM sidecar narrowing of
// rule (c) (resolvingReaders, resolvesWithin, rootAnchors) is mirrored by
// resolves.

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
	content := graph.contentPathTokens()
	for i, id := range graph.order {
		_, seen := reached[id]
		if seen && graph.units[id].FrontierBears != FrontierBearsOnDependents {
			continue
		}
		goUnit := strings.HasPrefix(id, "go:")
		for j, dirtyPath := range dirty {
			if !graph.RecordEntryAllows(id, dirtyPath) {
				continue
			}
			// A name-listing token selects only for a dirty path whose
			// change status can change a listing (AFP-V0-9962).
			unitTokens := tokens[i]
			if content[i] != nil && !graph.listingMayChange(dirtyPath) {
				unitTokens = content[i]
			}
			if !namesAny(unitTokens, split[j], goUnit && !resolvedEvidence(dirtyPath)) {
				continue
			}
			if graph.foreignTestFile(id, dirtyPath) && !namesFileName(unitTokens, split[j]) {
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
		if !graph.RecordEntryAllows(id, dirtyPath) {
			continue
		}
		if unit.ReadScoped && !graph.inReadScope(directory, unit, dirtyPath) {
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
// no test of its own, and that names an unowned dirty path or one whose owner
// is untested, or a comment-only path omitted from compilation seeds
// (AFP-V0-3322), to the units that can exercise it (AFP-V0-495, AZU-V0-006): its
// reverse dependents, the units whose code runs a reached command's binary and
// theirs (AFP-V0-2992), and their test-only users and test-only runners, less a
// unit whose declared read scope excludes the path, witnessed by the chain from
// the reader through the smallest such path. Hidden Go paths also retain their
// callers (AZU-V0-005). A testless unit whose declared read scope holds such a
// path, other than through the declaration itself, is carried the same way
// (AZU-V0-006). A reader with its own tests stays untraversed (AFP-V0-021), as
// does one that names only other owned paths, whose change already reaches its
// importers through their edges (a runner config's test-match literal, say). An
// untested owner's importers need not include a reader's callers: walk those
// callers before claiming a coverage gap. A test-less reader would otherwise drop it
// with no selection and no unknown, so this only adds; witnesses already in
// reached are kept. It returns every unit it traversed, which therefore is no
// terminal selection whatever witness it keeps (AFP-V0-560), and every unit the
// traversal expands joins followed (AFP-V0-581).
func (graph *Graph) testlessReaderUsers(reached map[string]Witness, dirty []string, followed map[string]bool) map[string]bool {
	testless := func(id string) bool {
		witness := reached[id]
		// A comment-only owner's direct witness has not traversed its
		// callers. Let the independent readers scan below establish any
		// self-read before deciding those callers can be omitted.
		reader := witness.Kind == WitnessPathLiteralReader || witness.Kind == WitnessPathGlobReader || witness.Kind == WitnessDirectSource && graph.goCommentOnly[witness.DirtyPath]
		return reader && witness.ViaLength() == 1 && witness.viaHead() == id && len(graph.units[id].Tests) == 0
	}
	found := false
	for id := range reached {
		if found = testless(id); found {
			break
		}
	}
	paths := make([]string, 0, len(dirty))
	for _, path := range dirty {
		if owner, owned := graph.owner[path]; !owned || graph.untested(owner) || graph.goCommentOnly[path] || strings.HasPrefix(owner, "go:") && hiddenPackagePath(path) {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	start := make(map[string]Witness)
	if found {
		named := make(map[string]Witness)
		graph.readers(named, paths)
		for id, witness := range named {
			if testless(id) {
				start[id] = witness
			}
		}
	}
	// AZU-V0-006: a testless unit whose declared read scope holds such a path
	// observes it only through its callers, as a literal reader does. The
	// declaration itself is read by no unit, so it carries no caller.
	read := slices.DeleteFunc(slices.Clone(paths), func(path string) bool { return path == ReadScopesPath })
	scoped := make(map[string]Witness)
	graph.scopedReadersOf(scoped, read)
	for id, witness := range scoped {
		if !followed[id] && len(graph.units[id].Tests) == 0 {
			addWitness(start, id, witness)
		}
	}
	if len(start) == 0 {
		return nil
	}
	// A runner of a test-less command is followed like an importer of a
	// test-less library: its tests are the only check of the read
	// (AFP-V0-2992).
	users := graph.traverse(start, nil)
	walked := make(map[string]bool, len(users))
	for id := range users {
		followed[id] = true
		walked[id] = true
	}
	graph.testUsersOf(users, nil)
	graph.testRunnersOf(users, walked)
	carried := make(map[string]bool, len(users))
	for id, witness := range users {
		carried[id] = true
		// Each user is here for the read of an unowned path, not for a
		// build change: a unit whose declared read scope excludes that path
		// reads none of it, through code or a binary it runs (AFP-V0-161,
		// AFP-V0-2992). It is still walked through.
		if graph.scopeExcludes(id, witness.DirtyPath) {
			delete(users, id)
		}
	}
	mergeWitnesses(reached, users)
	return carried
}

// scopeExcludes reports whether unit id declares a read scope (AFP-V0-023)
// that excludes dirtyPath: its tests read nothing there, whatever code or
// binary they run, and full CI confines them to the scope (AFP-V0-161).
func (graph *Graph) scopeExcludes(id, dirtyPath string) bool {
	unit := graph.units[id]
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	return unit.ReadScoped && len(files) != 0 && !graph.inReadScope(relativeDirectory(files[0]), unit, dirtyPath)
}

// unboundedReadersOf selects, on any dirty path, each unit not already reached
// whose reads no literal bounds, witnessed by the smallest dirty path (rule
// (d)), with the cause that makes it one (AFP-V0-192). Like a reader it is not
// traversed: its unbounded dependents are already in the set.
func (graph *Graph) unboundedReadersOf(reached map[string]Witness, dirty []string) {
	if len(dirty) == 0 {
		return
	}
	listing := ""
	for _, dirtyPath := range dirty {
		if graph.listingMayChange(dirtyPath) {
			listing = dirtyPath
			break
		}
	}
	for _, id := range graph.unbounded {
		if _, seen := reached[id]; !seen {
			// A name-only walker is witnessed by the smallest dirty path
			// that can change a listing, and not selected without one
			// (AFP-V0-9963).
			witness := dirty[0]
			if graph.namesOnly[id] {
				if listing == "" {
					continue
				}
				witness = listing
			}
			cause := graph.causes[id]
			reached[id] = Witness{Kind: WitnessUnboundedReader, DirtyPath: witness, Via: []string{id}, Cause: &UnboundedCause{Unit: cause, Reads: graph.units[cause].UnboundedReads}}
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
// paths hold GatedRootLocatorsPath (TSP-V0-003) and whose edit the plan cannot
// bound (TSP-V0-608): the edited declaration may clear a root locator its own
// head graph trusts.
const FrontierGatedLocatorsChanged = "go:gated-root-locators-changed"

// WitnessGatedLocatorChange selects a Go package whose gated root-locator
// declaration entry was added, removed or changed between the commit a plan
// is measured from and its tree (TSP-V0-608): under the base declaration its
// dependents were root-locator dependents, so it is seeded as a changed unit
// and traversed to its transitive importers.
const WitnessGatedLocatorChange = "GATED_LOCATOR_CHANGE"

// GatedLocator is one gated root-locator declaration entry (TSP-V0-003): the
// absolute gate the package's root locating requires and the project's reason.
type GatedLocator struct {
	Gate   string `json:"gate"`
	Reason string `json:"reason"`
}

// GatedLocatorChanges returns, sorted, the package directories whose entries
// differ between the base and head declarations: added, removed or changed
// in gate or reason (TSP-V0-608). An absent declaration is an empty map.
func GatedLocatorChanges(base, head map[string]GatedLocator) []string {
	changed := map[string]bool{}
	for directory, entry := range base {
		if other, kept := head[directory]; !kept || other != entry {
			changed[directory] = true
		}
	}
	for directory := range head {
		if _, present := base[directory]; !present {
			changed[directory] = true
		}
	}
	return sortedKeys(changed)
}

// gatedLocatorSeeds returns the units a bounded edit to the gated root-locator
// declaration seeds (TSP-V0-608): the Go unit of each changed directory, with
// a GATED_LOCATOR_CHANGE witness, and whether the edit is closed. It is
// closed only when the plan knows the changed directories (the measured
// commit's declaration was read, WithGatedLocatorChanges), the head
// declaration is valid (the Go plugin raised no go:gated-root-locators-invalid
// frontier) and every changed directory holds an observed Go unit; otherwise
// the edit stays the TSP-V0-003 unknown. The declaration path itself keeps
// its non-source classification and readers as before.
func (graph *Graph) gatedLocatorSeeds() (map[string]Witness, bool) {
	if graph.gatedLocatorChanges == nil || slices.Contains(graph.frontier, "go:gated-root-locators-invalid") {
		return nil, false
	}
	packages := graph.goPackageDirectories()
	seeds := make(map[string]Witness, len(graph.gatedLocatorChanges))
	for _, directory := range graph.gatedLocatorChanges {
		id, observed := packages[directory]
		if !observed {
			return nil, false
		}
		seeds[id] = Witness{Kind: WitnessGatedLocatorChange, DirtyPath: GatedRootLocatorsPath, Via: []string{id}}
	}
	return seeds, true
}

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

// ShellTestInputsPath is the repository's declaration of the inputs each
// declared shell test reads by a word the shell plugin does not resolve
// (AFP-V0-2400). The plugin reads it when the graph is built, so a plan over a
// changed declaration would judge the test's inputs by the change itself: a
// plan with the declaration dirty, deleted included, is UNKNOWN
// (shellTestInputsChanged).
const ShellTestInputsPath = ".corvint/shell-test-inputs.json"

// FrontierShellTestInputsChanged is the unknown detail of a plan whose dirty
// paths hold ShellTestInputsPath (AFP-V0-2400).
const FrontierShellTestInputsChanged = "shell:test-inputs-changed"

// shellTestInputsChanged returns the AFP-V0-2400 unknown when normalized holds
// the shell test input declaration.
func shellTestInputsChanged(normalized []string) []Unknown {
	if !slices.Contains(normalized, ShellTestInputsPath) {
		return nil
	}
	return []Unknown{{Reason: UnknownLanguageFrontier, Detail: FrontierShellTestInputsChanged}}
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
// the gated root-locator declaration, whatever unit names it, unless the
// edit is closed by its base comparison (TSP-V0-608, gatedLocatorSeeds).
func (graph *Graph) gatedLocatorsChanged(normalized []string) []Unknown {
	if !slices.Contains(normalized, GatedRootLocatorsPath) {
		return nil
	}
	if _, closed := graph.gatedLocatorSeeds(); closed {
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
		if _, seen := reached[id]; seen || (!unit.ReadScoped && graph.priorReadScopes == nil && (graph.readScopeInputs == nil || graph.readScopeInputs.priorTrees == nil)) {
			continue
		}
		files := append(append([]string(nil), unit.Sources...), unit.Tests...)
		if len(files) == 0 {
			continue
		}
		directory := relativeDirectory(files[0])
		for _, dirtyPath := range dirty {
			if !graph.RecordEntryAllows(id, dirtyPath) {
				continue
			}
			bears := unit.ReadScoped && graph.inReadScope(directory, unit, dirtyPath)
			if dirtyPath == ReadScopesPath && graph.priorReadScopes != nil {
				bears = graph.declarationChanged(id, unit, directory) || (unit.ReadScoped && inDeclaredScope(directory, unit.ReadScope, dirtyPath))
			}
			if dirtyPath == ReadScopeTreesPath {
				bears = graph.treeDeclarationChanged(id, directory, unit) || (unit.ReadScoped && inDeclaredScope(directory, unit.ReadScope, dirtyPath))
			}
			if bears {
				reached[id] = Witness{Kind: WitnessDeclaredReadScope, DirtyPath: dirtyPath, Via: []string{id}, ReadScope: graph.treeReadScopeCause(id, directory, unit, dirtyPath)}
				break
			}
		}
	}
}

// readScopeCause records the exact declaration that justifies fan-out rather
// than treating a declared subtree as a Go import (TSP-V0-1062).
func readScopeCause(id, directory string, scope []string, dirtyPath string) *ReadScopeCause {
	if dirtyPath == ReadScopesPath {
		// The Go declaration itself changed; InReadScope selects shell units on it too.
		return &ReadScopeCause{Declaration: ReadScopesPath, Entry: ReadScopesPath}
	}
	if dirtyPath == ShellReadScopesPath {
		return &ReadScopeCause{Declaration: ShellReadScopesPath, Entry: ShellReadScopesPath}
	}
	declaration := ReadScopesPath
	if strings.HasPrefix(id, "shell:") {
		declaration = ShellReadScopesPath
	}
	entry := directory + "/"
	if directory == "." {
		entry = "./"
	}
	for _, candidate := range scope {
		target := strings.TrimSuffix(candidate, "/")
		if dirtyPath == target || strings.HasPrefix(target, dirtyPath+"/") || (target != candidate && strings.HasPrefix(dirtyPath, candidate)) {
			entry = candidate
			break
		}
	}
	return &ReadScopeCause{Declaration: declaration, Entry: entry}
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
// a lone file name (namesFileOnlyWithin, AFP-V0-400): it runs from the
// repository root or below its own directory. Any other non-Go unit keeps the
// component-run relation: its tests' working directory is unknown. A Go unit
// whose token names a path other than a CEM or a record entry reads it (AFP-V0-2203): no literal
// index proves the root its literal is joined to, which may be
// `os.Getenv("FIXTURES")`, a flag, a walked directory or any other
// non-constant prefix, so placing the path's directories would drop a real
// reader. For the CEM sidecar and a sealed CEM (resolvedEvidence), a Go unit's
// token must resolve against the unit's directory, or against the repository
// root when a parent-only or anchored token of the unit could put it there, to
// that path or one of its ancestors. An anchored token always resolves: it is
// anchored at its module's directory, which this graph does not record, so
// narrowing it could drop a real reader.
// For a record entry (recordDirectory, AFP-V0-2743) the resolved path must lie
// at or below the record directory (resolvesRecord), or at the root or an
// ancestor of it from which the unit's other literals join the rest of the way
// (reachesRecord, AFP-V0-2746); a `/`-anchored token is resolved from the root,
// and a unit that locates the root at run time resolves every token from it too.
// A unit with a declared read scope (AFP-V0-023) reads no dirty path outside
// that scope, whatever its tokens name (AFP-V0-161): the declaration states
// it, and full CI confines the package's tests to it.
func (graph *Graph) resolves(id, dirtyPath string) bool {
	unit := graph.units[id]
	files := append(append([]string(nil), unit.Sources...), unit.Tests...)
	if unit.ReadScoped && len(files) != 0 && !graph.inReadScope(relativeDirectory(files[0]), unit, dirtyPath) {
		return false
	}
	if unit.PlacesFileNames && !strings.HasPrefix(id, "go:") && len(files) != 0 {
		return namesFileOnlyWithin(relativeDirectory(files[0]), unit.PathTokens, dirtyPath)
	}
	if !strings.HasPrefix(id, "go:") || len(files) == 0 {
		// A unit with no files, such as a dependent nested module
		// (AFP-V0-582), has no directory to narrow a token against.
		return true
	}
	directory := relativeDirectory(files[0])
	record, isRecord := recordDirectory(dirtyPath)
	if !resolvedEvidence(dirtyPath) && !isRecord {
		// AFP-V0-2203: no placement narrowing for any other path.
		return true
	}
	anchored := rootAnchored(unit.PathTokens)
	if isRecord {
		// A unit that locates the root at run time (an unbounded reader)
		// can join any of its tokens to it (AFP-V0-2743).
		_, locates := graph.causes[id]
		anchored = anchored || locates
	}
	parts := strings.Split(dirtyPath, "/")
	for _, value := range unit.PathTokens {
		if !namesPath(value, parts) {
			continue
		}
		if isRecord {
			if rest, rooted := strings.CutPrefix(value, "/"); rooted {
				if reachesRecord(unit.PathTokens, "", rest, dirtyPath, record) {
					return true
				}
				continue
			}
			if reachesRecord(unit.PathTokens, directory, value, dirtyPath, record) || (anchored && reachesRecord(unit.PathTokens, "", value, dirtyPath, record)) {
				return true
			}
			continue
		}
		if strings.HasPrefix(value, "/") || resolvesWithin(directory, value, dirtyPath) || (anchored && resolvesWithin("", value, dirtyPath)) {
			return true
		}
	}
	return false
}

// BuildLogDirectory holds the build log, one `docs/build-log/<date>-<slug>.md`
// entry per recorded decision or result (decision 0423); nearly every change
// adds one.
const BuildLogDirectory = "docs/build-log/"

// recordDirectory returns the record directory of a record entry
// (AFP-V0-2743): a sealed CEM directly under SealedChangesDirectory, or a
// Markdown entry directly under BuildLogDirectory. Nearly every change adds
// one of each, so a reader of either must read that entry or list that
// directory; a token that resolves only to an ancestor of the directory (the
// repository root, `.corvint`, `docs`) reaches the entry through other
// literals the unit joins to it (AFP-V0-2746), or through a name computed or
// walked at run time, which the reader rules never bound and the
// unbounded-reader rule (AFP-V0-012 rule (d)) and declared read scopes
// (AFP-V0-023) select instead; AFP-V0-2747 refuses to record such a walker.
func recordDirectory(dirtyPath string) (string, bool) {
	if name, ok := strings.CutPrefix(dirtyPath, SealedChangesDirectory); ok && !strings.Contains(name, "/") && len(name) > len(".cem.json") && strings.HasSuffix(name, ".cem.json") {
		return SealedChangesDirectory, true
	}
	if name, ok := strings.CutPrefix(dirtyPath, BuildLogDirectory); ok && !strings.Contains(name, "/") && len(name) > len(".md") && strings.HasSuffix(name, ".md") {
		return BuildLogDirectory, true
	}
	return "", false
}

// resolvesRecord reports whether value, joined to directory, names the
// record entry dirtyPath as resolvesWithin does, at or below its record
// directory: the entry itself, the record directory, or a fragment completed
// at the entry's position. A token that resolves to the repository root or
// to a proper ancestor of record does not by itself (AFP-V0-2743); see
// reachesRecord for one the unit's other literals extend (AFP-V0-2746).
func resolvesRecord(directory, value, dirtyPath, record string) bool {
	if !resolvesWithin(directory, value, dirtyPath) {
		return false
	}
	resolved := path.Join(directory, value)
	if resolved == "." || resolved == "" {
		return false
	}
	return !strings.HasPrefix(record, resolved+"/") || resolved+"/" == record
}

// reachesRecord reports whether a unit holding tokens reaches the record entry
// dirtyPath of record through value, joined to directory: value resolves at
// or below record (resolvesRecord), or it resolves to the repository root or
// a proper ancestor of record and the unit's literals can join the rest of
// the way (AFP-V0-2746). `filepath.Join("../docs", "build-log", name)` in
// package `reader` reads every entry of `docs/build-log/` through three
// separate literals, none of which resolves to the directory alone.
func reachesRecord(tokens []string, directory, value, dirtyPath, record string) bool {
	if resolvesRecord(directory, value, dirtyPath, record) {
		return true
	}
	below, ok := recordAncestor(directory, value, dirtyPath, record)
	return ok && joinsComponents(tokens, below)
}

// recordAncestor returns, when value joined to directory resolves as
// resolvesWithin does to the repository root or to a proper ancestor of
// record, the components of record below that ancestor: what other literals
// must join to reach record (AFP-V0-2746).
func recordAncestor(directory, value, dirtyPath, record string) ([]string, bool) {
	if !resolvesWithin(directory, value, dirtyPath) {
		return nil, false
	}
	prefix := path.Join(directory, value) + "/"
	if prefix == "./" {
		prefix = ""
	}
	if prefix == record || !strings.HasPrefix(record, prefix) {
		return nil, false
	}
	return strings.Split(strings.TrimSuffix(record[len(prefix):], "/"), "/"), true
}

// joinsComponents reports whether every one of components is a whole
// component of one of tokens, so separate literals could join them in any
// order or expression (AFP-V0-2746): the conservative reading of a composed
// path, since the literal index does not reconstruct the expression.
func joinsComponents(tokens, components []string) bool {
	for _, component := range components {
		if !slices.ContainsFunc(tokens, func(value string) bool {
			return strings.Contains(value, component) && slices.Contains(strings.Split(value, "/"), component)
		}) {
			return false
		}
	}
	return true
}

// namesFileOnlyWithin reports whether a non-Go unit in directory whose plugin
// certifies its roots (Unit.PlacesFileNames, AFP-V0-400) and whose tokens name
// dirtyPath can reach it (AFP-V0-072). A token that names it by a run that
// places a directory (a multi-component run, or any run of an anchored or
// climbing token) keeps the component-run relation. When only unanchored lone
// tokens name it by its file name (V1-0290), the unit reaches it only in its
// own subtree, where its tests run, or under a base its literals can reach:
// the repository root, or any ancestor of its directory once a token climbs
// (`../model.sh`). Below that base every directory component of the path must
// be a whole component of one of its tokens, so a literal could join it, or a
// component the lexer cannot keep whole (a space, a non-ASCII letter), which
// no token can witness and so never narrows. The certification is the proof
// that no root comes from anywhere else; a Go unit has none (AFP-V0-2203).
func namesFileOnlyWithin(directory string, tokens []string, dirtyPath string) bool {
	if directory == "." || strings.HasPrefix(dirtyPath, directory+"/") {
		return true
	}
	parts := strings.Split(dirtyPath, "/")
	climbs, named := false, false
	for _, value := range tokens {
		if strings.HasPrefix(value, "/") || strings.Contains(value, "..") {
			if namesPath(value, parts) {
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
	if !named {
		return false
	}
	vocabulary := make(map[string]bool)
	for _, value := range tokens {
		for _, component := range strings.Split(value, "/") {
			vocabulary[component] = true
		}
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
	// Internally assembled graphs may not have passed through finish yet.
	// Every built or restored graph carries the immutable precomputed index.
	if graph.tokens == nil {
		return graph.splitPathTokens()
	}
	return graph.tokens
}

// contentPathTokens holds, in graph.order, each unit's path tokens less its
// NameListings, nil for a unit that has none (AFP-V0-9962).
func (graph *Graph) contentPathTokens() [][]pathToken {
	if graph.contentTokens == nil {
		return graph.splitContentTokens()
	}
	return graph.contentTokens
}

func (graph *Graph) splitContentTokens() [][]pathToken {
	tokens := make([][]pathToken, len(graph.order))
	for i, id := range graph.order {
		unit := graph.units[id]
		if len(unit.NameListings) == 0 {
			continue
		}
		tokens[i] = make([]pathToken, 0, len(unit.PathTokens))
		for _, value := range unit.PathTokens {
			if !slices.Contains(unit.NameListings, value) {
				tokens[i] = append(tokens[i], newPathToken(value))
			}
		}
	}
	return tokens
}

func (graph *Graph) splitPathTokens() [][]pathToken {
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

// namesAny reports whether one of the tokens names the split path. anyDepth
// drops the V1-0290 file-name-only rule: a Go unit's lone component names a
// directory component too, for a path other than a CEM (AFP-V0-2203).
func namesAny(tokens []pathToken, parts []string, anyDepth bool) bool {
	for _, token := range tokens {
		if token.namesAt(parts, anyDepth) {
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
	return token.namesAt(parts, false)
}

func (token pathToken) namesAt(parts []string, anyDepth bool) bool {
	for _, candidate := range token.runs {
		first := 0
		if len(candidate.components) == 1 && token.nameOnly && !anyDepth {
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

// Readers selects, from no prior reach, every unit the reader rules select for
// paths: a literal or glob naming one, an unbounded reader, a declared read
// scope or existence declaration holding one, an import-graph reader the
// change can alter, a declared test owner of one, and a package embedding one
// with every unit that reaches that package. It is the read reach of
// the paths alone, apart from import and source reach, so a caller that knows
// a change by its executed lines can still tell which units may observe the
// changed bytes (GLS-V0-013). The units are keyed by id with the witness the
// rule gives; a nil graph has no readers.
func (graph *Graph) Readers(paths []string) map[string]Witness {
	reached := make(map[string]Witness)
	if graph == nil || len(paths) == 0 {
		return reached
	}
	normalized := NormalizePaths(paths)
	graph.importGraphReadersOf(reached, normalized)
	graph.readers(reached, normalized)
	graph.unboundedReadersOf(reached, graph.unboundedDirty(normalized))
	graph.scopedReadersOf(reached, normalized)
	graph.deletedScriptObserversOf(reached, normalized, make(map[string]bool))
	graph.existenceReadersOf(reached, normalized)
	graph.ownersOf(reached, normalized)
	graph.testEmbedReadersOf(reached, normalized)
	// A package embedding files under its directory reads a changed file
	// there as data (goDataRule), whatever else reaches it.
	packages := graph.goPackageDirectories()
	embedding := make(map[string]Witness)
	for _, p := range normalized {
		for directory := relativeDirectory(p); ; directory = relativeDirectory(directory) {
			if id, present := packages[directory]; present && graph.units[id].Embeds {
				addWitness(embedding, id, Witness{Kind: WitnessEnclosingPackage, DirtyPath: p, Via: []string{id}})
			}
			if directory == "." {
				break
			}
		}
	}
	if len(embedding) == 0 {
		return reached
	}
	// The embedded bytes leave the package through its API: an importer's
	// test can observe them without executing a changed line, so every
	// transitive importer and runner of an embedding package, with their
	// test-only users and test-only runners, reads the path too. All edges
	// are followed, so no such reader is dropped.
	carried := graph.traverse(embedding, nil)
	walked := make(map[string]bool, len(carried))
	for id := range carried {
		walked[id] = true
	}
	graph.testUsersOf(carried, nil)
	graph.testRunnersOf(carried, walked)
	mergeWitnesses(reached, carried)
	return reached
}

// hiddenPackagePath identifies the Go directories newly indexed by AZU-V0-005.
// Their source used to be unowned, so retain every testless reader's callers:
// owning the Go file does not model a shell script that reads or executes it.
func hiddenPackagePath(relative string) bool {
	for _, component := range strings.Split(path.Dir(relative), "/") {
		if component != "." && (strings.HasPrefix(component, ".") || strings.HasPrefix(component, "_")) {
			return true
		}
	}
	return false
}

// NamesPath reports whether the path literals of unit id name dirtyPath, the
// match a PATH_LITERAL_READER selection rests on (AFP-V0-2377).
func (graph *Graph) NamesPath(id, dirtyPath string) bool {
	parts := strings.Split(dirtyPath, "/")
	for i, tokens := range graph.pathTokens() {
		if graph.order[i] == id {
			return namesAny(tokens, parts, strings.HasPrefix(id, "go:") && !resolvedEvidence(dirtyPath))
		}
	}
	return false
}

// GlobRead returns the first dirty path a glob of unit id matches, with the
// glob, the match a PATH_GLOB_READER selection rests on (AFP-V0-2377).
func (graph *Graph) GlobRead(id string, dirty []string) (dirtyPath, glob string, ok bool) {
	return graph.globRead(id, dirty)
}

// testEmbedReadersOf selects only the embedding tests, never production
// importers or binary consumers (AFP-V0-3350/3351). Like production embeds,
// retain every descendant path: deleted targets and unresolved directives
// must not disappear merely because the current walk cannot resolve them.
func (graph *Graph) testEmbedReadersOf(reached map[string]Witness, paths []string) {
	packages := graph.goPackageDirectories()
	for _, dirty := range paths {
		for directory := relativeDirectory(dirty); ; directory = relativeDirectory(directory) {
			if id, ok := packages[directory]; ok && graph.units[id].TestEmbeds {
				addWitness(reached, id, Witness{Kind: WitnessEnclosingPackage, DirtyPath: dirty, Via: []string{id}})
			}
			if directory == "." {
				break
			}
		}
	}
}

// Namers lists, in id order, the units carrying a path token that names
// dirtyPath under the AFP-V0-021 reader match, so a test outside the package
// can resolve a PATH_LITERAL_READER witness.
func (graph *Graph) Namers(dirtyPath string) []string {
	parts := strings.Split(dirtyPath, "/")
	ids := make([]string, 0)
	for i, tokens := range graph.pathTokens() {
		if namesAny(tokens, parts, strings.HasPrefix(graph.order[i], "go:") && !resolvedEvidence(dirtyPath)) {
			ids = append(ids, graph.order[i])
		}
	}
	return ids
}
